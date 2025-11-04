// This is a single-file hot-reloading tool tailored for Go web development. I've named it `hot`.

// ### Key Features (JS-Ecosystem Inspired)
// *   **Integrated Proxy:** It sits in front of your application.
// *                         When your app is rebuilding, the proxy **holds** incoming
// *                         requests instead of failing immediately (like `nodemon` + standard proxy setups).
// *                         Once your app is ready, the pending requests are fulfilled.
// *
// *   **Zero External Dependencies:** Uses only the Go standard library (including a standard library
// *                                   polling-based file watcher to avoid CGO/platform-specific dependency
// *                                   issues in a single file).
// *
// *   **Port Management:** It explicitly manages an "App Port" (for your actual binary) and a "Proxy Port" (where you point your browser).
// *
// *   **Resilient:** It waits for your application's TCP port to be accepting connections before releasing
// *                  held proxy requests, preventing "Connection Refused" races.

// ### How to use it
//  1. Save the code below as `hot.go` in your project root.
//  2. Ensure your web application reads the port it should listen on from the `PORT` environment variable (or a flag you pass to it).
//     ```go
//     Example in your main.go
//     =======================
//     port := os.Getenv("PORT")
//     if port == "" { port = "8080" }
//     http.ListenAndServe(":"+port, nil)
//     ```
//  3. Run it: `go run hot.go`
//     *   By default, it proxies `localhost:3000` -> Your App on `localhost:3001`.
//     *   Access your app at `http://localhost:3000`.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
)

// --- Configuration ---

// Config represents the complete hot configuration
type Config struct {
	Root   string       `toml:"root"`
	TmpDir string       `toml:"tmp_dir"`
	Build  BuildConfig  `toml:"build"`
	Watch  WatchConfig  `toml:"watch"`
	Proxy  ProxyConfig  `toml:"proxy"`
	Screen ScreenConfig `toml:"screen"`
	Log    LogConfig    `toml:"log"`
}

// BuildConfig contains build-related settings
type BuildConfig struct {
	Cmd          string   `toml:"cmd"`
	Bin          string   `toml:"bin"`
	Target       string   `toml:"target"`
	BuildArgs    []string `toml:"build_args"`
	RunArgs      []string `toml:"run_args"`
	PreCmd       []string `toml:"pre_cmd"`
	PostCmd      []string `toml:"post_cmd"`
	Delay        int      `toml:"delay"`
	ErrorLog     string   `toml:"error_log"`
	StopOnError  bool     `toml:"stop_on_error"`
	KillDelay    string   `toml:"kill_delay"`
}

// WatchConfig contains file watching settings
type WatchConfig struct {
	Dirs          []string `toml:"dirs"`
	Extensions    []string `toml:"extensions"`
	ExcludeDirs   []string `toml:"exclude_dirs"`
	ExcludeFiles  []string `toml:"exclude_files"`
	ExcludeRegex  []string `toml:"exclude_regex"`
	PollInterval  string   `toml:"poll_interval"`
}

// ProxyConfig contains proxy server settings
type ProxyConfig struct {
	Port          int  `toml:"port"`
	AppPort       int  `toml:"app_port"`
	BrowserReload bool `toml:"browser_reload"`
}

// ScreenConfig contains terminal display settings
type ScreenConfig struct {
	ClearOnRebuild bool `toml:"clear_on_rebuild"`
	KeepScroll     bool `toml:"keep_scroll"`
}

// LogConfig contains logging settings
type LogConfig struct {
	Timestamps bool `toml:"timestamps"`
}

var (
	config struct {
		AppPort       string
		ProxyPort     string
		BuildTarget   string
		BuildOutput   string
		BuildCmd      string
		WatchDirs     []string
		Extensions    map[string]bool
		ExcludeRegex  []string
		BuildArgs     []string
		RunArgs       []string
		PollInterval  time.Duration
		BrowserReload bool
	}
	ansi = struct {
		Reset, Red, Green, Yellow, Blue, Magenta, Cyan string
	}{
		"\033[0m", "\033[31m", "\033[32m", "\033[33m", "\033[34m", "\033[35m", "\033[36m",
	}
)

// loadConfig loads configuration from a TOML file
func loadConfig(path string) (*Config, error) {
	var cfg Config

	// Set defaults
	cfg.Root = "."
	cfg.TmpDir = "tmp"
	cfg.Build.Bin = "./tmp/main"
	cfg.Build.Target = "."
	cfg.Build.Delay = 200
	cfg.Build.StopOnError = true
	cfg.Build.KillDelay = "1s"
	cfg.Watch.Dirs = []string{"."}
	cfg.Watch.Extensions = []string{"go"}
	cfg.Watch.ExcludeDirs = []string{"tmp", "vendor", ".git"}
	cfg.Watch.PollInterval = "500ms"
	cfg.Proxy.Port = 9000
	cfg.Proxy.AppPort = 8080
	cfg.Proxy.BrowserReload = true
	cfg.Screen.KeepScroll = true

	// If file doesn't exist, return defaults
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &cfg, nil
	}

	// Load config file
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// findConfigFile looks for hot.toml or .air.toml in order
func findConfigFile() string {
	candidates := []string{"hot.toml", ".air.toml"}
	for _, name := range candidates {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}
	return ""
}

