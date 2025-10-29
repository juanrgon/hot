package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type LiveReloadServer struct {
	port       int
	clients    map[chan liveMessage]bool
	mu         sync.Mutex
	server     *http.Server
	lastStatus RunnerStatus
}

type liveMessage struct {
	Event string
	Data  string
}

func NewLiveReloadServer(port int) *LiveReloadServer {
	return &LiveReloadServer{
		port:    port,
		clients: make(map[chan liveMessage]bool),
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

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	clientCh := make(chan liveMessage, 16)

	s.mu.Lock()
	s.clients[clientCh] = true
	status := s.lastStatus
	s.mu.Unlock()

	defer s.removeClient(clientCh)

	// Send last known status (or connected state) to the new client.
	if status.State == "" {
		status = RunnerStatus{
			State:     StateIdle,
			Message:   "connected",
			Timestamp: time.Now().UTC(),
		}
	}
	if data, err := json.Marshal(status); err == nil {
		clientCh <- liveMessage{Event: "state", Data: string(data)}
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-clientCh:
			if !ok {
				return
			}
			if msg.Event != "" {
				fmt.Fprintf(w, "event: %s\n", msg.Event)
			}
			fmt.Fprintf(w, "data: %s\n\n", msg.Data)
			flusher.Flush()
		}
	}
}

