package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer logger.Close()

	// 检查日志文件是否创建，且文件名包含启动时间戳而不是按天复用
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Expected exactly 1 log file, got %d", len(entries))
	}
	if !strings.HasPrefix(entries[0].Name(), "llm_proxy_") || !strings.HasSuffix(entries[0].Name(), ".log") {
		t.Errorf("Unexpected log file name: %s", entries[0].Name())
	}
}

func TestNewLogger_InvalidDir(t *testing.T) {
	// Use a path that should fail on both Windows and Unix
	// Try to use an invalid character in the path or a protected location
	invalidDir := string([]byte{0}) + "invalid" // Null character in path

	_, err := NewLogger(invalidDir)
	if err == nil {
		t.Error("Expected error for invalid directory, got nil")
	}
}

func TestLogger_LogRequest(t *testing.T) {
	tempDir := t.TempDir()
	logger, _ := NewLogger(tempDir)
	defer logger.Close()

	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"test":"data"}`))
	req.Header.Set("Content-Type", "application/json")

	body := []byte(`{"test":"data"}`)
	log := logger.LogRequest(req, body)

	if log.Method != "POST" {
		t.Errorf("Expected method POST, got %s", log.Method)
	}
	if log.URL != "/test" {
		t.Errorf("Expected URL /test, got %s", log.URL)
	}
	if log.Body != `{"test":"data"}` {
		t.Errorf("Expected body to be logged correctly")
	}
	if log.Headers["Content-Type"][0] != "application/json" {
		t.Errorf("Expected Content-Type header to be logged")
	}
}

func TestLogger_LogResponse(t *testing.T) {
	tempDir := t.TempDir()
	logger, _ := NewLogger(tempDir)
	defer logger.Close()

	log := &RequestLog{
		Method: "POST",
		URL:    "/test",
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
	}

	body := []byte(`{"response":"data"}`)
	logger.LogResponse(log, resp, body, false)

	if log.ResponseCode != 200 {
		t.Errorf("Expected response code 200, got %d", log.ResponseCode)
	}
	if log.Response != `{"response":"data"}` {
		t.Errorf("Expected response body to be logged correctly")
	}
	if log.IsStream != false {
		t.Errorf("Expected IsStream to be false")
	}
}

func TestLogger_LogError(t *testing.T) {
	tempDir := t.TempDir()
	logger, _ := NewLogger(tempDir)
	defer logger.Close()

	log := &RequestLog{
		Method: "POST",
		URL:    "/test",
	}

	testErr := fmt.Errorf("test error: invalid JSON at offset 123")
	logger.LogError(log, testErr)

	if log.Error == "" {
		t.Errorf("Expected error to be logged, but got empty string. Error was: %v", testErr)
	}
}

func TestLogger_LogStreamChunk(t *testing.T) {
	tempDir := t.TempDir()
	logger, _ := NewLogger(tempDir)
	defer logger.Close()

	log := &RequestLog{
		Method: "POST",
		URL:    "/test",
	}

	logger.LogStreamChunk(log, "data: test chunk\n")

	// 验证日志文件中有内容
	content, _ := os.ReadFile(logger.logFile.Name())

	var loggedLine RequestLog
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			err := json.Unmarshal([]byte(line), &loggedLine)
			if err != nil {
				t.Logf("Failed to parse log line: %v", err)
				continue
			}
			if loggedLine.Response == "data: test chunk\n" {
				return // Success
			}
		}
	}

	t.Error("Expected stream chunk to be logged")
}

func TestLogger_ConcurrentWrites(t *testing.T) {
	tempDir := t.TempDir()
	logger, _ := NewLogger(tempDir)
	defer logger.Close()

	// 并发写入测试
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			log := &RequestLog{Method: "GET", URL: "/concurrent"}
			logger.writeLog(log)
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证所有内容都被写入
	content, _ := os.ReadFile(logger.logFile.Name())

	lines := strings.Split(string(content), "\n")
	count := 0
	for _, line := range lines {
		if strings.Contains(line, "concurrent") {
			count++
		}
	}

	if count != 10 {
		t.Errorf("Expected 10 concurrent logs, got %d", count)
	}
}

func TestReadRequestBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"test":"body"}`))

	body, err := ReadRequestBody(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if string(body) != `{"test":"body"}` {
		t.Errorf("Expected body to be read correctly, got %s", string(body))
	}

	// 验证Body可以被重新读取
	reReadBody, _ := io.ReadAll(req.Body)
	if string(reReadBody) != `{"test":"body"}` {
		t.Error("Expected request body to be reset properly")
	}
}