func init() {
	// Disable colors if on Windows and likely not supported (basic heuristic)
	if runtime.GOOS == "windows" && os.Getenv("TERM") == "" {
		ansi = struct{ Reset, Red, Green, Yellow, Blue, Magenta, Cyan string }{}
	}
}

// --- Main ---

const version = "1.0.0"

func main() {
	// Check for subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			cmdInit()
			return
		case "convert":
			cmdConvert()
			return
		case "version", "-v", "--version":
			fmt.Printf("hot version %s\n", version)
			return
		case "help", "-h", "--help":
			printHelp()
			return
		case "run":
			// Remove "run" from args so flag parsing works
			os.Args = append(os.Args[:1], os.Args[2:]...)
		default:
			// If it starts with -, treat as a flag for run command
			if !strings.HasPrefix(os.Args[1], "-") {
				fmt.Printf("Unknown command: %s\n\n", os.Args[1])
				printHelp()
				os.Exit(1)
			}
		}
	}

	// Run the dev server (default command)
	runDevServer()
}

func runDevServer() {
	parseFlags()
	logger.Info("🚀 Hot dev server starting...")
	logger.Info(fmt.Sprintf("Proxy: http://localhost:%s -> App: http://localhost:%s", config.ProxyPort, config.AppPort))

	// Show mode
	if config.BrowserReload {
		logger.Info("📡 Mode: WEB (browser auto-reload enabled)")
	} else {
		logger.Info("🔌 Mode: API (browser features disabled)")
	}

	orchestrator := NewOrchestrator()

	// 1. Start the Proxy Server
	go startProxyServer(orchestrator)

	// 2. Start the File Watcher
	watcherEvents := make(chan string)
	go startPollingWatcher(watcherEvents)

	// 3. Main Event Loop
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	// Initial build and run
	orchestrator.TriggerRestart("Initial start")

	debounceTimer := time.NewTimer(0)
	if !debounceTimer.Stop() {
		<-debounceTimer.C
	}

	for {
		select {
		case changedFile := <-watcherEvents:
			// Simple debounce
			debounceTimer.Reset(200 * time.Millisecond)
			go func() {
				<-debounceTimer.C
				orchestrator.TriggerRestart(filepath.Base(changedFile) + " changed")
			}()
		case <-interrupt:
			logger.Info("Shutting down...")
			orchestrator.Kill()
			os.Exit(0)
		}
	}
}

func printHelp() {
	fmt.Printf(`Hot - Zero-dependency hot reload for Go

Usage:
  hot [command] [flags]

Commands:
  run          Start the dev server (default)
  init         Generate hot.toml config file
  convert      Convert .air.toml to hot.toml
  version      Show version
  help         Show this help

Run Flags:
  -c, --config string    Path to config file (default: auto-detect)
  --port int            Proxy port (default: 3000)
  --app-port int        App port (default: 3001)
  --target string       Package to build (default: ".")
  --bin string          Output binary path (default: "./tmp/main")
  --build-cmd string    Custom build command
  --build-args string   Additional go build arguments
  --run-args string     Arguments to pass to binary
  --watch string        Comma-separated dirs to watch (default: ".")
  --ext string          Comma-separated extensions (default: "go")
  --poll duration       Polling interval (default: 500ms)

Examples:
  hot                              # Start with auto-detected config
  hot -c hot.toml               # Start with specific config
  hot --port 8080 --ext "go,html" # Override config with flags
  hot init                        # Generate hot.toml
  hot convert .air.toml           # Convert air config

For more information, visit: https://github.com/juanrgon/hot
`)
}

