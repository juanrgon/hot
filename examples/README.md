# Hot Reload Examples

This directory contains example applications demonstrating different use cases for the `hot` reloading tool.

## Examples

### 1. Web Server (`web/`)

A simple web server with automatic browser reload.

**Use case:** HTML-based web applications, server-rendered sites

**Run:**
```bash
cd examples/web
hot --mode web
```

**Features:**
- Automatic browser refresh on file changes
- Live reload script injection
- Multiple page handling

---

### 2. API Server (`api/`)

A REST API server with hot reload but no browser reload.

**Use case:** REST APIs, GraphQL servers, microservices

**Run:**
```bash
cd examples/api
hot --mode api
```

**Features:**
- Fast restart without browser reload overhead
- JSON API endpoints
- Optimized for backend development

---

### 3. Templ + Tailwind (`templ-tailwind/`)

A web server using Templ templates and Tailwind CSS with integrated hot reload.

**Use case:** Modern Go web apps with templ and Tailwind

**Prerequisites:**
- Install templ: `go install github.com/a-h/templ/cmd/templ@latest`
- Install tailwindcss: `npm install -g tailwindcss`

**Run:**
```bash
cd examples/templ-tailwind
hot --mode web --templ --tailwind
```

**Features:**
- Automatic `templ generate` on `.templ` file changes
- Automatic Tailwind CSS compilation
- Browser reload on any change
- Full type-safe template development

---

## Quick Test

To quickly test all examples:

```bash
# Web server
cd examples/web && hot --mode web &
sleep 3 && curl http://localhost:8080 && kill %1

# API server  
cd examples/api && hot --mode api &
sleep 3 && curl http://localhost:8080/api/status && kill %1

# Templ + Tailwind (requires templ and tailwindcss installed)
cd examples/templ-tailwind && hot --mode web --templ --tailwind
```

## General Tips

1. **Port conflicts**: If port 8080 is in use, modify the port in `main.go`
2. **Live reload port**: The live reload server uses port 3000 by default. Change with `--port`
3. **Custom commands**: Use `--build` and `--run` flags for custom build/run commands
4. **Watch specific dirs**: Use `--watch` to monitor specific directories only

## Creating Your Own Example

1. Create a new directory in `examples/`
2. Add a `main.go` file with your application
3. Include a `README.md` explaining the example
4. For web apps, include the live reload script:
   ```html
   <script src="http://localhost:3000/livereload.js"></script>
   ```

## Contributing

Feel free to add more examples! Common additions:
- gRPC server example
- WebSocket server example
- Multi-service example
- Database-backed application
- Full-stack example with frontend build tools