func (s *LiveReloadServer) handleScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	script := `
(function() {
	var source = new EventSource('http://localhost:` + fmt.Sprintf("%d", s.port) + `/livereload');
	var overlay;
	var overlayContent;
	var pendingErrorMessage = null;
	var statusBanner;
	var statusText;
	var pendingStatusMessage = null;
	var statusIcon;

	function createOverlay() {
		if (overlay) {
			return;
		}

		if (!document.body) {
			window.addEventListener('DOMContentLoaded', createOverlay, { once: true });
			return;
		}

		overlay = document.createElement('div');
		overlay.id = 'hot-reload-overlay';
		overlay.style.position = 'fixed';
		overlay.style.top = '0';
		overlay.style.left = '0';
		overlay.style.right = '0';
		overlay.style.bottom = '0';
		overlay.style.background = 'rgba(0, 0, 0, 0.85)';
		overlay.style.color = '#fff';
		overlay.style.padding = '24px';
		overlay.style.overflow = 'auto';
		overlay.style.zIndex = '2147483647';
		overlay.style.display = 'none';
		overlay.style.fontFamily = 'Menlo, Monaco, monospace';
		overlay.style.whiteSpace = 'pre-wrap';

		var title = document.createElement('div');
		title.textContent = '❌ Build failed';
		title.style.fontSize = '20px';
		title.style.marginBottom = '12px';
		overlay.appendChild(title);

		overlayContent = document.createElement('pre');
		overlayContent.style.margin = '0';
		overlayContent.style.fontSize = '14px';
		overlayContent.style.lineHeight = '1.5';
		overlay.appendChild(overlayContent);

		document.body.appendChild(overlay);

		if (pendingErrorMessage) {
			var message = pendingErrorMessage;
			pendingErrorMessage = null;
			showOverlay(message);
		}
	}

	function createStatusBanner() {
		if (statusBanner) {
			return;
		}

		if (!document.body) {
			window.addEventListener('DOMContentLoaded', createStatusBanner, { once: true });
			return;
		}

		statusBanner = document.createElement('div');
		statusBanner.id = 'hot-reload-status';
		statusBanner.style.position = 'fixed';
		statusBanner.style.bottom = '20px';
		statusBanner.style.right = '20px';
		statusBanner.style.padding = '10px 14px';
		statusBanner.style.borderRadius = '999px';
		statusBanner.style.background = 'rgba(31, 41, 51, 0.92)';
		statusBanner.style.color = '#fff';
		statusBanner.style.display = 'none';
		statusBanner.style.fontSize = '13px';
		statusBanner.style.alignItems = 'center';
		statusBanner.style.gap = '6px';
		statusBanner.style.boxShadow = '0 10px 30px rgba(15, 23, 42, 0.35)';
		statusBanner.style.zIndex = '2147483647';
		statusBanner.style.fontFamily = 'system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';

		statusIcon = document.createElement('span');
		statusIcon.textContent = '⏳';
		statusIcon.style.fontSize = '14px';
		statusBanner.appendChild(statusIcon);

		statusText = document.createElement('span');
		statusText.style.display = 'inline-block';
		statusText.style.verticalAlign = 'middle';
		statusBanner.appendChild(statusText);

		document.body.appendChild(statusBanner);

		if (pendingStatusMessage) {
			showStatus(pendingStatusMessage.message, pendingStatusMessage.tone);
			pendingStatusMessage = null;
		}
	}

	function showOverlay(message) {
		if (!overlay || !overlayContent) {
			pendingErrorMessage = message || 'Build failed';
			createOverlay();
			return;
		}
		pendingErrorMessage = null;
		overlayContent.textContent = message || 'Build failed';
		overlay.style.display = 'block';
	}

	function hideOverlay() {
		if (!overlay) {
			return;
		}
		pendingErrorMessage = null;
		overlay.style.display = 'none';
	}

	function showStatus(message, tone) {
		if (!statusBanner || !statusText) {
			pendingStatusMessage = { message: message, tone: tone };
			createStatusBanner();
			return;
		}

		if (tone === 'success') {
			statusBanner.style.background = 'rgba(22, 163, 74, 0.92)';
			statusIcon.textContent = '✅';
		} else if (tone === 'warning') {
			statusBanner.style.background = 'rgba(234, 179, 8, 0.92)';
			statusIcon.textContent = '⏳';
		} else if (tone === 'danger') {
			statusBanner.style.background = 'rgba(239, 68, 68, 0.92)';
			statusIcon.textContent = '⚠️';
		} else {
			statusBanner.style.background = 'rgba(31, 41, 51, 0.92)';
			statusIcon.textContent = 'ℹ️';
		}

		statusText.textContent = message || 'Working…';
		statusBanner.style.display = 'flex';
	}

	function hideStatus() {
		pendingStatusMessage = null;
		if (!statusBanner) {
			return;
		}
		statusBanner.style.display = 'none';
	}

	source.addEventListener('open', function() {
		console.log('🔥 Hot reload connected');
	});

	source.addEventListener('reload', function(e) {
		console.log('🔄 Reloading page...');
		hideOverlay();
		showStatus('Refreshing latest build…', 'success');
		location.reload();
	});

	source.addEventListener('state', function(e) {
		try {
			var payload = JSON.parse(e.data || '{}');
			if (payload.state === 'build_failed') {
				var message = payload.error || payload.message || 'Build failed';
				showOverlay(message);
				hideStatus();
			} else if (payload.state === 'starting') {
				hideOverlay();
				showStatus(payload.message || 'Starting application…', 'warning');
			} else if (payload.state === 'building') {
				hideOverlay();
				showStatus(payload.message || 'Building latest changes…', 'warning');
			} else if (payload.state === 'stopping') {
				hideOverlay();
				showStatus(payload.message || 'Stopping previous process…', 'danger');
			} else if (payload.state === 'running') {
				hideOverlay();
				showStatus(payload.message || 'Running fresh build ✅', 'success');
				setTimeout(hideStatus, 1500);
			} else if (payload.state === 'idle') {
				hideOverlay();
				hideStatus();
			} else {
				hideOverlay();
				hideStatus();
			}
		} catch (err) {
			console.warn('⚠️ Failed to parse hot reload state', err);
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
	log.Printf("🔄 Triggering browser reload (%d clients)", len(s.clients))
	payload, err := json.Marshal(map[string]string{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		log.Printf("Error marshaling reload payload: %v", err)
		payload = []byte("{}")
	}
	for client := range s.clients {
		select {
		case client <- liveMessage{Event: "reload", Data: string(payload)}:
		default:
		}
	}
	s.mu.Unlock()
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

func (s *LiveReloadServer) EmitState(status RunnerStatus) {
	if status.Timestamp.IsZero() {
		status.Timestamp = time.Now().UTC()
	}

	payload, err := json.Marshal(status)
	if err != nil {
		log.Printf("Error marshaling state: %v", err)
		return
	}

	s.mu.Lock()
	s.lastStatus = status
	for client := range s.clients {
		select {
		case client <- liveMessage{Event: "state", Data: string(payload)}:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *LiveReloadServer) removeClient(ch chan liveMessage) {
	s.mu.Lock()
	delete(s.clients, ch)
	close(ch)
	s.mu.Unlock()
}
