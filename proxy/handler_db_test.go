package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestProxyHandlerDB_ServeHTTP_RegularResponse(t *testing.T) {
	// 创建临时数据库
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
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
	handler := NewProxyHandlerDB(mock.URL, logger)

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

	// 验证数据库中的日志
	logs, err := logger.GetLogs(10, 0)
	if err != nil {
		t.Fatalf("Failed to get logs from database: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(logs))
	}

	if logs[0].Method != "POST" {
		t.Errorf("Expected method POST, got %s", logs[0].Method)
	}
	if logs[0].URL != "/test" {
		t.Errorf("Expected URL /test, got %s", logs[0].URL)
	}
	if logs[0].ResponseCode != 200 {
		t.Errorf("Expected response code 200, got %d", logs[0].ResponseCode)
	}
	if logs[0].Duration <= 0 {
		t.Errorf("Expected duration to be positive, got %d", logs[0].Duration)
	}
}

func TestProxyHandlerDB_ServeHTTP_StreamingResponse(t *testing.T) {
	// 创建临时数据库
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
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
	handler := NewProxyHandlerDB(mock.URL, logger)

	// 创建测试请求
	req := httptest.NewRequest("POST", "/stream", strings.NewReader(`{"stream":true}`))

	// 创建支持流式的响应记录器
	rr := httptest.NewRecorder()

	// 确保响应记录器支持Flusher
	if _, ok := rr.(http.Flusher); !ok {
		customRR := &streamingRecorder{ResponseRecorder: rr}
		rr = customRR
	}

	handler.ServeHTTP(rr, req)

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

	// 验证数据库中的日志
	logs, err := logger.GetLogs(10, 0)
	if err != nil {
		t.Fatalf("Failed to get logs from database: %v", err)
	}

	// 应该至少有一条流式日志
	var streamLogs int
	for _, log := range logs {
		if log.IsStream {
			streamLogs++
		}
	}

	if streamLogs == 0 {
		t.Error("Expected to find stream logs in database")
	}
}

func TestProxyHandlerDB_ServeHTTP_ErrorCases(t *testing.T) {
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
			dbPath := filepath.Join(tempDir, "test.db")

			logger, err := NewDatabaseLogger(dbPath)
			if err != nil {
				t.Fatalf("Failed to create database logger: %v", err)
			}
			defer logger.Close()

			mock, req := tc.setup()
			if tc.name != "Invalid URL" {
				defer mock.Close()
			}

			targetURL := mock.URL
			if tc.name == "Invalid URL" {
				targetURL = "http://invalid.url:12345"
			}

			handler := NewProxyHandlerDB(targetURL, logger)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, rr.Code)
			}

			// 验证错误被记录到数据库
			if tc.name == "Invalid URL" || tc.name == "Target server error" {
				logs, err := logger.GetErrorLogs(10)
				if err != nil {
					t.Logf("Failed to get error logs: %v", err)
					return
				}
				if len(logs) == 0 {
					t.Log("Expected error to be logged to database")
				}
			}
		})
	}
}

func TestProxyHandlerDB_QueryParameters(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}
	defer logger.Close()

	// 创建模拟目标服务器
	mock := newMockServer()
	defer mock.Close()

	mock.Server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "param1=value1&param2=value2" {
			t.Errorf("Expected query parameters to be preserved, got %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"query":"received"}`))
	})

	handler := NewProxyHandlerDB(mock.URL, logger)

	// 创建带查询参数的测试请求
	req := httptest.NewRequest("GET", "/test?param1=value1&param2=value2", nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// 验证日志包含查询参数
	logs, err := logger.GetLogsByURL("/test?param1=value1&param2=value2", 10)
	if err != nil {
		t.Fatalf("Failed to get logs by URL: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 log entry with query parameters, got %d", len(logs))
	}
}

func TestProxyHandlerDB_HeaderForwarding(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}
	defer logger.Close()

	// 创建模拟目标服务器
	mock := newMockServer()
	defer mock.Close()

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

	handler := NewProxyHandlerDB(mock.URL, logger)

	// 创建带自定义头部的测试请求
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"test":"data"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// 验证Headers被正确记录
	logs, err := logger.GetLogs(10, 0)
	if err != nil {
		t.Fatalf("Failed to get logs: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(logs))
	}

	var headers map[string][]string
	err = json.Unmarshal([]byte(logs[0].Headers), &headers)
	if err != nil {
		t.Errorf("Failed to unmarshal headers: %v", err)
	}

	if headers["Authorization"][0] != "Bearer test-token" {
		t.Errorf("Expected Authorization header to be logged")
	}
}

func TestProxyHandlerDB_DurationCalculation(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}
	defer logger.Close()

	// 创建模拟目标服务器
	mock := newMockServer()
	defer mock.Close()

	mock.Server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond) // 确保可测量的延迟
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"delayed":"response"}`))
	})

	handler := NewProxyHandlerDB(mock.URL, logger)

	// 创建测试请求
	req := httptest.NewRequest("GET", "/delay", nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// 验证持续时间被正确记录
	logs, err := logger.GetLogs(10, 0)
	if err != nil {
		t.Fatalf("Failed to get logs: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(logs))
	}

	if logs[0].Duration < 40 {
		t.Errorf("Expected duration >= 40ms, got %d", logs[0].Duration)
	}
}