// cmdInit generates a hot.toml config file
func cmdInit() {
	outputFile := "hot.toml"

	// Check if file already exists
	if _, err := os.Stat(outputFile); err == nil {
		fmt.Printf("❌ %s already exists\n", outputFile)
		fmt.Println("Remove it first or use 'hot convert' to migrate from air")
		os.Exit(1)
	}

	// Read the embedded template from the current hot.toml if it exists
	template := `# hot.toml - Hot Development Server Configuration
# This file is optional - all settings can be overridden with CLI flags

# Root directory of the project
root = "."

# Temporary directory for build artifacts
tmp_dir = "tmp"

[build]
  # Custom build command (leave empty to use default 'go build')
  # Example: cmd = "make build-app"
  cmd = ""

  # Output binary path
  bin = "./tmp/main"

  # Package to build
  target = "."

  # Additional arguments for 'go build' (only if cmd is empty)
  build_args = []

  # Arguments to pass to the running binary
  run_args = []

  # Commands to run before each build (optional)
  pre_cmd = []

  # Commands to run after stopping (Ctrl+C) (optional)
  post_cmd = []

  # Delay before triggering rebuild after file change (milliseconds)
  delay = 200

  # Log build errors to this file (optional)
  error_log = ""

  # Stop running binary when build fails
  stop_on_error = true

  # Grace period before forcefully killing old process
  kill_delay = "1s"

[watch]
  # Directories to watch
  dirs = ["."]

  # File extensions to watch
  extensions = ["go", "html", "tpl", "tmpl"]

  # Directories to exclude from watching
  exclude_dirs = ["tmp", "vendor", ".git", "node_modules"]

  # Files to exclude from watching (exact matches)
  exclude_files = []

  # Regex patterns to exclude
  exclude_regex = [".*_test\\.go"]

  # Polling interval for file changes
  poll_interval = "500ms"

[proxy]
  # Port for the proxy server (access your app here)
  port = 9000

  # Port for the application server
  app_port = 8080

  # Enable browser auto-reload (disable for API-only servers)
  browser_reload = true

[screen]
  # Clear terminal on rebuild
  clear_on_rebuild = false

  # Keep scroll position after rebuild
  keep_scroll = true

[log]
  # Show timestamps in logs
  timestamps = false
`

	// Write the config file
	if err := os.WriteFile(outputFile, []byte(template), 0644); err != nil {
		fmt.Printf("❌ Failed to create %s: %v\n", outputFile, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Created %s\n", outputFile)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit hot.toml to customize your setup")
	fmt.Println("  2. Run 'hot' to start the dev server")
}

// cmdConvert converts an .air.toml config to hot.toml
func cmdConvert() {
	inputFile := ".air.toml"
	outputFile := "hot.toml"

	// Parse command line args for convert
	if len(os.Args) > 2 {
		inputFile = os.Args[2]
	}
	if len(os.Args) > 3 && os.Args[2] == "--output" || os.Args[2] == "-o" {
		outputFile = os.Args[3]
		if len(os.Args) > 4 {
			inputFile = os.Args[4]
		}
	}

	// Check if input file exists
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		fmt.Printf("❌ Input file not found: %s\n", inputFile)
		os.Exit(1)
	}

	// Check if output file already exists
	if _, err := os.Stat(outputFile); err == nil {
		fmt.Printf("❌ Output file already exists: %s\n", outputFile)
		fmt.Println("Remove it first or specify a different output with --output")
		os.Exit(1)
	}

	// Parse the air config
	type AirConfig struct {
		Root   string `toml:"root"`
		TmpDir string `toml:"tmp_dir"`
		Build  struct {
			Cmd           string   `toml:"cmd"`
			Bin           string   `toml:"bin"`
			FullBin       string   `toml:"full_bin"`
			ArgsBin       []string `toml:"args_bin"`
			IncludeExt    []string `toml:"include_ext"`
			ExcludeDir    []string `toml:"exclude_dir"`
			ExcludeFile   []string `toml:"exclude_file"`
			ExcludeRegex  []string `toml:"exclude_regex"`
			Delay         int      `toml:"delay"`
			StopOnError   bool     `toml:"stop_on_error"`
			Log           string   `toml:"log"`
			PollInterval  int      `toml:"poll_interval"`
		} `toml:"build"`
		Proxy struct {
			Enabled   bool `toml:"enabled"`
			ProxyPort int  `toml:"proxy_port"`
			AppPort   int  `toml:"app_port"`
		} `toml:"proxy"`
		Screen struct {
			ClearOnRebuild bool `toml:"clear_on_rebuild"`
			KeepScroll     bool `toml:"keep_scroll"`
		} `toml:"screen"`
		Log struct {
			Time bool `toml:"time"`
		} `toml:"log"`
	}

	var airCfg AirConfig
	if _, err := toml.DecodeFile(inputFile, &airCfg); err != nil {
		fmt.Printf("❌ Failed to parse %s: %v\n", inputFile, err)
		os.Exit(1)
	}

	// Convert to hot config
	hotCfg := Config{
		Root:   airCfg.Root,
		TmpDir: airCfg.TmpDir,
		Build: BuildConfig{
			Cmd:         airCfg.Build.Cmd,
			Bin:         airCfg.Build.Bin,
			Target:      ".",
			RunArgs:     airCfg.Build.ArgsBin,
			Delay:       airCfg.Build.Delay,
			ErrorLog:    airCfg.Build.Log,
			StopOnError: airCfg.Build.StopOnError,
			KillDelay:   "1s",
		},
		Watch: WatchConfig{
			Dirs:         []string{"."},
			Extensions:   airCfg.Build.IncludeExt,
			ExcludeDirs:  airCfg.Build.ExcludeDir,
			ExcludeFiles: airCfg.Build.ExcludeFile,
			ExcludeRegex: airCfg.Build.ExcludeRegex,
		},
		Proxy: ProxyConfig{
			Port:       airCfg.Proxy.ProxyPort,
			AppPort:    airCfg.Proxy.AppPort,
			BrowserReload: airCfg.Proxy.Enabled,
		},
		Screen: ScreenConfig{
			ClearOnRebuild: airCfg.Screen.ClearOnRebuild,
			KeepScroll:     airCfg.Screen.KeepScroll,
		},
		Log: LogConfig{
			Timestamps: airCfg.Log.Time,
		},
	}

	// Set defaults if not specified
	if hotCfg.Root == "" {
		hotCfg.Root = "."
	}
	if hotCfg.TmpDir == "" {
		hotCfg.TmpDir = "tmp"
	}
	if hotCfg.Build.Bin == "" {
		hotCfg.Build.Bin = "./tmp/main"
	}
	if hotCfg.Build.Delay == 0 {
		hotCfg.Build.Delay = 200
	}
	if hotCfg.Proxy.Port == 0 {
		hotCfg.Proxy.Port = 3000
	}
	if hotCfg.Proxy.AppPort == 0 {
		hotCfg.Proxy.AppPort = 3001
	}
	if len(hotCfg.Watch.Extensions) == 0 {
		hotCfg.Watch.Extensions = []string{"go"}
	}
	if hotCfg.Watch.PollInterval == "" {
		if airCfg.Build.PollInterval > 0 {
			hotCfg.Watch.PollInterval = fmt.Sprintf("%dms", airCfg.Build.PollInterval)
		} else {
			hotCfg.Watch.PollInterval = "500ms"
		}
	}

	// Encode to TOML
	var buf bytes.Buffer
	buf.WriteString("# hot.toml - Converted from " + inputFile + "\n")
	buf.WriteString("# Generated by hot convert\n\n")

	encoder := toml.NewEncoder(&buf)
	if err := encoder.Encode(hotCfg); err != nil {
		fmt.Printf("❌ Failed to encode config: %v\n", err)
		os.Exit(1)
	}

	// Write output file
	if err := os.WriteFile(outputFile, buf.Bytes(), 0644); err != nil {
		fmt.Printf("❌ Failed to write %s: %v\n", outputFile, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Converted %s to %s\n", inputFile, outputFile)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Review hot.toml for any needed adjustments")
	fmt.Println("  2. Run 'hot' to start the dev server")
	fmt.Println("\nNote: Some air features are not supported by hot:")
	fmt.Println("  - Color customization (hot has sensible defaults)")
	fmt.Println("  - full_bin (use run_args instead)")
	fmt.Println("  - Some advanced options (hot focuses on simplicity)")
}

func parseFlags() {
	var watch, ext, buildArgs, runArgs, configFile string
	var apiMode bool

	// Add -c/--config flag
	flag.StringVar(&configFile, "c", "", "Path to config file (default: auto-detect hot.toml or .air.toml)")
	flag.StringVar(&configFile, "config", "", "Path to config file (default: auto-detect hot.toml or .air.toml)")

	// API mode flag
	flag.BoolVar(&apiMode, "api-mode", false, "API mode: disable browser features (sets browser_reload = false)")

	// CLI flags (will override config file)
	flag.StringVar(&config.AppPort, "app-port", "", "Port for the Go application to listen on")
	flag.StringVar(&config.ProxyPort, "port", "", "Port for the proxy server (access this one)")
	flag.StringVar(&config.BuildTarget, "target", "", "Go package to build")
	flag.StringVar(&config.BuildOutput, "bin", "", "Path to output binary")
	flag.StringVar(&config.BuildCmd, "build-cmd", "", "Custom build command (if empty, uses 'go build')")
	flag.StringVar(&watch, "watch", "", "Comma-separated directories to watch recursively")
	flag.StringVar(&ext, "ext", "", "Comma-separated extensions to watch")
	flag.StringVar(&buildArgs, "build-args", "", "Additional arguments for 'go build'")
	flag.StringVar(&runArgs, "run-args", "", "Additional arguments for the running app")
	var poll time.Duration
	flag.DurationVar(&poll, "poll", 0, "File polling interval")
	flag.Parse()

	// Find and load config file
	if configFile == "" {
		configFile = findConfigFile()
	}

	var cfg *Config
	if configFile != "" {
		logger.Debug("Loading config from: " + configFile)
		var err error
		cfg, err = loadConfig(configFile)
		if err != nil {
			logger.Error("Failed to load config: " + err.Error())
			os.Exit(1)
		}
	} else {
		// No config file found, use defaults
		cfg, _ = loadConfig("")
	}

	// Merge config file with CLI flags (CLI flags take precedence)
	if config.AppPort == "" {
		config.AppPort = strconv.Itoa(cfg.Proxy.AppPort)
	}
	if config.ProxyPort == "" {
		config.ProxyPort = strconv.Itoa(cfg.Proxy.Port)
	}
	if config.BuildTarget == "" {
		config.BuildTarget = cfg.Build.Target
	}
	if config.BuildOutput == "" {
		config.BuildOutput = cfg.Build.Bin
	}
	if config.BuildCmd == "" {
		config.BuildCmd = cfg.Build.Cmd
	}
	if watch == "" {
		if len(cfg.Watch.Dirs) > 0 {
			watch = strings.Join(cfg.Watch.Dirs, ",")
		} else {
			watch = "."
		}
	}
	if ext == "" {
		if len(cfg.Watch.Extensions) > 0 {
			ext = strings.Join(cfg.Watch.Extensions, ",")
		} else {
			ext = "go,gohtml,html,tpl"
		}
	}
	if buildArgs == "" && len(cfg.Build.BuildArgs) > 0 {
		config.BuildArgs = cfg.Build.BuildArgs
	} else if buildArgs != "" {
		config.BuildArgs = strings.Fields(buildArgs)
	}
	if runArgs == "" && len(cfg.Build.RunArgs) > 0 {
		config.RunArgs = cfg.Build.RunArgs
	} else if runArgs != "" {
		config.RunArgs = strings.Fields(runArgs)
	}
	if poll == 0 {
		if cfg.Watch.PollInterval != "" {
			var err error
			poll, err = time.ParseDuration(cfg.Watch.PollInterval)
			if err != nil {
				logger.Yellow("Invalid poll_interval in config, using default")
				poll = 500 * time.Millisecond
			}
		} else {
			poll = 500 * time.Millisecond
		}
	}
	config.PollInterval = poll

	// Handle browser reload (apiMode flag overrides config)
	if apiMode {
		config.BrowserReload = false
	} else {
		config.BrowserReload = cfg.Proxy.BrowserReload
	}

	// Store exclude regex patterns
	config.ExcludeRegex = cfg.Watch.ExcludeRegex

	// Parse watch directories and extensions
	config.WatchDirs = strings.Split(watch, ",")
	config.Extensions = make(map[string]bool)
	for _, e := range strings.Split(ext, ",") {
		config.Extensions["."+strings.TrimSpace(e)] = true
	}
}

// --- Orchestrator (Manages Build/Run Lifecycle) ---

// Broadcaster manages Server-Sent Events (SSE) connections for browser auto-reload.
type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan string]bool
}

