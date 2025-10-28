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
	
	// Handle file change events
	debounceTimer := time.NewTimer(0)
	<-debounceTimer.C // drain the timer
	
	for {
		select {
		case <-r.ctx.Done():
			return nil
		case path := <-eventCh:
			// Debounce rapid file changes
			debounceTimer.Reset(300 * time.Millisecond)
			
			go func(p string) {
				<-debounceTimer.C
				log.Printf("📝 File changed: %s", p)
				
				// Run templ generate if .templ file changed
				if r.config.IncludeTempl && hasExt(p, ".templ") {
					log.Println("🔨 Running templ generate...")
					if err := runCommand("templ", "generate"); err != nil {
						log.Printf("⚠️  templ generate failed: %v", err)
					}
				}
				
				// Run tailwindcss if relevant files changed
				if r.config.IncludeTailw && (hasExt(p, ".css") || hasExt(p, ".html") || hasExt(p, ".js")) {
					log.Println("🎨 Running tailwindcss...")
					if err := runCommand("tailwindcss", "-i", "./input.css", "-o", "./static/output.css"); err != nil {
						log.Printf("⚠️  tailwindcss failed: %v", err)
					}
				}
				
				if err := r.buildAndRun(); err != nil {
					log.Printf("❌ Build failed: %v", err)
				} else {
					// Trigger browser reload in web mode
					if r.config.Mode == "web" && r.liveReload != nil {
						r.liveReload.TriggerReload()
					}
				}
			}(path)
		}
	}
}

func (r *Runner) buildAndRun() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Stop existing process
	if r.cmd != nil && r.cmd.Process != nil {
		log.Println("🛑 Stopping existing process...")
		r.cmd.Process.Kill()
		r.cmd.Wait()
		r.cmd = nil
	}
	
	// Build
	log.Printf("🔨 Building: %s", r.config.BuildCmd)
	buildCmd := exec.Command("sh", "-c", r.config.BuildCmd)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	
	// Run
	log.Printf("🚀 Starting: %s", r.config.RunCmd)
	r.cmd = exec.Command("sh", "-c", r.config.RunCmd)
	r.cmd.Stdout = os.Stdout
	r.cmd.Stderr = os.Stderr
	
	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("run failed: %w", err)
	}
	
	log.Println("✅ Application started successfully")
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

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func hasExt(path, ext string) bool {
	return len(path) >= len(ext) && path[len(path)-len(ext):] == ext
}
