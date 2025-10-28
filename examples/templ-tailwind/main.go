package main

import (
	"log"
	"net/http"
)

func main() {
	// Serve static files (CSS)
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Routes
	http.HandleFunc("/", handleHome)

	port := ":8080"
	log.Printf("🌐 Server starting on http://localhost%s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	// In a real app with templ, you would render a templ component here
	// For this example, we'll use inline HTML to demonstrate the structure
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Templ + Tailwind Example</title>
    <link href="/static/output.css" rel="stylesheet">
    <script src="http://localhost:3000/livereload.js"></script>
</head>
<body class="bg-gray-100">
    <div class="container mx-auto px-4 py-8">
        <div class="max-w-2xl mx-auto bg-white rounded-lg shadow-lg p-8">
            <h1 class="text-4xl font-bold text-orange-500 mb-4">🔥 Templ + Tailwind</h1>
            <p class="text-gray-700 mb-4">
                This example demonstrates hot reloading with Templ templates and Tailwind CSS.
            </p>
            
            <div class="bg-blue-50 border-l-4 border-blue-500 p-4 mb-4">
                <p class="text-blue-700">
                    <strong>Note:</strong> In a real application, you would use templ components here.
                    Install templ with: <code class="bg-gray-200 px-2 py-1 rounded">go install github.com/a-h/templ/cmd/templ@latest</code>
                </p>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mt-6">
                <div class="bg-gradient-to-r from-purple-400 to-pink-500 rounded-lg p-6 text-white">
                    <h3 class="text-xl font-bold mb-2">Feature 1</h3>
                    <p>Automatic template regeneration</p>
                </div>
                <div class="bg-gradient-to-r from-green-400 to-blue-500 rounded-lg p-6 text-white">
                    <h3 class="text-xl font-bold mb-2">Feature 2</h3>
                    <p>Live CSS compilation</p>
                </div>
            </div>

            <div class="mt-8 p-4 bg-yellow-50 rounded-lg">
                <h3 class="text-lg font-semibold text-yellow-800 mb-2">Try it out:</h3>
                <ol class="list-decimal list-inside text-gray-700 space-y-2">
                    <li>Edit this file and save</li>
                    <li>Modify <code class="bg-gray-200 px-2 py-1 rounded text-sm">input.css</code></li>
                    <li>Watch the browser reload automatically!</li>
                </ol>
            </div>
        </div>
    </div>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
