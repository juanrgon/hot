package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
)

type LiveReloadServer struct {
	port    int
	clients map[chan struct{}]bool
	mu      sync.Mutex
	server  *http.Server
}

func NewLiveReloadServer(port int) *LiveReloadServer {
	return &LiveReloadServer{
		port:    port,
		clients: make(map[chan struct{}]bool),
	}
}

func (s *LiveReloadServer) Start() error {
	mux := http.NewServeMux()
	
	// SSE endpoint for live reload
	mux.HandleFunc("/livereload", s.handleSSE)
	
	// Serve the live reload script
	mux.HandleFunc("/livereload.js", s.handleScript)
	
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.corsMiddleware(mux),
	}
	
	log.Printf("📡 Live reload server listening on :%d", s.port)
	return s.server.ListenAndServe()
}

func (s *LiveReloadServer) Stop() {
	if s.server != nil {
		s.server.Close()
	}
}

func (s *LiveReloadServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	
	clientCh := make(chan struct{})
	
	s.mu.Lock()
	s.clients[clientCh] = true
	s.mu.Unlock()
	
	defer func() {
		s.mu.Lock()
		delete(s.clients, clientCh)
		s.mu.Unlock()
		close(clientCh)
	}()
	
	// Send initial connection message
	fmt.Fprintf(w, "data: connected\n\n")
	w.(http.Flusher).Flush()
	
	// Wait for reload signal or client disconnect
	select {
	case <-clientCh:
		fmt.Fprintf(w, "data: reload\n\n")
		w.(http.Flusher).Flush()
	case <-r.Context().Done():
		return
	}
}

func (s *LiveReloadServer) handleScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	script := `
(function() {
	var source = new EventSource('http://localhost:` + fmt.Sprintf("%d", s.port) + `/livereload');
	
	source.addEventListener('open', function(e) {
		console.log('🔥 Hot reload connected');
	});
	
	source.addEventListener('message', function(e) {
		if (e.data === 'reload') {
			console.log('🔄 Reloading page...');
			location.reload();
		}
	});
	
	source.addEventListener('error', function(e) {
		if (e.readyState === EventSource.CLOSED) {
			console.log('🔥 Hot reload disconnected, retrying...');
		}
	});
})();
`
	fmt.Fprintf(w, "%s", script)
}

func (s *LiveReloadServer) TriggerReload() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	log.Printf("🔄 Triggering browser reload (%d clients)", len(s.clients))
	
	for client := range s.clients {
		select {
		case client <- struct{}{}:
		default:
		}
	}
	
	// Clear clients after reload
	s.clients = make(map[chan struct{}]bool)
}

func (s *LiveReloadServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}