// NewBroadcaster creates a new Broadcaster instance.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[chan string]bool),
	}
}

// Subscribe registers a new subscriber and returns a channel for receiving messages.
func (b *Broadcaster) Subscribe() chan string {
	ch := make(chan string, 10)
	b.mu.Lock()
	b.subscribers[ch] = true
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *Broadcaster) Unsubscribe(ch chan string) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	close(ch)
	b.mu.Unlock()
}

// Broadcast sends a message to all subscribers.
func (b *Broadcaster) Broadcast(msg string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- msg:
		default:
			// Skip if channel is full to avoid blocking
		}
	}
}

// --- Orchestrator (Manages Build/Run Lifecycle) ---

// AppState represents the current state of the application.
type AppState int

const (
	// StateStopped indicates the app is not running.
	StateStopped AppState = iota

	// StateBuilding indicates the app is being built.
	StateBuilding

	// StateRunning indicates the app is running.
	StateRunning

	// StateError indicates the app is in an error state.
	StateError
)

// Orchestrator manages the build and run lifecycle of the application.
type Orchestrator struct {
	sync.RWMutex
	state       AppState
	readyCond   *sync.Cond
	cmd         *exec.Cmd
	restartCh   chan string
	broadcaster *Broadcaster
}

// NewOrchestrator creates a new Orchestrator instance.
func NewOrchestrator() *Orchestrator {
	o := &Orchestrator{
		state:       StateStopped,
		restartCh:   make(chan string, 1),
		broadcaster: NewBroadcaster(),
	}
	o.readyCond = sync.NewCond(&o.RWMutex)
	go o.runLoop()
	return o
}

