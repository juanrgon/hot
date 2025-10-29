package main

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// InjectorProxy forwards HTTP traffic to an upstream server while injecting the
// live reload script tag into HTML responses.
type InjectorProxy struct {
	target       *url.URL
	server       *http.Server
	snippet      []byte
	shutdown     time.Duration
	launched     bool
	statusFn     func() RunnerStatus
	lastErrorMsg string
	lastErrorAt  time.Time
	logCooldown  time.Duration
}

// NewInjectorProxy creates a proxy listening on listenPort and forwarding to the
// provided target URL. The injected script will load from scriptPort.
func NewInjectorProxy(listenPort int, target string, scriptPort int, statusFn func() RunnerStatus) (*InjectorProxy, error) {
	if listenPort == 0 || target == "" {
		return nil, fmt.Errorf("proxy requires listen port and target")
	}

	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy target: %w", err)
	}

	snippet := []byte("\n<script defer src=\"http://localhost:" + strconv.Itoa(scriptPort) + "/livereload.js\"></script>\n")

	proxy := &InjectorProxy{
		target:      targetURL,
		snippet:     snippet,
		shutdown:    3 * time.Second,
		statusFn:    statusFn,
		logCooldown: 2 * time.Second,
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(targetURL)
	reverseProxy.ModifyResponse = proxy.injectResponse
	reverseProxy.ErrorHandler = proxy.handleProxyError

	proxy.server = &http.Server{
		Addr: fmt.Sprintf(":%d", listenPort),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reverseProxy.ServeHTTP(w, r)
		}),
	}

	return proxy, nil
}

// Start begins serving proxy traffic. It should be invoked in its own goroutine.
func (p *InjectorProxy) Start() error {
	if p.server == nil {
		return fmt.Errorf("proxy server not configured")
	}
	p.launched = true
	log.Printf("🪞 HTML proxy: http://localhost%s -> %s", p.server.Addr, p.target)
	err := p.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Stop gracefully shuts down the proxy server.
func (p *InjectorProxy) Stop() {
	if p.server == nil || !p.launched {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.shutdown)
	defer cancel()
	if err := p.server.Shutdown(ctx); err != nil {
		log.Printf("Error stopping proxy: %v", err)
	}
}

func (p *InjectorProxy) injectResponse(resp *http.Response) error {
	if resp == nil {
		return nil
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "text/html") {
		return nil
	}
	if resp.Header.Get("Content-Encoding") != "" {
		return nil
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if shouldSkipInjection(bodyBytes, p.snippet) {
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return nil
	}

	modified, injected := insertSnippet(bodyBytes, p.snippet)
	if !injected {
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return nil
	}

	resp.Body = io.NopCloser(bytes.NewReader(modified))
	resp.ContentLength = int64(len(modified))
	resp.Header.Set("Content-Length", strconv.Itoa(len(modified)))

	return nil
}

func shouldSkipInjection(body []byte, snippet []byte) bool {
	lowerBody := bytes.ToLower(body)
	lowerSnippet := bytes.ToLower(snippet)
	if bytes.Contains(lowerBody, lowerSnippet) {
		return true
	}
	if bytes.Contains(lowerBody, []byte("livereload.js")) {
		return true
	}
	return false
}

func insertSnippet(body []byte, snippet []byte) ([]byte, bool) {
	lower := bytes.ToLower(body)
	if idx := bytes.Index(lower, []byte("</head>")); idx != -1 {
		var output bytes.Buffer
		output.Grow(len(body) + len(snippet))
		output.Write(body[:idx])
		output.Write(snippet)
		output.Write(body[idx:])
		return output.Bytes(), true
	}
	if idx := bytes.Index(lower, []byte("</body>")); idx != -1 {
		var output bytes.Buffer
		output.Grow(len(body) + len(snippet))
		output.Write(body[:idx])
		output.Write(snippet)
		output.Write(body[idx:])
		return output.Bytes(), true
	}
	return body, false
}

func (p *InjectorProxy) handleProxyError(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		now := time.Now()
		errMsg := err.Error()
		if errMsg != p.lastErrorMsg || now.Sub(p.lastErrorAt) > p.logCooldown {
			log.Printf("Proxy error: %v", err)
			p.lastErrorMsg = errMsg
			p.lastErrorAt = now
		}
	}

	status := p.currentStatus()
	p.renderStatusPage(w, status, err)
}

func (p *InjectorProxy) currentStatus() RunnerStatus {
	if p.statusFn == nil {
		return RunnerStatus{State: StateIdle}
	}
	status := p.statusFn()
	if status.State == "" {
		status.State = StateIdle
	}
	return status
}

func (p *InjectorProxy) renderStatusPage(w http.ResponseWriter, status RunnerStatus, proxyErr error) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)

	icon := stateIcon(status.State)
	message := strings.TrimSpace(status.Message)
	if message == "" {
		message = defaultStateMessage(status.State)
	}
	if message == "" {
		message = "Waiting for application..."
	}

	detail := strings.TrimSpace(status.Error)
	if detail == "" && proxyErr != nil {
		detail = proxyErr.Error()
	}

	detailHTML := ""
	if detail != "" {
		detailHTML = "<pre>" + html.EscapeString(detail) + "</pre>"
	}

	stateLabel := strings.ToUpper(string(status.State))
	if stateLabel == "" {
		stateLabel = "IDLE"
	}

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>hot proxy</title>
  <style>
    body { margin: 0; font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #0f172a; color: #e2e8f0; display: flex; align-items: center; justify-content: center; min-height: 100vh; }
    .card { text-align: center; max-width: 520px; padding: 32px; background: rgba(15, 23, 42, 0.75); border-radius: 16px; box-shadow: 0 30px 60px rgba(2, 12, 27, 0.65); backdrop-filter: blur(12px); }
    h1 { margin: 12px 0 8px 0; font-size: 24px; font-weight: 600; }
    .icon { font-size: 42px; }
    .state { letter-spacing: 0.08em; font-size: 12px; color: #94a3b8; margin-bottom: 12px; }
    pre { text-align: left; white-space: pre-wrap; background: rgba(15, 23, 42, 0.55); border-radius: 12px; padding: 16px; font-size: 13px; line-height: 1.6; color: #f87171; max-height: 260px; overflow: auto; }
    .hint { margin-top: 20px; font-size: 13px; color: #94a3b8; }
  </style>
</head>
<body>
  <div class="card">
    <div class="icon">%s</div>
    <div class="state">%s</div>
    <h1>%s</h1>
    %s
    <p class="hint">This page will refresh automatically when your app is ready.</p>
  </div>
  %s
</body>
</html>`, icon, html.EscapeString(stateLabel), html.EscapeString(message), detailHTML, string(p.snippet))
}

func stateIcon(state RunnerState) string {
	switch state {
	case StateBuilding:
		return "⚙️"
	case StateStarting:
		return "⏳"
	case StateStopping:
		return "🛑"
	case StateRunning:
		return "🚀"
	case StateBuildFailed:
		return "❌"
	default:
		return "⏳"
	}
}

func defaultStateMessage(state RunnerState) string {
	switch state {
	case StateBuilding:
		return "Building latest changes..."
	case StateStarting:
		return "Starting application..."
	case StateStopping:
		return "Stopping previous process..."
	case StateRunning:
		return "Application ready."
	case StateBuildFailed:
		return "Build failed. Check errors below."
	default:
		return "Waiting for application..."
	}
}
