package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

const version = "1.0.0"

type Config struct {
	Mode        string
	Port        int
	BuildCmd    string
	RunCmd      string
	WatchDirs   []string
	WatchExts   []string
	ExcludeDirs []string
	ProxyListen int
	ProxyTarget string
}

func main() {
	var (
		mode        = flag.String("mode", "web", "Mode: web (browser reload) or api (no browser reload)")
		port        = flag.Int("port", 3000, "Port for live reload server (web mode only)")
		buildCmd    = flag.String("build", "", "Custom build command (default: go build -o /tmp/app)")
		runCmd      = flag.String("run", "", "Custom run command (default: ./tmp/app)")
		watchDirs   = flag.String("watch", "", "Comma-separated directories to watch (default: current directory)")
		watchExts   = flag.String("exts", "", "Comma-separated file extensions to watch (default: .go)")
		excludeDirs = flag.String("exclude", "", "Comma-separated directories to exclude")
		proxyListen = flag.Int("proxy-listen", 5173, "Port for HTML-injecting proxy (web mode). Set to 0 to disable")
		proxyTarget = flag.String("proxy-target", "http://localhost:8080", "Upstream URL for HTML-injecting proxy (web mode). Empty to disable")
		showVersion = flag.Bool("version", false, "Show version")
	)

	flag.Parse()

	if *showVersion {
		fmt.Printf("hot version %s\n", version)
		os.Exit(0)
	}

	config := &Config{
		Mode:        *mode,
		Port:        *port,
		BuildCmd:    *buildCmd,
		RunCmd:      *runCmd,
		ProxyListen: *proxyListen,
		ProxyTarget: *proxyTarget,
	}

	// Parse watch directories
	if *watchDirs != "" {
		config.WatchDirs = parseCommaSeparated(*watchDirs)
	} else {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		config.WatchDirs = []string{wd}
	}

	// Parse watch extensions
	if *watchExts != "" {
		config.WatchExts = parseCommaSeparated(*watchExts)
	} else {
		config.WatchExts = []string{".go"}
	}

	// Parse exclude directories
	if *excludeDirs != "" {
		config.ExcludeDirs = parseCommaSeparated(*excludeDirs)
	} else {
		config.ExcludeDirs = []string{
			"vendor", "node_modules", ".git", ".idea", ".vscode",
			"tmp", "dist", "build",
		}
	}

	// Set default build command
	if config.BuildCmd == "" {
		tmpDir := filepath.Join(os.TempDir(), "hot-build")
		os.MkdirAll(tmpDir, 0755)
		config.BuildCmd = fmt.Sprintf("go build -o %s/app", tmpDir)
	}

	// Set default run command
	if config.RunCmd == "" {
		tmpDir := filepath.Join(os.TempDir(), "hot-build")
		config.RunCmd = fmt.Sprintf("%s/app", tmpDir)
	}

	log.Printf("🔥 Hot reloading started in %s mode", config.Mode)
	log.Printf("   Watching: %v", config.WatchDirs)
	log.Printf("   Extensions: %v", config.WatchExts)
	log.Printf("   Excluding: %v", config.ExcludeDirs)

	if config.Mode == "web" {
		log.Printf("   Live reload server: http://localhost:%d", config.Port)
		if config.ProxyListen != 0 && config.ProxyTarget != "" {
			log.Printf("   Proxy: http://localhost:%d -> %s (auto inject)", config.ProxyListen, config.ProxyTarget)
		}
	}

	runner := NewRunner(config)
	defer runner.Stop()

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)
	defer signal.Stop(sigChan)

	go func() {
		sig := <-sigChan
		log.Printf("\n🛑 Shutting down (%s)...", sig)
		runner.Stop()
	}()

	if err := runner.Start(); err != nil {
		log.Fatal(err)
	}
}

func parseCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
