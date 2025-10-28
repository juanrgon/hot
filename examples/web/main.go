package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/about", handleAbout)

	port := ":8080"
	log.Printf("🌐 Web server starting on http://localhost%s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Hot Reload Demo</title>
    <script src="http://localhost:3000/livereload.js"></script>
    <style>
        body {
            font-family: system-ui, -apple-system, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            line-height: 1.6;
        }
        h1 { color: #ff6b35; }
        .info { background: #f0f0f0; padding: 15px; border-radius: 5px; }
        a { color: #ff6b35; text-decoration: none; }
        a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <h1>🔥 Hot Reload Demo</h1>
    <p>This is a simple web server with hot reload enabled!</p>
    
    <div class="info">
        <strong>Current time:</strong> ` + time.Now().Format("2006-01-02 15:04:05") + `<br>
        <strong>Try this:</strong> Edit this file and save. Your browser will reload automatically!
    </div>
    
    <p><a href="/about">About Page</a></p>
</body>
</html>`
	fmt.Fprint(w, html)
}

func handleAbout(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>About - Hot Reload Demo</title>
    <script src="http://localhost:3000/livereload.js"></script>
    <style>
        body {
            font-family: system-ui, -apple-system, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            line-height: 1.6;
        }
        h1 { color: #ff6b35; }
        a { color: #ff6b35; text-decoration: none; }
        a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <h1>About Hot Reload</h1>
    <p>Hot reload makes development faster by automatically rebuilding and reloading your application when files change.</p>
    <p><a href="/">Back to Home</a></p>
</body>
</html>`
	fmt.Fprint(w, html)
}
