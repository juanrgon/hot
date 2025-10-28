# Templ + Tailwind Example

This example demonstrates hot reloading with Templ templates and Tailwind CSS.

## Prerequisites

Install the required tools:

```bash
# Install templ
go install github.com/a-h/templ/cmd/templ@latest

# Install tailwindcss (requires Node.js)
npm install -g tailwindcss
```

## Running

```bash
# From this directory
hot --mode web --templ --tailwind
```

Or from the repository root:

```bash
hot --mode web --templ --tailwind --watch ./examples/templ-tailwind
```

## What it does

1. Starts a web server on port 8080
2. Enables live reload server on port 3000
3. Watches for changes to:
   - `.go` files → rebuilds the application
   - `.templ` files → runs `templ generate`, then rebuilds
   - `.css`, `.html`, `.js` files → runs `tailwindcss`, triggers browser reload
4. Automatically reloads your browser when changes are detected

## Project Structure

```
templ-tailwind/
├── main.go              # Main application
├── input.css            # Tailwind source CSS
├── tailwind.config.js   # Tailwind configuration
└── static/
    └── output.css       # Generated CSS (auto-generated)
```

## Try it

1. Run the command above
2. Open http://localhost:8080 in your browser
3. Edit `main.go` and save → browser reloads
4. Edit `input.css` and save → Tailwind rebuilds, browser reloads
5. Watch the terminal for build output

## Using with Real Templ Templates

To use actual templ templates:

1. Create a `.templ` file:
   ```templ
   package main

   templ Home() {
       <html>
           <head>
               <title>My App</title>
               <link href="/static/output.css" rel="stylesheet"/>
               <script src="http://localhost:3000/livereload.js"></script>
           </head>
           <body>
               <h1 class="text-4xl font-bold">Hello from Templ!</h1>
           </body>
       </html>
   }
   ```

2. Use it in your Go code:
   ```go
   func handleHome(w http.ResponseWriter, r *http.Request) {
       Home().Render(r.Context(), w)
   }
   ```

3. Run with hot reload:
   ```bash
   hot --mode web --templ --tailwind
   ```

The `--templ` flag will automatically run `templ generate` whenever `.templ` files change!

## Tailwind Configuration

The `tailwind.config.js` is configured to scan:
- All `.html` files
- All `.templ` files
- All `.go` files (for inline classes)

Adjust the `content` array to match your project structure.

## Notes

- Make sure `templ` and `tailwindcss` are in your PATH
- The first build might take a moment as Tailwind processes all files
- Subsequent builds are faster thanks to Tailwind's caching
