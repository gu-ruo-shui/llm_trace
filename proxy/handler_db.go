package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
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
		client:    NewHTTPClient(30 * time.Second),
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
	client := NewStreamingHTTPClient()

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
	sequence := 0

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

		// Parse and log SSE events
		lineStr := string(line)
		if strings.HasPrefix(lineStr, "event: ") {
			eventType := strings.TrimSpace(strings.TrimPrefix(lineStr, "event: "))
			// Look for the next data line
			dataLine, err := reader.ReadBytes('\n')
			if err == nil && strings.HasPrefix(string(dataLine), "data: ") {
				// Write the data line to response too
				if _, writeErr := w.Write(dataLine); writeErr != nil {
					p.logger.LogError(logEntry, writeErr)
					break
				}
				flusher.Flush()
				streamBuffer.Write(dataLine)

				dataContent := strings.TrimSpace(strings.TrimPrefix(string(dataLine), "data: "))
				p.logger.LogSSEEvent(logEntry.RequestUUID, eventType, dataContent, sequence)
				sequence++
			}
		}
	}

	// Process SSE events and store as JSON response
	processedSSE, err := p.logger.ProcessSSEEvents(logEntry.RequestUUID)
	if err == nil {
		processedJSON, _ := json.Marshal(processedSSE)
		logEntry.Response = string(processedJSON)
	} else {
		logEntry.Response = streamBuffer.String()
	}

	// Log final response
	finalDuration := time.Since(time.Now().Add(-startDuration))
	p.logger.LogResponse(logEntry, resp, []byte(logEntry.Response), true, finalDuration)
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