// TriggerRestart queues a restart request without blocking.
func (o *Orchestrator) TriggerRestart(reason string) {
	select {
	case o.restartCh <- reason:
	default:
		// deeply already queued
	}
}

func (o *Orchestrator) setState(s AppState) {
	o.Lock()
	o.state = s
	// Broadcast to wake up any proxy requests waiting for StateRunning
	o.readyCond.Broadcast()
	o.Unlock()

	// Trigger browser reload when app becomes ready
	if s == StateRunning {
		logger.Info("📡 Broadcasting reload to browsers...")
		o.broadcaster.Broadcast("reload")
	}
}

// WaitUntilRunning blocks until the application is running or in an error state.
func (o *Orchestrator) WaitUntilRunning() bool {
	o.Lock()
	defer o.Unlock()
	for o.state == StateBuilding || o.state == StateStopped {
		o.readyCond.Wait()
	}
	return o.state == StateRunning
}

func (o *Orchestrator) runLoop() {
	for reason := range o.restartCh {
		cycleStart := time.Now()
		logger.Cyan("⟳ Restarting: " + reason)
		o.Kill()

		// Building phase
		logger.Info("🔨 Building...")
		o.broadcaster.Broadcast("building")
		o.setState(StateBuilding)
		buildStart := time.Now()
		if err := o.build(); err != nil {
			logger.Error("Build failed: " + err.Error())
			o.broadcaster.Broadcast("build_failed")
			o.setState(StateError)
			continue
		}
		logger.Debug(fmt.Sprintf("Build complete in %v", time.Since(buildStart).Round(time.Millisecond)))

		// Starting phase
		logger.Info("🚀 Starting app...")
		o.broadcaster.Broadcast("starting")
		if err := o.start(); err != nil {
			logger.Error("Failed to start: " + err.Error())
			o.broadcaster.Broadcast("start_failed")
			o.setState(StateError)
			continue
		}

		// Wait for port to be open before declaring officially running
		if o.waitForPort(5 * time.Second) {
			logger.Green(fmt.Sprintf("✓ App is ready on :%s (total: %v)", config.AppPort, time.Since(cycleStart).Round(time.Millisecond)))
			o.setState(StateRunning)
		} else {
			logger.Error("Timed out waiting for app to bind to :" + config.AppPort)
			o.Kill()
			o.setState(StateError)
		}
	}
}

