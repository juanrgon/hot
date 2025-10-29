package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Runner struct {
	config        *Config
	cmd           *exec.Cmd
	watcher       *Watcher
	liveReload    *LiveReloadServer
	proxy         *InjectorProxy
	mu            sync.Mutex
	ctx           context.Context
	cancel        context.CancelFunc
	statusMu      sync.RWMutex
	status        RunnerStatus
	reloadMu      sync.Mutex
	reloadPending bool
}

func NewRunner(config *Config) *Runner {
	ctx, cancel := context.WithCancel(context.Background())
	return &Runner{
		config: config,
		ctx:    ctx,
		cancel: cancel,
		status: RunnerStatus{
			State:     StateIdle,
			Message:   "idle",
			Timestamp: time.Now().UTC(),
		},
	}
}

func (r *Runner) Start() error {
	// Start live reload server for web mode
	if r.config.Mode == "web" {
		r.liveReload = NewLiveReloadServer(r.config.Port)
		go func() {
			if err := r.liveReload.Start(); err != nil {
				log.Printf("Error starting live reload server: %v", err)
			}
		}()
		// Give the server a moment to start
		time.Sleep(100 * time.Millisecond)
		r.syncLiveReloadState()
	}

	// Initial build and run
	if err := r.buildAndRun(); err != nil {
		return fmt.Errorf("initial build failed: %w", err)
	}

	r.startProxy()

	// Start file watcher
	r.watcher = NewWatcher(r.config)

	eventCh := make(chan string)
	go r.watcher.Watch(r.ctx, eventCh)

	// Handle file change events with debouncing
	var debounceMu sync.Mutex
	var debounceTimer *time.Timer

	for {
		select {
		case <-r.ctx.Done():
			return nil
		case path := <-eventCh:
			debounceMu.Lock()
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(300*time.Millisecond, func() {
				log.Printf("📝 File changed: %s", path)

				if err := r.buildAndRun(); err != nil {
					log.Printf("❌ Build failed: %v", err)
				}
			})
			debounceMu.Unlock()
		}
	}
}

func (r *Runner) buildAndRun() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.updateState(RunnerStatus{
		State:     StateBuilding,
		Message:   "Building latest changes…",
		Timestamp: time.Now().UTC(),
	})

	log.Printf("🔨 Building: %s", r.config.BuildCmd)
	buildCmd := exec.Command("sh", "-c", r.config.BuildCmd)
	var buildOutput bytes.Buffer
	buildCmd.Stdout = io.MultiWriter(os.Stdout, &buildOutput)
	buildCmd.Stderr = io.MultiWriter(os.Stderr, &buildOutput)

	if err := buildCmd.Run(); err != nil {
		details := strings.TrimSpace(buildOutput.String())
		if details == "" {
			details = err.Error()
		}
		r.updateState(RunnerStatus{
			State:     StateBuildFailed,
			Message:   "build failed",
			Error:     details,
			Timestamp: time.Now().UTC(),
		})
		r.clearReloadPending()
		return fmt.Errorf("build failed: %w", err)
	}

	if r.cmd != nil {
		if r.cmd.Process != nil {
			r.updateState(RunnerStatus{
				State:     StateStopping,
				Message:   "Stopping previous process…",
				Timestamp: time.Now().UTC(),
			})
			log.Println("🛑 Stopping old process...")
			terminateCommand(r.cmd)
		}
		r.cmd = nil
	}

	log.Printf("🚀 Starting new process: %s", r.config.RunCmd)
	newCmd := exec.Command("sh", "-c", r.config.RunCmd)
	prepareCommand(newCmd)
	newCmd.Stdout = os.Stdout
	newCmd.Stderr = os.Stderr

	if err := newCmd.Start(); err != nil {
		r.updateState(RunnerStatus{
			State:     StateBuildFailed,
			Message:   "failed to start new process",
			Error:     err.Error(),
			Timestamp: time.Now().UTC(),
		})
		r.clearReloadPending()
		return fmt.Errorf("run failed: %w", err)
	}

	r.cmd = newCmd

	if r.shouldWaitForReady() {
		target := r.appReadyURL()
		display := displayURL(target)
		r.setReloadPending(true)
		r.updateState(RunnerStatus{
			State:     StateStarting,
			Message:   fmt.Sprintf("Waiting for %s", display),
			Timestamp: time.Now().UTC(),
		})

		r.mu.Unlock()
		err := r.waitForApp(newCmd, target, display)
		r.mu.Lock()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
	} else {
		r.clearReloadPending()
		log.Println("✅ Application reloaded successfully")
		r.updateState(RunnerStatus{
			State:     StateRunning,
			Message:   fmt.Sprintf("process PID %d running", newCmd.Process.Pid),
			Timestamp: time.Now().UTC(),
		})
		r.triggerReload()
	}

	return nil
}

func (r *Runner) shouldWaitForReady() bool {
	return r.config.Mode == "web"
}

func (r *Runner) appReadyURL() string {
	target := strings.TrimSpace(r.config.ProxyTarget)
	if target == "" {
		target = "http://localhost:8080"
	}
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target
	}
	return "http://" + target
}

func displayURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "http://")
	trimmed = strings.TrimPrefix(trimmed, "https://")
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return raw
	}
	return trimmed
}

func (r *Runner) waitForApp(cmd *exec.Cmd, url, display string) error {
	client := &http.Client{Timeout: 1 * time.Second}
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-r.ctx.Done():
			r.clearReloadPending()
			return context.Canceled
		case <-timeout.C:
			if !r.isCurrentCommand(cmd) {
				return nil
			}
			r.updateState(RunnerStatus{
				State:     StateStarting,
				Message:   fmt.Sprintf("Still waiting for %s", display),
				Error:     fmt.Sprintf("startup timeout after 30s waiting for %s", display),
				Timestamp: time.Now().UTC(),
			})
			r.clearReloadPending()
			return fmt.Errorf("startup timeout waiting for %s", display)
		case <-ticker.C:
			if !r.isCurrentCommand(cmd) {
				return nil
			}
			resp, err := client.Get(url)
			if err != nil {
				if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
					r.clearReloadPending()
					r.updateState(RunnerStatus{
						State:     StateBuildFailed,
						Message:   "process exited during startup",
						Error:     fmt.Sprintf("exit status: %d", cmd.ProcessState.ExitCode()),
						Timestamp: time.Now().UTC(),
					})
					return fmt.Errorf("process exited during startup")
				}
				continue
			}
			resp.Body.Close()
			if !r.isCurrentCommand(cmd) {
				return nil
			}
			log.Println("✅ Application reloaded successfully")
			r.updateState(RunnerStatus{
				State:     StateRunning,
				Message:   fmt.Sprintf("Application ready on %s", display),
				Timestamp: time.Now().UTC(),
			})
			r.triggerReloadIfPending()
			return nil
		}
	}
}

func (r *Runner) triggerReload() {
	if r.config.Mode != "web" {
		return
	}
	if r.liveReload != nil {
		r.liveReload.TriggerReload()
	}
}

func (r *Runner) setReloadPending(value bool) {
	r.reloadMu.Lock()
	r.reloadPending = value
	r.reloadMu.Unlock()
}

func (r *Runner) clearReloadPending() {
	r.setReloadPending(false)
}

func (r *Runner) triggerReloadIfPending() {
	r.reloadMu.Lock()
	pending := r.reloadPending
	if pending {
		r.reloadPending = false
	}
	r.reloadMu.Unlock()

	if pending {
		r.triggerReload()
	}
}

func (r *Runner) isCurrentCommand(cmd *exec.Cmd) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cmd == cmd
}

func (r *Runner) Stop() {
	r.cancel()
	r.clearReloadPending()

	r.updateState(RunnerStatus{
		State:     StateStopping,
		Message:   "Shutting down…",
		Timestamp: time.Now().UTC(),
	})

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd != nil {
		if r.cmd.Process != nil {
			terminateCommand(r.cmd)
		}
		r.cmd = nil
	}

	if r.liveReload != nil {
		r.liveReload.Stop()
		r.liveReload = nil
	}

	if r.proxy != nil {
		r.proxy.Stop()
		r.proxy = nil
	}

	r.updateState(RunnerStatus{
		State:     StateIdle,
		Message:   "Stopped",
		Timestamp: time.Now().UTC(),
	})
}

func (r *Runner) startProxy() {
	if r.config.Mode != "web" {
		return
	}
	if r.config.ProxyListen == 0 || r.config.ProxyTarget == "" {
		return
	}
	if r.proxy != nil {
		return
	}

	proxy, err := NewInjectorProxy(r.config.ProxyListen, r.config.ProxyTarget, r.config.Port, r.currentStatus)
	if err != nil {
		log.Printf("Proxy setup failed: %v", err)
		return
	}

	r.proxy = proxy

	log.Printf("👉 Open http://localhost:%d in your browser for auto-injected reloads (proxying %s)", r.config.ProxyListen, displayURL(r.config.ProxyTarget))

	go func() {
		if err := proxy.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("Proxy runtime error: %v", err)
		}
	}()
}

func (r *Runner) currentStatus() RunnerStatus {
	r.statusMu.RLock()
	defer r.statusMu.RUnlock()
	return r.status
}

func (r *Runner) updateState(status RunnerStatus) {
	if status.Timestamp.IsZero() {
		status.Timestamp = time.Now().UTC()
	}
	r.statusMu.Lock()
	previous := r.status
	r.status = status
	r.statusMu.Unlock()

	if status.State == previous.State && status.Message == previous.Message && status.Error == previous.Error {
		return
	}

	if status.Error != "" {
		log.Printf("⚠️ State %s: %s", status.State, status.Error)
	} else if status.Message != "" {
		log.Printf("ℹ️ State %s: %s", status.State, status.Message)
	} else {
		log.Printf("ℹ️ State %s", status.State)
	}

	if r.liveReload != nil {
		r.liveReload.EmitState(status)
	}
}

func (r *Runner) syncLiveReloadState() {
	if r.liveReload == nil {
		return
	}
	r.statusMu.RLock()
	status := r.status
	r.statusMu.RUnlock()
	r.liveReload.EmitState(status)
}
