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
		p.logErrorWithDuration(logEntry, err, startTime)
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
		p.logErrorWithDuration(logEntry, err, startTime)
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
		p.handleStreamingResponse(w, resp, logEntry, startTime)
	} else {
		// Handle regular response
		p.handleRegularResponse(w, resp, logEntry, startTime)
	}
}

func (p *ProxyHandlerDB) logErrorWithDuration(logEntry *DatabaseRequestLog, err error, startTime time.Time) {
	if logEntry == nil {
		return
	}
	logEntry.Duration = time.Since(startTime).Milliseconds()
	p.logger.LogError(logEntry, err)
}

func (p *ProxyHandlerDB) handleStreamingResponse(w http.ResponseWriter, resp *http.Response, logEntry *DatabaseRequestLog, startTime time.Time) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		p.logErrorWithDuration(logEntry, http.ErrNotSupported, startTime)
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	reader := bufio.NewReader(resp.Body)
	var streamBuffer bytes.Buffer
	parser := &sseEventParser{}
	sequence := 0

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			// Write to response first so streaming clients receive chunks immediately.
			if _, writeErr := w.Write(line); writeErr != nil {
				p.logErrorWithDuration(logEntry, writeErr, startTime)
				break
			}
			flusher.Flush()

			// Accumulate the exact upstream stream for the final request log.
			streamBuffer.Write(line)

			// Parse SSE incrementally without assuming an `event:` line. Data-only
			// streams are valid SSE and are recorded as `message` events.
			if event, ok := parser.ProcessLine(string(line)); ok {
				p.logger.LogSSEEvent(logEntry.RequestUUID, event.EventType, event.Data, sequence)
				sequence++
			}
		}

		if err != nil {
			if err != io.EOF {
				p.logErrorWithDuration(logEntry, err, startTime)
			}
			break
		}
	}

	// Dispatch a final event if the upstream ended without the SSE blank-line
	// delimiter. This protects data-only streams and truncated-but-readable data.
	if event, ok := parser.Flush(); ok {
		p.logger.LogSSEEvent(logEntry.RequestUUID, event.EventType, event.Data, sequence)
	}

	// Preserve the raw stream in request_logs. If the Anthropic-style event
	// processor produced useful derived text/tool data, keep the prior processed
	// JSON behavior; otherwise do not replace the raw stream with an empty object.
	logResponse := streamBuffer.String()
	if processedSSE, err := p.logger.ProcessSSEEvents(logEntry.RequestUUID); err == nil &&
		(processedSSE.ToolName != "" || processedSSE.ProcessedText != "") {
		if processedJSON, marshalErr := json.Marshal(processedSSE); marshalErr == nil {
			logResponse = string(processedJSON)
		}
	}
	logEntry.Response = logResponse

	// Log final response with full streaming duration, including body read/write.
	p.logger.LogResponse(logEntry, resp, []byte(logResponse), true, time.Since(startTime))
}

func (p *ProxyHandlerDB) handleRegularResponse(w http.ResponseWriter, resp *http.Response, logEntry *DatabaseRequestLog, startTime time.Time) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		p.logErrorWithDuration(logEntry, err, startTime)
		return
	}

	// Write response
	if _, err := w.Write(body); err != nil {
		p.logErrorWithDuration(logEntry, err, startTime)
		return
	}

	// Log response with duration after the response body has been read and written.
	p.logger.LogResponse(logEntry, resp, body, false, time.Since(startTime))
}