func (o *Orchestrator) build() error {
	var cmd *exec.Cmd
	if config.BuildCmd != "" {
		// Use custom build command
		cmd = exec.Command("sh", "-c", config.BuildCmd)
	} else {
		// Use default go build
		args := []string{"build", "-o", config.BuildOutput}
		args = append(args, config.BuildArgs...)
		args = append(args, config.BuildTarget)
		cmd = exec.Command("go", args...)
	}

	// Capture output for display if it fails
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		fmt.Print(output.String())
		return err
	}
	return nil
}

func (o *Orchestrator) start() error {
	cmd := exec.Command(config.BuildOutput, config.RunArgs...)
	// Set environment so the app knows its port
	cmd.Env = append(os.Environ(), "PORT="+config.AppPort)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set process group to ensure we can kill children if needed
	setProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		return err
	}
	o.cmd = cmd

	// Monitor process exit in background
	go func(c *exec.Cmd) {
		if err := c.Wait(); err != nil {
			// Process crashed or exited with error
			// We only care if this is still the active command
			o.Lock()
			if o.cmd == c && o.state == StateRunning {
				o.Unlock()
				logger.Yellow("App exited unexpectedly")
				o.setState(StateError)
			} else {
				o.Unlock()
			}
		}
	}(cmd)

	return nil
}

// Kill terminates the running application process, if any.
func (o *Orchestrator) Kill() {
	o.Lock()
	defer o.Unlock()
	if o.cmd != nil && o.cmd.Process != nil {
		// Try graceful interrupt first
		// On Windows this usually just forces kill anyway, but good practice for *nix
		killProcessGroup(o.cmd)
		o.cmd = nil
	}
}