func TestReadRequestBody_EmptyBody(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	body, err := ReadRequestBody(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if body != nil {
		t.Errorf("Expected nil for empty body, got %v", body)
	}
}

func TestReadRequestBody_EmptyString(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", strings.NewReader(""))

	body, err := ReadRequestBody(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if body != nil {
		t.Errorf("Expected nil for empty string body, got %v", body)
	}
}

func TestNewLogger_CreateDirectoryError(t *testing.T) {
	tempDir := t.TempDir()

	// Create a file with the same name as the intended directory
	filePath := filepath.Join(tempDir, "not_a_dir")
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	file.Close()

	// Try to create logger with file path instead of directory
	_, err = NewLogger(filePath)
	if err == nil {
		t.Error("Expected error when log path is not a directory")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("Expected 'not a directory' error, got: %v", err)
	}
}

func TestNewLogger_NewStartCreatesNewFile(t *testing.T) {
	tempDir := t.TempDir()

	first, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create first logger: %v", err)
	}
	first.writeLog(&RequestLog{Method: "GET", URL: "/first"})
	firstPath := first.logFile.Name()
	first.Close()

	second, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create second logger: %v", err)
	}
	defer second.Close()
	second.writeLog(&RequestLog{Method: "GET", URL: "/second"})
	secondPath := second.logFile.Name()

	if firstPath == secondPath {
		t.Fatalf("Expected a new log file for a new logger start, got same file %s", firstPath)
	}

	firstContent, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("Failed to read first log file: %v", err)
	}
	if strings.Contains(string(firstContent), "/second") {
		t.Error("Expected second start not to append to first log file")
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read log dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("Expected 2 log files, got %d", len(entries))
	}
}

func TestLogger_writeLog_MarshalError(t *testing.T) {
	tempDir := t.TempDir()
	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create a log with a channel that can't be marshaled to JSON
	log := &RequestLog{
		Method: "GET",
		URL:    "/test",
		Headers: map[string][]string{
			"test": {"value"},
		},
	}

	// Add a value that can't be marshaled
	type unmarshalable struct {
		Ch chan int
	}

	// Since we can't directly cause a marshal error with RequestLog,
	// we'll have to test the write error path instead

	// Close the file to cause write error
	logger.logFile.Close()

	// This should handle the error gracefully
	logger.writeLog(log)
}

func TestLogger_LogError_NilError(t *testing.T) {
	tempDir := t.TempDir()
	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	log := &RequestLog{
		Method: "POST",
		URL:    "/test",
	}

	// Log with nil error
	logger.LogError(log, nil)

	if log.Error != "" {
		t.Errorf("Expected empty error for nil error, got %s", log.Error)
	}
}

func TestLogger_LogResponse_NilResponse(t *testing.T) {
	tempDir := t.TempDir()
	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	log := &RequestLog{
		Method: "POST",
		URL:    "/test",
	}

	// Log with nil response
	logger.LogResponse(log, nil, []byte("body"), false)

	if log.ResponseCode != 0 {
		t.Errorf("Expected response code 0 for nil response, got %d", log.ResponseCode)
	}
}

func TestReadRequestBody_ReadError(t *testing.T) {
	// Create a custom reader that returns an error
	errorReader := &failingReader{err: fmt.Errorf("read error")}
	req := httptest.NewRequest("POST", "/test", errorReader)

	_, err := ReadRequestBody(req)
	if err == nil {
		t.Error("Expected error from failing reader")
	}
	if err.Error() != "read error" {
		t.Errorf("Expected 'read error', got %v", err)
	}
}

// failingReader implements io.Reader but always returns an error
type failingReader struct {
	err error
}

func (r *failingReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}
