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
	"time"
)

func TestNewLogger(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer logger.Close()

	// 检查日志文件是否创建
	expectedFile := filepath.Join(tempDir, "llm_proxy_"+time.Now().Format("2006-01-02")+".log")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected log file %s to be created", expectedFile)
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
	logFile := filepath.Join(tempDir, "llm_proxy_"+time.Now().Format("2006-01-02")+".log")
	content, _ := os.ReadFile(logFile)

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
	logFile := filepath.Join(tempDir, "llm_proxy_"+time.Now().Format("2006-01-02")+".log")
	content, _ := os.ReadFile(logFile)

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