func (o *Orchestrator) waitForPort(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "localhost:"+config.AppPort, 100*time.Millisecond)
		if err == nil {
			err = conn.Close()
			if err != nil {
				logger.Yellow("Warning: error closing test connection: " + err.Error())
			}
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// --- Proxy Server ---

const reloadScript = `<script>
(function() {
  if (window.__hot_reload) return;
  window.__hot_reload = true;

  // Create status indicator
  const status = document.createElement('div');
  status.id = '__hot_status';
  status.style.cssText = 'position:fixed;bottom:20px;right:20px;padding:12px 20px;border-radius:8px;font-family:system-ui,-apple-system,sans-serif;font-size:14px;font-weight:500;z-index:999999;transition:all 0.3s ease;box-shadow:0 4px 12px rgba(0,0,0,0.15);display:none';

  function showStatus(text, color, emoji) {
    status.textContent = emoji + ' ' + text;
    status.style.backgroundColor = color;
    status.style.color = '#fff';
    status.style.display = 'block';
    document.body.appendChild(status);
  }

  function hideStatus() {
    status.style.opacity = '0';
    setTimeout(function() {
      status.style.display = 'none';
      status.style.opacity = '1';
    }, 300);
  }

  const events = new EventSource('/__hot/reload');

  events.onmessage = function(e) {
    if (e.data === 'building') {
      showStatus('Building...', '#3b82f6', '🔨');
    } else if (e.data === 'starting') {
      showStatus('Starting app...', '#8b5cf6', '🚀');
    } else if (e.data === 'reload') {
      showStatus('Reloading...', '#10b981', '✓');
      setTimeout(function() {
        window.location.reload();
      }, 200);
    } else if (e.data === 'build_failed') {
      showStatus('Build failed', '#ef4444', '✗');
    } else if (e.data === 'start_failed') {
      showStatus('Start failed', '#ef4444', '✗');
    }
  };

  events.onerror = function() {
    showStatus('Reconnecting...', '#f59e0b', '⟳');
    events.close();
    setTimeout(function() {
      window.location.reload();
    }, 1000);
  };

  window.addEventListener('beforeunload', function() {
    events.close();
  });
})();
</script>`

// injectReloadScript injects the auto-reload script before the closing </body> tag.
func injectReloadScript(html []byte) []byte {
	bodyEnd := bytes.LastIndex(html, []byte("</body>"))
	if bodyEnd == -1 {
		return html // No </body> tag, skip injection
	}

	result := make([]byte, 0, len(html)+len(reloadScript))
	result = append(result, html[:bodyEnd]...)
	result = append(result, []byte(reloadScript)...)
	result = append(result, html[bodyEnd:]...)
	return result
}

// sseReloadHandler serves the Server-Sent Events endpoint for browser auto-reload.
func sseReloadHandler(orch *Orchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set headers for Server-Sent Events
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Subscribe to reload events
		ch := orch.broadcaster.Subscribe()
		defer orch.broadcaster.Unsubscribe(ch)

		// Send initial connection message
		_, err := fmt.Fprintf(w, "data: connected\n\n")
		if err != nil {
			return
		}
		flusher.Flush()

		// Stream reload events
		for {
			select {
			case msg := <-ch:
				_, err := fmt.Fprintf(w, "data: %s\n\n", msg)
				if err != nil {
					return
				}
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}

func startProxyServer(orch *Orchestrator) {
	targetURL, _ := url.Parse("http://localhost:" + config.AppPort)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Inject reload script into HTML responses (only if browser_reload is enabled)
	proxy.ModifyResponse = func(resp *http.Response) error {
		// Skip injection if browser reload is disabled
		if !config.BrowserReload {
			return nil
		}

		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/html") {
			return nil
		}

		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if err = resp.Body.Close(); err != nil {
			return err
		}

		// Inject the reload script
		injected := injectReloadScript(body)

		// Update the response
		resp.Body = io.NopCloser(bytes.NewReader(injected))
		resp.ContentLength = int64(len(injected))
		resp.Header.Set("Content-Length", strconv.Itoa(len(injected)))

		return nil
	}

	// Custom error handler for when the app crashes *during* a request
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Yellow(fmt.Sprintf("Proxy error: %v", err))
		if !orch.WaitUntilRunning() {
			renderError(w, "Application failed to start. Check your terminal.")
			return
		}
		// If we got here, it might be a transient connection drop, try one reload if you want,
		// but usually 502 is appropriate.
		w.WriteHeader(http.StatusBadGateway)
		_, err = fmt.Fprintf(w, "Bad Gateway: Application unavailable")
		if err != nil {
			logger.Yellow("Error writing bad gateway response: " + err.Error())
		}
	}

	// Create router for handling SSE endpoint and proxying
	mux := http.NewServeMux()

	// SSE endpoint for browser auto-reload
	mux.HandleFunc("/__hot/reload", sseReloadHandler(orch))

	// Proxy all other requests
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// The Magic: Wait here if the app is rebuilding.
		// This holds the browser connection open until the new app is ready.
		if !orch.WaitUntilRunning() {
			// If Wait returns false, it means we are in Error state
			renderError(w, "Build Error. Check your terminal for details.")
			return
		}
		proxy.ServeHTTP(w, r)
	})

	logger.Info("Proxy listening on :" + config.ProxyPort)
	if err := http.ListenAndServe(":"+config.ProxyPort, mux); err != nil {
		logger.Fatal("Proxy failed to start: " + err.Error())
	}
}

func renderError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusInternalServerError)
	// Simple auto-reloading error page
	_, err := fmt.Fprintf(w, `<html><head><meta http-equiv="refresh" content="2"></head>
<body style="font-family:sans-serif;background:#222;color:#e74c3c;text-align:center;padding-top:100px;">
<h1>Hot Error</h1><p>%s</p><p style="color:#999">Waiting for fix...</p>
</body></html>`, msg)
	if err != nil {
		logger.Yellow("Error writing error page: " + err.Error())
	}
}

// --- Polling File Watcher (Stdlib only) ---
// Uses polling to avoid OS-specific syscalls or external dependencies in a single file.

