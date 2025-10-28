# hot 🔥

A fast and simple hot-reloading tool for Go applications with smart defaults. An alternative to `air` that requires minimal configuration for common use cases.

## Features

- 🚀 **Zero config** - Works out of the box for most Go projects
- 🌐 **Web mode** - Automatic browser reload via Server-Sent Events (SSE)
- 🔌 **API mode** - Quick restart without browser reload overhead
- 📦 **Templ support** - Automatically runs `templ generate` on `.templ` file changes
- 🎨 **Tailwind support** - Watches and rebuilds Tailwind CSS
- ⚡ **Fast** - Efficient file watching with smart debouncing
- 🎯 **Smart defaults** - Excludes common directories like `vendor`, `node_modules`, `.git`

## Installation

### Using `go install`

```bash
go install github.com/juanrgon/hot@latest
```

### Building from source

```bash
git clone https://github.com/juanrgon/hot.git
cd hot
go build -o hot
sudo mv hot /usr/local/bin/
```

### Manual download

Download the latest binary from the [releases page](https://github.com/juanrgon/hot/releases).

## Quick Start

### Basic Go Application

Navigate to your Go project directory and run:

```bash
hot
```

That's it! The tool will:
- Watch for `.go` file changes
- Rebuild your application
- Restart the process automatically

## Usage Examples

### Web Server with Browser Reload

For web applications that serve HTML, use web mode to get automatic browser reloading:

```bash
hot --mode web
```

Add this script tag to your HTML templates to enable live reload:

```html
<script src="http://localhost:3000/livereload.js"></script>
```

Your browser will automatically reload when you save changes to any Go file!

**Example with custom port:**

```bash
hot --mode web --port 3001
```

### API Server (No Browser Reload)

For REST APIs or services without a UI, use API mode:

```bash
hot --mode api
```

This mode skips the browser reload server entirely, making it lighter and faster for backend-only development.

### Web Server with Templ and Tailwind

For modern Go web apps using [templ](https://templ.guide/) and Tailwind CSS:

```bash
hot --mode web --templ --tailwind
```

This will:
- Watch `.go` and `.templ` files
- Run `templ generate` when `.templ` files change
- Watch `.css`, `.html`, and `.js` files for Tailwind
- Run `tailwindcss` to rebuild your styles
- Reload the browser automatically

**Note:** Make sure you have `templ` and `tailwindcss` installed and available in your PATH.

### Custom Build and Run Commands

```bash
hot --build "go build -o bin/myapp cmd/server/main.go" --run "./bin/myapp"
```

### Watch Specific Directories

```bash
hot --watch "./cmd,./pkg,./internal"
```

### Watch Additional File Extensions

```bash
hot --watch . --exts ".go,.html,.css"
```

### Exclude Specific Directories

```bash
hot --exclude "vendor,tmp,dist,testdata"
```

## Configuration Options

| Flag | Default | Description |
|------|---------|-------------|
| `--mode` | `web` | Mode: `web` (with browser reload) or `api` (no browser reload) |
| `--port` | `3000` | Port for the live reload server (web mode only) |
| `--build` | `go build -o /tmp/hot-build/app` | Custom build command |
| `--run` | `/tmp/hot-build/app` | Custom run command |
| `--watch` | `.` (current directory) | Comma-separated directories to watch |
| `--exts` | `.go` | Comma-separated file extensions to watch |
| `--exclude` | See below | Comma-separated directories to exclude |
| `--templ` | `false` | Watch `.templ` files and run `templ generate` |
| `--tailwind` | `false` | Watch for Tailwind files and rebuild CSS |
| `--tailwind-input` | `./input.css` | Tailwind input CSS file path |
| `--tailwind-output` | `./static/output.css` | Tailwind output CSS file path |
| `--version` | - | Show version information |

**Default excluded directories:** `vendor`, `node_modules`, `.git`, `.idea`, `.vscode`, `tmp`, `dist`, `build`

## How It Works

1. **File Watching**: Monitors your project for file changes using a polling-based watcher
2. **Smart Debouncing**: Waits 300ms after the last change to avoid rebuilding too frequently
3. **Build & Run**: Compiles your Go application and starts it
4. **Live Reload** (web mode): Maintains SSE connections with browsers and triggers reload on changes
5. **Process Management**: Cleanly stops the old process before starting the new one

## Comparison with Air

| Feature | hot | air |
|---------|-----|-----|
| Zero-config setup | ✅ Yes | ❌ Requires config file |
| Web mode with browser reload | ✅ Built-in | ⚠️ Requires proxy |
| API mode | ✅ Dedicated mode | ❌ Same as web |
| Templ integration | ✅ Flag-based | ❌ Manual setup |
| Tailwind integration | ✅ Flag-based | ❌ Manual setup |
| Configuration | 🎯 CLI flags or config file | 📄 Config file only |

## Project Structure Example

For a typical Go web application with templ and Tailwind:

```
myproject/
├── main.go
├── handlers/
│   ├── home.go
│   └── home.templ
├── static/
│   ├── output.css (generated)
│   └── ...
├── input.css
├── tailwind.config.js
└── go.mod
```

Run with:

```bash
hot --mode web --templ --tailwind
```

Add to your HTML:

```html
<link href="/static/output.css" rel="stylesheet">
<script src="http://localhost:3000/livereload.js"></script>
```

## Tips

1. **Faster builds**: Use `go build` flags for faster compilation during development:
   ```bash
   hot --build "go build -gcflags='all=-N -l' -o /tmp/app"
   ```

2. **Environment variables**: Pass environment variables to your app:
   ```bash
   hot --run "ENV=dev /tmp/hot-build/app"
   ```

3. **Multiple commands**: Chain commands in build or run:
   ```bash
   hot --build "go generate ./... && go build -o /tmp/app"
   ```

4. **Debugging**: Check the logs for build errors and runtime output

## Troubleshooting

### Browser not reloading

- Ensure the live reload script is included in your HTML
- Check that port 3000 (or your custom port) is not blocked
- Verify the browser console shows "🔥 Hot reload connected"

### Builds failing

- Check the build output in the console
- Verify your Go code compiles with `go build`
- Make sure dependencies are downloaded with `go mod download`

### High CPU usage

- Reduce watch scope with `--watch` to specific directories
- Add more exclusions with `--exclude`
- Check that you're not watching large generated directories

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE file for details

## Credits

Inspired by [air](https://github.com/cosmtrek/air) and other hot-reload tools, built with simplicity and smart defaults in mind.
