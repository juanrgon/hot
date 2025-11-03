# Hot - Zero-Dependency Hot Reload for Go

> The fastest, simplest way to develop Go web applications with live reload.

[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.21-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

## Why Hot?

### vs Air

- ⚡ **Minimal dependencies** - Only uses BurntSushi/toml for config files
- 🔒 **No connection refused** - Intelligent proxy holds requests during rebuilds
- 🎨 **Browser integration** - Live status updates without page refresh
- 📦 **Smaller & faster** - Pure Go, no external watchers
- 🚀 **Better DX** - Visual feedback, auto-reload just works

### Features

- ✨ Integrated HTTP proxy with request queuing
- 🔄 Browser auto-reload via Server-Sent Events
- 📁 Configurable file watching (extensions, directories, excludes)
- 🔨 Custom build commands & hooks
- 🎯 Cross-platform (Windows, Linux, macOS)
- ⚡ Fast rebuilds with intelligent debouncing
- 🧹 Graceful process management
- 📝 Optional TOML config file

## Quick Start

### Installation

```bash
# Install from GitHub
go install github.com/juanrgon/hot@latest

# Or clone and build
git clone https://github.com/juanrgon/hot
cd hot
go build -o hot main.go
```

### Usage

```bash
# In your Go project directory
hot init    # Optional: generate hot.toml
hot         # Start the dev server
```

Access your app at `http://localhost:9000` 🎉

## Configuration

### Generate Config

```bash
hot init              # Creates hot.toml
```

### Example: hot.toml

```toml
[build]
  cmd = "go build -o ./tmp/main ."
  bin = "./tmp/main"
  run_args = ["--port", "8080"]

[watch]
  extensions = ["go", "html", "templ"]
  exclude_dirs = ["tmp", "vendor", ".git"]

[proxy]
  port = 9000
  app_port = 8080
```

### All Config Options

| Section | Option | Type | Default | Description |
|---------|--------|------|---------|-------------|
| **Root** | `root` | string | "." | Project root directory |
| | `tmp_dir` | string | "tmp" | Temporary directory for build artifacts |
| **[build]** | `cmd` | string | "" | Custom build command (empty = use default go build) |
| | `bin` | string | "./tmp/main" | Output binary path |
| | `target` | string | "." | Package to build |
| | `build_args` | []string | [] | Additional go build arguments |
| | `run_args` | []string | [] | Arguments to pass to binary |
| | `pre_cmd` | []string | [] | Commands to run before each build |
| | `post_cmd` | []string | [] | Commands to run after stopping |
| | `delay` | int | 200 | Debounce delay (ms) after file change |
| | `error_log` | string | "" | Log build errors to file |
| | `stop_on_error` | bool | true | Stop binary when build fails |
| | `kill_delay` | string | "1s" | Grace period before force kill |
| **[watch]** | `dirs` | []string | ["."] | Directories to watch |
| | `extensions` | []string | ["go"] | File extensions to watch |
| | `exclude_dirs` | []string | ["tmp", "vendor", ".git"] | Directories to exclude |
| | `exclude_files` | []string | [] | Files to exclude |
| | `exclude_regex` | []string | [] | Regex patterns to exclude |
| | `poll_interval` | string | "500ms" | File polling interval |
| **[proxy]** | `port` | int | 9000 | Proxy server port |
| | `app_port` | int | 8080 | Application server port |
| | `browser_reload` | bool | true | Enable browser auto-reload |
| **[screen]** | `clear_on_rebuild` | bool | false | Clear terminal on rebuild |
| | `keep_scroll` | bool | true | Maintain scroll position |
| **[log]** | `timestamps` | bool | false | Show timestamps in logs |

## Migrating from Air

### Automatic Conversion

```bash
hot convert .air.toml     # Creates hot.toml
```

### Manual Migration

| Air Config | Hot Config | Notes |
|------------|--------------|-------|
| `[build].cmd` | `[build].cmd` | Direct mapping |
| `[build].bin` | `[build].bin` | Direct mapping |
| `[build].args_bin` | `[build].run_args` | Renamed for clarity |
| `[build].include_ext` | `[watch].extensions` | Moved to watch section |
| `[build].exclude_dir` | `[watch].exclude_dirs` | Moved to watch section |
| `[build].exclude_file` | `[watch].exclude_files` | Moved to watch section |
| `[build].exclude_regex` | `[watch].exclude_regex` | Moved to watch section |
| `[build].poll_interval` | `[watch].poll_interval` | Moved to watch section |
| `[proxy].proxy_port` | `[proxy].port` | Renamed |
| `[proxy].enabled` | Always enabled | Simplified |
| `[log].time` | `[log].timestamps` | Renamed |
| `[color].*` | Not supported | Sensible defaults |

### What Changes?

```bash
# Instead of: air -c .air.toml
hot

# Or with config:
hot -c hot.toml
```

### Benefits After Migration

✅ Faster startup (fewer dependencies)
✅ Better error messages during rebuild
✅ Browser status indicators (no manual refresh needed)
✅ Fewer config options to maintain
✅ Works on any Go-compatible platform

## How It Works

1. **Proxy Layer**: Hot starts a proxy server on port 9000
2. **App Server**: Your Go app runs on port 8080 (configurable)
3. **File Watcher**: Monitors your files for changes
4. **Smart Rebuild**: On change, rebuilds and restarts your app
5. **Request Queuing**: HTTP requests wait during rebuild (no errors!)
6. **Auto-Reload**: Browser automatically reloads when ready

```
Browser → Hot Proxy (9000) → Your App (8080)
           ↓
      File Watcher → Rebuild → Restart → Release Queued Requests
                                         ↓
                              Browser receives reload event
```

## CLI Reference

### Commands

```bash
hot                    # Start dev server (default)
hot run                # Explicit run command
hot init               # Generate hot.toml
hot convert .air.toml  # Convert from air
hot version            # Show version
hot help               # Show help
```

### Flags

All flags override config file values:

```bash
-c, --config string    Path to config file (auto-detect: hot.toml or .air.toml)
--api-mode             API mode: disable browser features (sets browser_reload = false)
--port int             Proxy port (default: 9000)
--app-port int         App port (default: 8080)
--target string        Package to build (default: ".")
--bin string           Output binary path (default: "./tmp/main")
--build-cmd string     Custom build command
--build-args string    Additional go build arguments
--run-args string      Arguments to pass to binary
--watch string         Comma-separated dirs to watch (default: ".")
--ext string           Comma-separated extensions (default: "go")
--poll duration        Polling interval (default: 500ms)
```

### Examples

```bash
# Start with auto-detected config
hot

# Start with specific config
hot -c hot.toml

# Override config with flags
hot --port 8080 --ext "go,html,templ"

# Custom build command
hot --build-cmd "make build"

# Watch specific directories
hot --watch "cmd,pkg,internal" --ext "go,tmpl"
```

## Browser Status Indicators

Hot displays visual status updates in your browser:

- 🔨 **Building...** - Blue indicator during compilation
- 🚀 **Starting app...** - Purple indicator during app startup
- ✓ **Reloading...** - Green indicator before page refresh
- ✗ **Build failed** - Red indicator on build errors
- ⟳ **Reconnecting...** - Orange indicator on connection loss

These appear in the bottom-right corner automatically!

## API Mode

For pure API servers (no HTML UI), disable browser features:

**Quick way** (CLI flag):
```bash
hot --api-mode
```

**Permanent way** (config file):
```toml
[proxy]
  browser_reload = false
```

In API mode:
- ✅ Request queuing still works (no connection refused during rebuilds)
- ✅ Hot reload on file changes
- ❌ No browser auto-reload script injection
- ❌ No visual status indicators

Perfect for:
- REST APIs
- GraphQL servers
- gRPC servers with HTTP gateways
- JSON-only services

## Examples

### Basic HTTP Server

```go
// main.go
package main

import (
    "fmt"
    "net/http"
    "os"
)

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello from Hot!")
    })

    http.ListenAndServe(":"+port, nil)
}
```

```bash
# Run with hot
hot --ext "go"
```

### With Templates

```toml
# hot.toml
[build]
  cmd = "templ generate && go build -o ./tmp/app ."
  bin = "./tmp/app"

[watch]
  extensions = ["go", "templ", "html", "css"]
  exclude_regex = [".*_templ\\.go"]

[proxy]
  port = 7331
  app_port = 5555
```

### Custom Build Pipeline

```toml
# hot.toml
[build]
  cmd = "make build-dev"
  bin = "./tmp/app"
  pre_cmd = ["make generate"]
  post_cmd = ["make cleanup"]

[watch]
  dirs = ["cmd", "pkg", "internal"]
  extensions = ["go", "proto", "sql"]
```

## Troubleshooting

### Port Already in Use

```bash
# Change ports
hot --port 9001 --app-port 8081

# Or kill existing process
lsof -ti:9000 | xargs kill
```

### Build Errors Not Showing

```bash
# Enable error logging
[build]
  error_log = "build-errors.log"
```

### App Not Reloading

Check that:
1. Your app reads `PORT` from environment
2. Browser extensions aren't blocking SSE
3. Config file is being detected (`hot` shows "Loading config from...")

### File Changes Not Detected

```bash
# Increase poll interval
hot --poll 1s

# Or watch more directories
hot --watch "cmd,pkg,web"
```

## FAQ

**Q: Does this work with Docker?**
A: Yes! Mount your code and run hot inside the container.

**Q: Can I use this in production?**
A: No. Hot is a development tool only.

**Q: Why polling instead of fsnotify?**
A: Minimal dependencies, works everywhere, no CGO issues.

**Q: Can I disable browser auto-reload?**
A: Use `--api-mode` flag or set `[proxy].browser_reload = false` in config.

**Q: Does it work with React/Vue frontends?**
A: Yes! The proxy forwards all requests to your app.

## Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

- Inspired by [air](https://github.com/air-verse/air)
- Built for the Go community
- Made with ❤️ by developers, for developers

---

**Made with Hot? [Share your experience!](https://github.com/juanrgon/hot/discussions)**
