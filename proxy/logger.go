package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type RequestLog struct {
	Timestamp    string              `json:"timestamp"`
	Method       string              `json:"method"`
	URL          string              `json:"url"`
	Headers      map[string][]string `json:"headers"`
	Body         string              `json:"body,omitempty"`
	ResponseCode int                 `json:"response_code,omitempty"`
	Response     string              `json:"response,omitempty"`
	IsStream     bool                `json:"is_stream"`
	Error        string              `json:"error,omitempty"`
}

type Logger struct {
	mu      sync.Mutex
	logFile *os.File
	logDir  string
}

func NewLogger(logDir string) (*Logger, error) {
	// Check if directory exists first
	if stat, err := os.Stat(logDir); err != nil {
		if os.IsNotExist(err) {
			// Try to create the directory
			if err := os.MkdirAll(logDir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create log directory: %w", err)
			}
		} else {
			// Some other error (e.g., permission denied)
			return nil, fmt.Errorf("failed to access log directory: %w", err)
		}
	} else if !stat.IsDir() {
		return nil, fmt.Errorf("log path exists but is not a directory")
	}

	fileName := fmt.Sprintf("llm_proxy_%s.log", time.Now().Format("2006-01-02"))
	filePath := filepath.Join(logDir, fileName)

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &Logger{
		logFile: file,
		logDir:  logDir,
	}, nil
}

func (l *Logger) LogRequest(req *http.Request, body []byte) *RequestLog {
	log := &RequestLog{
		Timestamp: time.Now().Format(time.RFC3339),
		Method:    req.Method,
		URL:       req.URL.String(),
		Headers:   req.Header,
		Body:      string(body),
	}
	return log
}

func (l *Logger) LogResponse(log *RequestLog, resp *http.Response, body []byte, isStream bool) {
	if resp != nil {
		log.ResponseCode = resp.StatusCode
	}
	log.Response = string(body)
	log.IsStream = isStream
	l.writeLog(log)
}

func (l *Logger) LogError(log *RequestLog, err error) {
	if err != nil {
		log.Error = err.Error()
	}
	l.writeLog(log)
}

func (l *Logger) LogStreamChunk(log *RequestLog, chunk string) {
	streamLog := &RequestLog{
		Timestamp: time.Now().Format(time.RFC3339),
		Method:    log.Method,
		URL:       log.URL,
		IsStream:  true,
		Response:  chunk,
	}
	l.writeLog(streamLog)
}

func (l *Logger) writeLog(log *RequestLog) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.Marshal(log)
	if err != nil {
		fmt.Printf("Failed to marshal log: %v\n", err)
		return
	}

	if _, err := l.logFile.Write(append(data, '\n')); err != nil {
		fmt.Printf("Failed to write log: %v\n", err)
	}
}

func (l *Logger) Close() error {
	return l.logFile.Close()
}

func ReadRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(body))
	
	// Return nil for empty body instead of empty slice
	if len(body) == 0 {
		return nil, nil
	}
	return body, nil
}
