package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// 创建测试用的模拟服务器
type mockServer struct {
	*httptest.Server
	responses map[string]*mockResponse
}

type mockResponse struct {
	statusCode int
	headers    map[string]string
	body       string
	stream     bool
}

func newMockServer() *mockServer {
	ms := &mockServer{
		responses: make(map[string]*mockResponse),
	}

	ms.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response, exists := ms.responses[r.URL.Path]
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		for key, value := range response.headers {
			w.Header().Set(key, value)
		}

		w.WriteHeader(response.statusCode)

		if response.stream {
			// 模拟流式响应
			writer := w.(http.Flusher)
			chunks := strings.Split(response.body, "\n")
			for _, chunk := range chunks {
				if strings.TrimSpace(chunk) != "" {
					w.Write([]byte(chunk + "\n"))
					writer.Flush()
					time.Sleep(10 * time.Millisecond)
				}
			}
		} else {
			w.Write([]byte(response.body))
		}
	}))

	return ms
}

func (ms *mockServer) addResponse(path string, response *mockResponse) {
	ms.responses[path] = response
}

func TestProxyHandler_ServeHTTP_RegularResponse(t *testing.T) {
	// 创建临时日志目录
	tempDir := t.TempDir()
	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// 创建模拟目标服务器
	mock := newMockServer()
	defer mock.Close()

	mock.addResponse("/test", &mockResponse{
		statusCode: http.StatusOK,
		headers:    map[string]string{"Content-Type": "application/json"},
		body:       `{"response":"test"}`,
		stream:     false,
	})

	// 创建代理处理器
	handler := NewProxyHandler(mock.URL, logger)

	// 创建测试请求
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"request":"test"}`))
	req.Header.Set("Content-Type", "application/json")

	// 记录响应
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// 验证响应
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["response"] != "test" {
		t.Errorf("Expected response to contain 'test', got %v", response)
	}

	// 验证日志文件
	content, _ := os.ReadFile(logger.logFile.Name())

	if !strings.Contains(string(content), "test") {
		t.Error("Expected request to be logged")
	}
}

func TestProxyHandler_ServeHTTP_StreamingResponse(t *testing.T) {
	// 创建临时日志目录
	tempDir := t.TempDir()
	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// 创建模拟目标服务器
	mock := newMockServer()
	defer mock.Close()

	mock.addResponse("/stream", &mockResponse{
		statusCode: http.StatusOK,
		headers:    map[string]string{"Content-Type": "text/event-stream"},
		body:       "data: chunk1\ndata: chunk2\ndata: [DONE]",
		stream:     true,
	})

	// 创建代理处理器
	handler := NewProxyHandler(mock.URL, logger)

	// 创建测试请求
	req := httptest.NewRequest("POST", "/stream", strings.NewReader(`{"stream":true}`))

	// 创建支持流式的响应记录器
	rr := httptest.NewRecorder()

	// 确保响应记录器支持Flusher
	var w http.ResponseWriter = rr
	if _, ok := w.(http.Flusher); !ok {
		// 创建一个自定义的ResponseRecorder
		w = &streamingRecorder{ResponseRecorder: rr}
	}

	handler.ServeHTTP(w, req)

	// 验证响应
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	response := rr.Body.String()
	if !strings.Contains(response, "data: chunk1") {
		t.Error("Expected streaming response to contain chunk1")
	}
	if !strings.Contains(response, "data: [DONE]") {
		t.Error("Expected streaming response to contain [DONE]")
	}

	// 验证日志文件包含流式内容
	content, _ := os.ReadFile(logger.logFile.Name())

	if !strings.Contains(string(content), "data: chunk1") {
		t.Error("Expected streaming chunks to be logged")
	}
}

func TestProxyHandler_StreamingLogsFinalResponseOnly(t *testing.T) {
	tempDir := t.TempDir()
	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	mock := newMockServer()
	defer mock.Close()

	mock.addResponse("/stream", &mockResponse{
		statusCode: http.StatusOK,
		headers:    map[string]string{"Content-Type": "text/event-stream"},
		body:       "data: chunk1\n\ndata: chunk2\n\n",
		stream:     true,
	})

	handler := NewProxyHandler(mock.URL, logger)
	req := httptest.NewRequest("POST", "/stream", strings.NewReader(`{"stream":true}`))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(&streamingRecorder{ResponseRecorder: rr}, req)

	contentBytes, err := os.ReadFile(logger.logFile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	content := string(contentBytes)

	logLines := strings.Split(strings.TrimSpace(content), "\n")
	if len(logLines) != 1 {
		t.Fatalf("Expected one final stream log entry, got %d: %s", len(logLines), content)
	}
	if strings.Count(content, "data: chunk1") != 1 || strings.Count(content, "data: chunk2") != 1 {
		t.Fatalf("Expected each stream chunk to appear once in file logs, got: %s", content)
	}
}

func TestProxyHandler_ServeHTTP_ErrorCases(t *testing.T) {
	testCases := []struct {
		name           string
		setup          func() (*mockServer, *http.Request)
		expectedStatus int
	}{
		{
			name: "Invalid request body",
			setup: func() (*mockServer, *http.Request) {
				mock := newMockServer()
				req := httptest.NewRequest("POST", "/test", &badReader{})
				return mock, req
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Target server error",
			setup: func() (*mockServer, *http.Request) {
				// 创建会返回错误的模拟服务器
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))

				req := httptest.NewRequest("GET", "/error", nil)
				return &mockServer{Server: ts, responses: make(map[string]*mockResponse)}, req
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "Invalid URL",
			setup: func() (*mockServer, *http.Request) {
				mock := newMockServer()
				// 关闭mock服务器使其URL无效
				mock.Close()
				req := httptest.NewRequest("GET", "/test", nil)
				return mock, req
			},
			expectedStatus: http.StatusBadGateway,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			logger, _ := NewLogger(tempDir)
			defer logger.Close()

			mock, req := tc.setup()
			if tc.name != "Invalid URL" {
				defer mock.Close()
			}

			// 对于Invalid URL测试，使用已关闭的服务器URL
			targetURL := mock.URL
			if tc.name == "Invalid URL" {
				targetURL = "http://invalid.url:12345"
			}

			handler := NewProxyHandler(targetURL, logger)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, rr.Code)
			}
		})
	}
}

func TestProxyHandler_QueryParameters(t *testing.T) {
	tempDir := t.TempDir()
	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// 创建模拟目标服务器
	mock := newMockServer()
	defer mock.Close()

	// 添加响应处理器来验证查询参数
	mock.Server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "param1=value1&param2=value2" {
			t.Errorf("Expected query parameters to be preserved, got %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"query":"received"}`))
	})

	handler := NewProxyHandler(mock.URL, logger)

	// 创建带查询参数的测试请求
	req := httptest.NewRequest("GET", "/test?param1=value1&param2=value2", nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestProxyHandler_HeaderForwarding(t *testing.T) {
	tempDir := t.TempDir()
	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// 创建模拟目标服务器
	mock := newMockServer()
	defer mock.Close()

	// 添加响应处理器来验证头部转发
	mock.Server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			t.Errorf("Expected Authorization header to be forwarded, got %s", authHeader)
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type header to be forwarded, got %s", contentType)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"headers":"received"}`))
	})

	handler := NewProxyHandler(mock.URL, logger)

	// 创建带自定义头部的测试请求
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"test":"data"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestProxyHandler_Timeout(t *testing.T) {
	tempDir := t.TempDir()
	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// 创建慢响应的模拟服务器
	mock := newMockServer()
	defer mock.Close()

	mock.Server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond) // 超过默认超时时间
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"timeout":"test"}`))
	})

	handler := NewProxyHandler(mock.URL, logger)

	// 创建测试请求
	req := httptest.NewRequest("GET", "/timeout", nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// 应该成功完成，因为我们没有设置超时
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for non-streaming, got %d", rr.Code)
	}
}

func TestProxyHandler_InvalidProxyRequest(t *testing.T) {
	tempDir := t.TempDir()
	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Use invalid URL with control characters
	handler := NewProxyHandler("http://\x00invalid", logger)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for invalid proxy URL, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Failed to create proxy request") {
		t.Errorf("Expected error message about proxy request creation")
	}
}

func TestProxyHandler_StreamingWithoutFlusher(t *testing.T) {
	tempDir := t.TempDir()
	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// 创建模拟目标服务器
	mock := newMockServer()
	defer mock.Close()

	mock.addResponse("/stream", &mockResponse{
		statusCode: http.StatusOK,
		headers:    map[string]string{"Content-Type": "text/event-stream"},
		body:       "data: test\n",
		stream:     true,
	})

	handler := NewProxyHandler(mock.URL, logger)

	// 创建不支持Flusher的自定义ResponseWriter
	req := httptest.NewRequest("GET", "/stream", nil)
	rr := newNonFlushingRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code() != http.StatusInternalServerError {
		t.Errorf("Expected status 500 when Flusher not supported, got %d", rr.Code())
	}
	if !strings.Contains(rr.Body().String(), "Streaming unsupported") {
		t.Errorf("Expected error message about streaming unsupported")
	}
}

func TestProxyHandler_StreamingReadError(t *testing.T) {
	tempDir := t.TempDir()
	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// 创建模拟服务器，会在流式响应中断开连接
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// 写入一些数据后关闭连接
		w.Write([]byte("data: chunk1\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// 强制关闭底层连接以模拟读取错误
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	}))
	defer mock.Close()

	handler := NewProxyHandler(mock.URL, logger)

	req := httptest.NewRequest("GET", "/stream", nil)
	rr := httptest.NewRecorder()
	w := &streamingRecorder{ResponseRecorder: rr}

	handler.ServeHTTP(w, req)

	// 验证日志文件包含错误
	content, _ := os.ReadFile(logger.logFile.Name())

	// Should have logged the chunk before error
	if !strings.Contains(string(content), "data: chunk1") {
		t.Error("Expected first chunk to be logged")
	}
}

func TestProxyHandler_StreamingWriteError(t *testing.T) {
	tempDir := t.TempDir()
	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// 创建模拟目标服务器
	mock := newMockServer()
	defer mock.Close()

	mock.addResponse("/stream", &mockResponse{
		statusCode: http.StatusOK,
		headers:    map[string]string{"Content-Type": "text/event-stream"},
		body:       "data: chunk1\ndata: chunk2\n",
		stream:     true,
	})

	handler := NewProxyHandler(mock.URL, logger)

	req := httptest.NewRequest("GET", "/stream", nil)
	// Use a failing writer that errors on write
	rr := &failingWriter{
		ResponseRecorder: httptest.NewRecorder(),
		failAfter:        1, // Fail after first write
	}

	handler.ServeHTTP(rr, req)

	// 验证错误被记录
	content, _ := os.ReadFile(logger.logFile.Name())

	if !strings.Contains(string(content), "error") {
		t.Error("Expected write error to be logged")
	}
}

func TestProxyHandler_RegularResponseReadError(t *testing.T) {
	tempDir := t.TempDir()
	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// 创建会返回读取错误的模拟服务器
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use a custom ResponseWriter that causes read errors
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("ResponseWriter doesn't support hijacking")
		}

		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)

		// Hijack the connection and close it to simulate read error
		conn, _, _ := hj.Hijack()
		conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 100\r\n\r\npartial"))
		conn.Close()
	}))
	defer mock.Close()

	handler := NewProxyHandler(mock.URL, logger)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// 验证错误被记录
	content, _ := os.ReadFile(logger.logFile.Name())

	// Should have an error logged or bad gateway response
	if rr.Code != http.StatusBadGateway && !strings.Contains(string(content), "error") {
		t.Error("Expected connection error to be handled")
	}
}

func TestProxyHandler_RegularResponseWriteError(t *testing.T) {
	tempDir := t.TempDir()
	logger, err := NewLogger(tempDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// 创建模拟目标服务器
	mock := newMockServer()
	defer mock.Close()

	mock.addResponse("/test", &mockResponse{
		statusCode: http.StatusOK,
		headers:    map[string]string{"Content-Type": "application/json"},
		body:       `{"response":"test"}`,
		stream:     false,
	})

	handler := NewProxyHandler(mock.URL, logger)

	req := httptest.NewRequest("GET", "/test", nil)
	// Use a failing writer
	rr := &failingWriter{
		ResponseRecorder: httptest.NewRecorder(),
		failAfter:        0, // Fail immediately
	}

	handler.ServeHTTP(rr, req)

	// 验证错误被记录
	content, _ := os.ReadFile(logger.logFile.Name())

	if !strings.Contains(string(content), "error") {
		t.Error("Expected write error to be logged")
	}
}

// nonFlushingRecorder is a ResponseWriter that doesn't implement http.Flusher
type nonFlushingRecorder struct {
	code   int
	header http.Header
	body   *bytes.Buffer
}

func newNonFlushingRecorder() *nonFlushingRecorder {
	return &nonFlushingRecorder{
		header: make(http.Header),
		body:   new(bytes.Buffer),
		code:   200,
	}
}

func (r *nonFlushingRecorder) Header() http.Header {
	return r.header
}

func (r *nonFlushingRecorder) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

func (r *nonFlushingRecorder) WriteHeader(statusCode int) {
	r.code = statusCode
}

func (r *nonFlushingRecorder) Code() int {
	return r.code
}

func (r *nonFlushingRecorder) Body() *bytes.Buffer {
	return r.body
}

// failingWriter is a ResponseWriter that fails after a certain number of writes
type failingWriter struct {
	*httptest.ResponseRecorder
	writeCount int
	failAfter  int
}

func (fw *failingWriter) Write(p []byte) (int, error) {
	if fw.writeCount >= fw.failAfter {
		return 0, http.ErrBodyNotAllowed
	}
	fw.writeCount++
	return fw.ResponseRecorder.Write(p)
}

func (fw *failingWriter) Flush() {
	// Implement Flusher interface
}

func TestStreamingRecorder_Flush(t *testing.T) {
	// Test that streamingRecorder implements Flusher interface correctly
	rr := httptest.NewRecorder()
	sr := &streamingRecorder{ResponseRecorder: rr}

	// Verify it implements http.Flusher
	if _, ok := interface{}(sr).(http.Flusher); !ok {
		t.Error("streamingRecorder should implement http.Flusher")
	}

	// Call Flush to ensure coverage
	sr.Flush()
}
