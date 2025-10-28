package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

type Runner struct {
	config     *Config
	cmd        *exec.Cmd
	watcher    *Watcher
	liveReload *LiveReloadServer
	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewRunner(config *Config) *Runner {
	ctx, cancel := context.WithCancel(context.Background())
	return &Runner{
		config: config,
		ctx:    ctx,
		cancel: cancel,
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
	}

	// Initial build and run
	if err := r.buildAndRun(); err != nil {
		return fmt.Errorf("initial build failed: %w", err)
	}

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
				} else {
					// Trigger browser reload in web mode
					if r.config.Mode == "web" && r.liveReload != nil {
						r.liveReload.TriggerReload()
					}
				}
			})
			debounceMu.Unlock()
		}
	}
}

func (r *Runner) buildAndRun() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Build first (before stopping the old process)
	log.Printf("🔨 Building: %s", r.config.BuildCmd)
	buildCmd := exec.Command("sh", "-c", r.config.BuildCmd)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Start new process
	log.Printf("🚀 Starting new process: %s", r.config.RunCmd)
	newCmd := exec.Command("sh", "-c", r.config.RunCmd)
	newCmd.Stdout = os.Stdout
	newCmd.Stderr = os.Stderr

	if err := newCmd.Start(); err != nil {
		return fmt.Errorf("run failed: %w", err)
	}

	// Give the new process a moment to start up
	time.Sleep(500 * time.Millisecond)

	// Stop old process (if exists)
	if r.cmd != nil && r.cmd.Process != nil {
		log.Println("🛑 Stopping old process...")
		r.cmd.Process.Kill()
		r.cmd.Wait()
	}

	// Update to the new process
	r.cmd = newCmd

	log.Println("✅ Application reloaded successfully")
	return nil
}

func (r *Runner) Stop() {
	r.cancel()

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd != nil && r.cmd.Process != nil {
		r.cmd.Process.Kill()
		r.cmd.Wait()
	}

	if r.liveReload != nil {
		r.liveReload.Stop()
	}
}
