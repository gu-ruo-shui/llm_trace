package proxy

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"
)

// NewHTTPClient creates a new HTTP client with proxy support for regular requests
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

// NewStreamingHTTPClient creates a new HTTP client with proxy support for streaming requests
func NewStreamingHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:              http.ProxyFromEnvironment,
			DisableCompression: true,
		},
		Timeout: 0, // No timeout for streaming
	}
}

type ProxyHandler struct {
	targetURL string
	logger    *Logger
	client    *http.Client
}

func NewProxyHandler(targetURL string, logger *Logger) *ProxyHandler {
	return &ProxyHandler{
		targetURL: targetURL,
		logger:    logger,
		client:    NewHTTPClient(30 * time.Second),
	}
}

func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Read request body
	reqBody, err := ReadRequestBody(r)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Log the incoming request
	logEntry := p.logger.LogRequest(r, reqBody)

	// Create proxy request
	proxyURL := p.targetURL + r.URL.Path
	if r.URL.RawQuery != "" {
		proxyURL += "?" + r.URL.RawQuery
	}

	proxyReq, err := http.NewRequest(r.Method, proxyURL, bytes.NewReader(reqBody))
	if err != nil {
		p.logger.LogError(logEntry, err)
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// Make the request with custom transport for SSE
	client := NewStreamingHTTPClient()

	resp, err := client.Do(proxyReq)
	if err != nil {
		p.logger.LogError(logEntry, err)
		http.Error(w, "Failed to proxy request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Check if this is a streaming response
	isStream := strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") ||
		strings.Contains(resp.Header.Get("Content-Type"), "application/stream+json")

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	if isStream {
		// Handle streaming response
		p.handleStreamingResponse(w, resp, logEntry)
	} else {
		// Handle regular response
		p.handleRegularResponse(w, resp, logEntry)
	}
}

func (p *ProxyHandler) handleStreamingResponse(w http.ResponseWriter, resp *http.Response, logEntry *RequestLog) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	reader := bufio.NewReader(resp.Body)
	var streamBuffer bytes.Buffer

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			// Write to response first so the client receives chunks as they arrive.
			if _, writeErr := w.Write(line); writeErr != nil {
				p.logger.LogError(logEntry, writeErr)
				break
			}
			flusher.Flush()

			// Accumulate the stream and write a single final log entry.  Logging every
			// data line plus the final body duplicates stream content in file logs.
			streamBuffer.Write(line)
		}

		if err != nil {
			if err != io.EOF {
				p.logger.LogError(logEntry, err)
			}
			break
		}
	}

	// Log final accumulated stream once.
	logEntry.IsStream = true
	p.logger.LogResponse(logEntry, resp, streamBuffer.Bytes(), true)
}

func (p *ProxyHandler) handleRegularResponse(w http.ResponseWriter, resp *http.Response, logEntry *RequestLog) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		p.logger.LogError(logEntry, err)
		return
	}

	// Write response
	if _, err := w.Write(body); err != nil {
		p.logger.LogError(logEntry, err)
		return
	}

	// Log response
	p.logger.LogResponse(logEntry, resp, body, false)
}
