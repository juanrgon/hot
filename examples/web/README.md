# Web Server Example

This example demonstrates a simple web server with hot reload and automatic browser refresh.

## Running

```bash
# From this directory
hot --mode web

# Or specify the path
hot --mode web --watch ./examples/web
```

## What it does

1. Starts a web server on port 8080
2. Enables live reload server on port 3000
3. Watches for changes to Go files
4. Automatically rebuilds and restarts the server
5. Reloads your browser when changes are detected

## Try it

1. Run the command above
2. Visit http://localhost:5173 in your browser (this proxy autoloads the live-reload script)
3. Edit `main.go` and save
4. Watch your browser automatically reload!

> Want to hit the app directly on http://localhost:8080 instead? Either add
> `<script src="http://localhost:3000/livereload.js"></script>` to your HTML
> or start `hot` with `--proxy-listen 0` to disable the injector proxy.

## Browser Console

Open your browser's developer console and you should see:
```
🔥 Hot reload connected
```

When you save a file, you'll see:
```
🔄 Reloading page...
```
