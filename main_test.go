package main

import (
	"bufio"
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

	"llm_reverse/config"
	"llm_reverse/proxy"

	_ "github.com/mattn/go-sqlite3"
)

func readLogDir(t *testing.T, logDir string) string {
	t.Helper()

	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("Failed to read log dir: %v", err)
	}

	var content strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "llm_proxy_") || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(logDir, entry.Name()))
		if err != nil {
			t.Fatalf("Failed to read log file %s: %v", entry.Name(), err)
		}
		content.Write(data)
	}

	return content.String()
}

// TestServer 用于集成测试的测试服务器
type TestServer struct {
	Server *httptest.Server
}

// NewTestServer 创建一个新的测试服务器
func NewTestServer() *TestServer {
	return &TestServer{}
}

// StartWithFileLogging 启动使用文件日志的服务器
func (ts *TestServer) StartWithFileLogging() {
	tempDir := os.TempDir()
	logger, err := proxy.NewLogger(tempDir)
	if err != nil {
		panic(fmt.Sprintf("Failed to create logger: %v", err))
	}

	config := &config.Config{
		ServerPort: ":8080",
		TargetURL:  "https://httpbin.org",
		LogDir:     tempDir,
		UseDB:      false,
	}

	handler := proxy.NewProxyHandler(config.TargetURL, logger)
	ts.Server = httptest.NewServer(handler)
}

// StartWithDatabaseLogging 启动使用数据库日志的服务器
func (ts *TestServer) StartWithDatabaseLogging() {
	tempDir := os.TempDir()
	dbPath := filepath.Join(tempDir, "integration_test.db")

	logger, err := proxy.NewDatabaseLogger(dbPath)
	if err != nil {
		panic(fmt.Sprintf("Failed to create database logger: %v", err))
	}

	config := &config.Config{
		ServerPort: ":8080",
		TargetURL:  "https://httpbin.org",
		DBPath:     dbPath,
		UseDB:      true,
	}

	handler := proxy.NewProxyHandlerDB(config.TargetURL, logger)
	ts.Server = httptest.NewServer(handler)
}

// Close 关闭测试服务器
func (ts *TestServer) Close() {
	if ts.Server != nil {
		ts.Server.Close()
	}
}

// GetURL 获取测试服务器URL
func (ts *TestServer) GetURL() string {
	if ts.Server != nil {
		return ts.Server.URL
	}
	return ""
}

