package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

const version = "1.0.0"

type Config struct {
	Mode          string
	Port          int
	BuildCmd      string
	RunCmd        string
	WatchDirs     []string
	WatchExts     []string
	ExcludeDirs   []string
	IncludeTempl  bool
	IncludeTailw  bool
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
		includeTempl = flag.Bool("templ", false, "Watch .templ files and run templ generate")
		includeTailw = flag.Bool("tailwind", false, "Watch tailwind.config.js and run tailwindcss")
		showVersion = flag.Bool("version", false, "Show version")
	)
	
	flag.Parse()
	
	if *showVersion {
		fmt.Printf("hot version %s\n", version)
		os.Exit(0)
	}
	
	config := &Config{
		Mode:         *mode,
		Port:         *port,
		BuildCmd:     *buildCmd,
		RunCmd:       *runCmd,
		IncludeTempl: *includeTempl,
		IncludeTailw: *includeTailw,
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
		if config.IncludeTempl {
			config.WatchExts = append(config.WatchExts, ".templ")
		}
		if config.IncludeTailw {
			config.WatchExts = append(config.WatchExts, ".css", ".html", ".js")
		}
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
	}
	
	runner := NewRunner(config)
	
	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Println("\n🛑 Shutting down...")
		runner.Stop()
		os.Exit(0)
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
	for _, part := range splitByComma(s) {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func splitByComma(s string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if c == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