func startPollingWatcher(events chan<- string) {
	type fileInfo struct {
		mtime time.Time
		size  int64
		mode  os.FileMode
	}
	seen := make(map[string]fileInfo)

	// Compile exclude regex patterns
	var excludePatterns []*regexp.Regexp
	for _, pattern := range config.ExcludeRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			logger.Yellow(fmt.Sprintf("Invalid exclude_regex pattern '%s': %v", pattern, err))
			continue
		}
		excludePatterns = append(excludePatterns, re)
	}

	for {
		time.Sleep(config.PollInterval)
		currentScan := make(map[string]bool)

		for _, dir := range config.WatchDirs {
			err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				// Ignore hidden directories (like .git)
				if info.IsDir() && strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
					return filepath.SkipDir
				}
				if info.IsDir() {
					return nil
				}
				// Check extension
				ext := filepath.Ext(path)
				if !config.Extensions[ext] {
					return nil
				}

				// Check exclude regex patterns
				for _, re := range excludePatterns {
					if re.MatchString(path) {
						return nil
					}
				}

				currentScan[path] = true
				lastInfo, exists := seen[path]
				currentInfo := fileInfo{mtime: info.ModTime(), size: info.Size(), mode: info.Mode()}

				if !exists {
					// New file found (don't trigger on initial scan, just add)
					seen[path] = currentInfo
				} else if info.ModTime().After(lastInfo.mtime) || info.Size() != lastInfo.size {
					// Check what changed
					sizeChanged := info.Size() != lastInfo.size
					modeChanged := info.Mode() != lastInfo.mode
					mtimeChanged := info.ModTime().After(lastInfo.mtime)

					// If only mode changed (permissions), ignore it
					if modeChanged && !sizeChanged && !mtimeChanged {
						logger.Debug(fmt.Sprintf("Ignoring permission change: %s (mode: %v->%v)",
							path, lastInfo.mode, info.Mode()))
						seen[path] = currentInfo
						return nil
					}

					// Real content change detected (mtime or size changed)
					logger.Debug(fmt.Sprintf("File changed: %s (size: %d->%d, mtime: %s->%s, mode: %v->%v)",
						path,
						lastInfo.size, info.Size(),
						lastInfo.mtime.Format("15:04:05.000"),
						info.ModTime().Format("15:04:05.000"),
						lastInfo.mode, info.Mode()))

					seen[path] = currentInfo
					// Non-blocking send to avoid deadlocks if consumer is slow
					select {
					case events <- path:
					default:
					}
				}
				return nil
			})

			if err != nil {
				logger.Yellow("Watcher error: " + err.Error())
			}
		}

		// Optional: cleanup 'seen' for deleted files if needed,
		// but for a dev tool, growing memory slightly isn't catastrophic.
	}
}

// --- Helpers ---

var logger = &logWriter{}

type logWriter struct{}

func (l *logWriter) Info(msg string)   { l.log(ansi.Blue, "INFO", msg) }
func (l *logWriter) Green(msg string)  { l.log(ansi.Green, "DONE", msg) }
func (l *logWriter) Cyan(msg string)   { l.log(ansi.Cyan, "WAIT", msg) }
func (l *logWriter) Yellow(msg string) { l.log(ansi.Yellow, "WARN", msg) }
func (l *logWriter) Error(msg string)  { l.log(ansi.Red, "FAIL", msg) }
func (l *logWriter) Fatal(msg string)  { l.log(ansi.Magenta, "EXIT", msg); os.Exit(1) }
func (l *logWriter) Debug(msg string)  { l.log(ansi.Magenta, "PERF", msg) }

func (l *logWriter) log(color, level, msg string) {
	fmt.Printf("%s%s [hot] %s %s\n", color, time.Now().Format("15:04:05"), level, msg+ansi.Reset)
}

// -- OS Specific Process Management --

func setProcessGroup(cmd *exec.Cmd) {
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
}

func killProcessGroup(cmd *exec.Cmd) {
	if runtime.GOOS == "windows" {
		err := cmd.Process.Kill()
		if err != nil {
			logger.Yellow("Warning: failed to kill process: " + err.Error())
		}
	} else {
		// Try to kill the whole process group
		pgid, err := syscall.Getpgid(cmd.Process.Pid)
		if err == nil {
			err = syscall.Kill(-pgid, syscall.SIGTERM)
			if err != nil {
				logger.Yellow("Warning: failed to kill process group: " + err.Error())
			}
		} else {
			err = cmd.Process.Signal(syscall.SIGTERM)
			if err != nil {
				logger.Yellow("Warning: failed to send SIGTERM: " + err.Error())
			}
		}
	}
	// Ensure it's dead if SIGTERM didn't work quickly
	go func() {
		time.Sleep(1 * time.Second)
		if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
			if runtime.GOOS != "windows" {
				pgid, err := syscall.Getpgid(cmd.Process.Pid)
				if err == nil {
					err = syscall.Kill(-pgid, syscall.SIGKILL)
					if err != nil {
						logger.Yellow("Warning: failed to kill process group: " + err.Error())
					}
				}
			}
			err := cmd.Process.Kill()
			if err != nil {
				logger.Yellow("Warning: failed to kill process: " + err.Error())
			}
		}
	}()
}