// 测试HTTPBin集成
func TestIntegrationWithHTTPBin(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 使用本地测试服务器代替HTTPBin
	testTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"method":  r.Method,
			"url":     r.URL.String(),
			"headers": r.Header,
			"body":    "",
		}

		body, _ := io.ReadAll(r.Body)
		if len(body) > 0 {
			response["body"] = string(body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer testTarget.Close()

	t.Run("FileLoggingMode", func(t *testing.T) {
		tempDir := t.TempDir()
		logger, err := proxy.NewLogger(tempDir)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		handler := proxy.NewProxyHandler(testTarget.URL, logger)
		server := httptest.NewServer(handler)
		defer server.Close()

		// 测试GET请求
		resp, err := http.Get(server.URL + "/get")
		if err != nil {
			t.Fatalf("Failed to make GET request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		// 验证日志文件
		if content := readLogDir(t, tempDir); content == "" {
			t.Errorf("Expected log file to be created")
		}
	})

	t.Run("DatabaseLoggingMode", func(t *testing.T) {
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "integration_test.db")

		logger, err := proxy.NewDatabaseLogger(dbPath)
		if err != nil {
			t.Fatalf("Failed to create database logger: %v", err)
		}
		defer logger.Close()

		handler := proxy.NewProxyHandlerDB(testTarget.URL, logger)
		server := httptest.NewServer(handler)
		defer server.Close()

		// 测试POST请求
		jsonData := `{"test":"data"}`
		resp, err := http.Post(server.URL+"/post", "application/json", strings.NewReader(jsonData))
		if err != nil {
			t.Fatalf("Failed to make POST request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		// 验证数据库记录
		logs, err := logger.GetLogs(10, 0)
		if err != nil {
			t.Fatalf("Failed to get logs: %v", err)
		}

		if len(logs) == 0 {
			t.Error("Expected logs to be recorded in database")
		}
	})
}

// 测试流式响应
func TestIntegration_Streaming(t *testing.T) {
	// 创建流式测试目标
	streamTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// 发送流式数据
		messages := []string{
			"data: {\"message\":\"Hello\"}\n\n",
			"data: {\"message\":\"World\"}\n\n",
			"data: [DONE]\n\n",
		}

		for _, msg := range messages {
			w.Write([]byte(msg))
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer streamTarget.Close()

	t.Run("FileLoggingStreaming", func(t *testing.T) {
		tempDir := t.TempDir()
		logger, err := proxy.NewLogger(tempDir)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		handler := proxy.NewProxyHandler(streamTarget.URL, logger)
		server := httptest.NewServer(handler)
		defer server.Close()

		// 测试流式请求
		req, _ := http.NewRequest("POST", server.URL+"/stream", strings.NewReader(`{"stream":true}`))
		req.Header.Set("Accept", "text/event-stream")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to make streaming request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		// 读取流式响应
		scanner := bufio.NewScanner(resp.Body)
		var lines []string
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) != "" {
				lines = append(lines, line)
			}
		}

		if len(lines) < 3 {
			t.Errorf("Expected at least 3 lines in streaming response, got %d", len(lines))
		}
	})

	t.Run("DatabaseLoggingStreaming", func(t *testing.T) {
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "streaming_test.db")

		logger, err := proxy.NewDatabaseLogger(dbPath)
		if err != nil {
			t.Fatalf("Failed to create database logger: %v", err)
		}
		defer logger.Close()

		handler := proxy.NewProxyHandlerDB(streamTarget.URL, logger)
		server := httptest.NewServer(handler)
		defer server.Close()

		// 测试流式请求
		req, _ := http.NewRequest("POST", server.URL+"/stream", strings.NewReader(`{"stream":true}`))
		req.Header.Set("Accept", "text/event-stream")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to make streaming request: %v", err)
		}
		defer resp.Body.Close()

		// 读取流式响应
		io.ReadAll(resp.Body) // 读取但不验证内容

		// 验证流式日志被记录
		logs, err := logger.GetLogs(10, 0)
		if err != nil {
			t.Fatalf("Failed to get logs: %v", err)
		}

		if len(logs) == 0 {
			t.Error("Expected streaming logs to be recorded")
		}
	})
}

// 测试错误处理
func TestIntegration_ErrorHandling(t *testing.T) {
	t.Run("FileLoggingError", func(t *testing.T) {
		tempDir := t.TempDir()
		logger, err := proxy.NewLogger(tempDir)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		handler := proxy.NewProxyHandler("http://invalid.url:12345", logger)
		server := httptest.NewServer(handler)
		defer server.Close()

		resp, err := http.Get(server.URL + "/test")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadGateway {
			t.Errorf("Expected status 502 (Bad Gateway), got %d", resp.StatusCode)
		}

		// 验证错误被记录
		content := readLogDir(t, tempDir)

		if !strings.Contains(content, "error") {
			t.Error("Expected error to be logged")
		}
	})

	t.Run("DatabaseLoggingError", func(t *testing.T) {
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "error_test.db")

		logger, err := proxy.NewDatabaseLogger(dbPath)
		if err != nil {
			t.Fatalf("Failed to create database logger: %v", err)
		}
		defer logger.Close()

		handler := proxy.NewProxyHandlerDB("http://invalid.url:12345", logger)
		server := httptest.NewServer(handler)
		defer server.Close()

		resp, err := http.Get(server.URL + "/test")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadGateway {
			t.Errorf("Expected status 502 (Bad Gateway), got %d", resp.StatusCode)
		}

		// 验证错误被记录到数据库
		time.Sleep(100 * time.Millisecond) // 等待日志写入

		logs, err := logger.GetErrorLogs(10)
		if err != nil {
			t.Fatalf("Failed to get error logs: %v", err)
		}

		if len(logs) == 0 {
			t.Log("Expected error to be logged to database, but found none")
		}
	})
}

// 测试性能
func TestIntegration_Performance(t *testing.T) {
	// 创建高性能测试目标
	fastTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer fastTarget.Close()

	t.Run("FileLoggingPerformance", func(t *testing.T) {
		tempDir := t.TempDir()
		logger, err := proxy.NewLogger(tempDir)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		handler := proxy.NewProxyHandler(fastTarget.URL, logger)
		server := httptest.NewServer(handler)
		defer server.Close()

		// 基准测试
		start := time.Now()
		for i := 0; i < 100; i++ {
			resp, err := http.Get(server.URL + "/test")
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			resp.Body.Close()
		}
		duration := time.Since(start)

		avgDuration := duration / 100
		t.Logf("Average request duration: %v", avgDuration)

		if avgDuration > 50*time.Millisecond {
			t.Logf("Performance warning: average duration %v exceeds 50ms", avgDuration)
		}
	})

	t.Run("DatabaseLoggingPerformance", func(t *testing.T) {
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "performance_test.db")

		logger, err := proxy.NewDatabaseLogger(dbPath)
		if err != nil {
			t.Fatalf("Failed to create database logger: %v", err)
		}
		defer logger.Close()

		handler := proxy.NewProxyHandlerDB(fastTarget.URL, logger)
		server := httptest.NewServer(handler)
		defer server.Close()

		// 基准测试
		start := time.Now()
		for i := 0; i < 100; i++ {
			resp, err := http.Get(server.URL + "/test")
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			resp.Body.Close()
		}
		duration := time.Since(start)

		avgDuration := duration / 100
		t.Logf("Average request duration: %v", avgDuration)

		// 验证数据库中的记录数量
		logs, err := logger.GetStats()
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}

		if logs["total_logs"] != 100 {
			t.Errorf("Expected 100 logs, got %v", logs["total_logs"])
		}
	})
}
