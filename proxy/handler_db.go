package proxy

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"
)

type ProxyHandlerDB struct {
	targetURL string
	logger    *DatabaseLogger
	client    *http.Client
}

func NewProxyHandlerDB(targetURL string, logger *DatabaseLogger) *ProxyHandlerDB {
	return &ProxyHandlerDB{
		targetURL: targetURL,
		logger:    logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
	}
}

func (p *ProxyHandlerDB) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	
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
	client := &http.Client{
		Transport: &http.Transport{
			DisableCompression: true,
		},
		Timeout: 0, // No timeout for streaming
	}

	resp, err := client.Do(proxyReq)
	if err != nil {
		p.logger.LogError(logEntry, err)
		http.Error(w, "Failed to proxy request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Calculate duration
	duration := time.Since(startTime)

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
		p.handleStreamingResponse(w, resp, logEntry, duration)
	} else {
		// Handle regular response
		p.handleRegularResponse(w, resp, logEntry, duration)
	}
}

func (p *ProxyHandlerDB) handleStreamingResponse(w http.ResponseWriter, resp *http.Response, logEntry *DatabaseRequestLog, startDuration time.Duration) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	reader := bufio.NewReader(resp.Body)
	var streamBuffer bytes.Buffer

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				p.logger.LogError(logEntry, err)
			}
			break
		}

		// Write to response
		if _, writeErr := w.Write(line); writeErr != nil {
			p.logger.LogError(logEntry, writeErr)
			break
		}
		flusher.Flush()

		// Accumulate for logging
		streamBuffer.Write(line)

		// Log chunks periodically or on data: lines
		if bytes.HasPrefix(line, []byte("data: ")) {
			duration := time.Since(time.Now().Add(-startDuration))
			p.logger.LogStreamChunk(logEntry, string(line), duration)
		}
	}

	// Log final accumulated stream
	finalDuration := time.Since(time.Now().Add(-startDuration))
	p.logger.LogResponse(logEntry, resp, streamBuffer.Bytes(), true, finalDuration)
}

func (p *ProxyHandlerDB) handleRegularResponse(w http.ResponseWriter, resp *http.Response, logEntry *DatabaseRequestLog, duration time.Duration) {
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
	p.logger.LogResponse(logEntry, resp, body, false, duration)
}