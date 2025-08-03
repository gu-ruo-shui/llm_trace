package proxy

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewDatabaseLogger(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer logger.Close()

	// 验证数据库文件是否创建
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Expected database file to be created")
	}

	// 验证表是否存在
	var tableName string
	err = logger.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='request_logs'").Scan(&tableName)
	if err != nil {
		t.Errorf("Expected request_logs table to be created")
	}

	if tableName != "request_logs" {
		t.Errorf("Expected table name request_logs, got %s", tableName)
	}
}

func TestDatabaseLogger_LogRequest(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}
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

	// 验证Headers被正确序列化为JSON
	var headers map[string][]string
	err = json.Unmarshal([]byte(log.Headers), &headers)
	if err != nil {
		t.Errorf("Failed to unmarshal headers JSON: %v", err)
	}

	if headers["Content-Type"][0] != "application/json" {
		t.Errorf("Expected Content-Type header to be logged")
	}
}

func TestDatabaseLogger_LogResponse(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}
	defer logger.Close()

	log := &DatabaseRequestLog{
		Method: "POST",
		URL:    "/test",
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
	}

	body := []byte(`{"response":"data"}`)
	duration := 150 * time.Millisecond
	logger.LogResponse(log, resp, body, false, duration)

	if log.ResponseCode != 200 {
		t.Errorf("Expected response code 200, got %d", log.ResponseCode)
	}
	if log.Response != `{"response":"data"}` {
		t.Errorf("Expected response body to be logged correctly")
	}
	if log.IsStream != false {
		t.Errorf("Expected IsStream to be false")
	}
	if log.Duration != 150 {
		t.Errorf("Expected duration 150ms, got %d", log.Duration)
	}

	// 验证数据被写入数据库
	var count int
	err = logger.db.QueryRow("SELECT COUNT(*) FROM request_logs WHERE url = '/test'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 log entry, got %d", count)
	}
}

func TestDatabaseLogger_LogError(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}
	defer logger.Close()

	log := &DatabaseRequestLog{
		Method:    "POST",
		URL:       "/test",
		Timestamp: time.Now(),
	}

	testErr := fmt.Errorf("test error: invalid JSON at offset 123")
	logger.LogError(log, testErr)

	if log.Error == "" {
		t.Error("Expected error to be logged")
	}

	// 验证错误被写入数据库
	var errorMsg string
	err = logger.db.QueryRow("SELECT error FROM request_logs WHERE url = '/test'").Scan(&errorMsg)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	if errorMsg == "" {
		t.Error("Expected error message to be stored in database")
	}
}

func TestDatabaseLogger_LogStreamChunk(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}
	defer logger.Close()

	log := &DatabaseRequestLog{
		Method: "POST",
		URL:    "/stream",
	}

	duration := 50 * time.Millisecond
	logger.LogStreamChunk(log, "data: test chunk\n", duration)

	// 验证流式数据被写入数据库
	var response string
	err = logger.db.QueryRow("SELECT response FROM request_logs WHERE is_stream = 1").Scan(&response)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	if response != "data: test chunk\n" {
		t.Errorf("Expected stream chunk to be logged, got %s", response)
	}
}

func TestDatabaseLogger_GetLogs(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}
	defer logger.Close()

	// 插入测试数据
	_, err = logger.db.Exec(`
		INSERT INTO request_logs (timestamp, method, url, response_code, duration_ms) VALUES
		(datetime('now', '-1 day'), 'GET', '/test1', 200, 100),
		(datetime('now', '-2 day'), 'POST', '/test2', 201, 200),
		(datetime('now', '-3 day'), 'PUT', '/test3', 204, 150)
	`)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// 测试获取所有日志
	logs, err := logger.GetLogs(10, 0)
	if err != nil {
		t.Fatalf("Failed to get logs: %v", err)
	}

	if len(logs) != 3 {
		t.Errorf("Expected 3 logs, got %d", len(logs))
	}

	// 测试分页
	logs, err = logger.GetLogs(2, 0)
	if len(logs) != 2 {
		t.Errorf("Expected 2 logs with limit 2, got %d", len(logs))
	}

	logs, err = logger.GetLogs(2, 2)
	if len(logs) != 1 {
		t.Errorf("Expected 1 log with offset 2, got %d", len(logs))
	}
}

func TestDatabaseLogger_GetErrorLogs(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}
	defer logger.Close()

	// 插入测试数据
	_, err = logger.db.Exec(`
		INSERT INTO request_logs (timestamp, method, url, response_code, error) VALUES
		(datetime('now'), 'GET', '/success', 200, NULL),
		(datetime('now'), 'POST', '/error1', 500, 'Internal server error'),
		(datetime('now'), 'PUT', '/error2', 404, 'Not found')
	`)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// 测试获取错误日志
	logs, err := logger.GetErrorLogs(10)
	if err != nil {
		t.Fatalf("Failed to get error logs: %v", err)
	}

	if len(logs) != 2 {
		t.Errorf("Expected 2 error logs, got %d", len(logs))
	}

	for _, log := range logs {
		if log.Error == "" {
			t.Errorf("Expected error message in log")
		}
	}
}

func TestDatabaseLogger_DeleteOldLogs(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}
	defer logger.Close()

	// 插入测试数据
	_, err = logger.db.Exec(`
		INSERT INTO request_logs (timestamp, method, url) VALUES
		(datetime('now', '-10 day'), 'GET', '/old1'),
		(datetime('now', '-5 day'), 'POST', '/old2'),
		(datetime('now'), 'PUT', '/recent')
	`)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// 验证初始数据
	var initialCount int
	err = logger.db.QueryRow("SELECT COUNT(*) FROM request_logs").Scan(&initialCount)
	if initialCount != 3 {
		t.Fatalf("Expected 3 initial logs, got %d", initialCount)
	}

	// 删除7天前的日志
	err = logger.DeleteOldLogs(7)
	if err != nil {
		t.Fatalf("Failed to delete old logs: %v", err)
	}

	// 验证删除结果
	var remainingCount int
	err = logger.db.QueryRow("SELECT COUNT(*) FROM request_logs").Scan(&remainingCount)
	if remainingCount != 2 {
		t.Errorf("Expected 2 remaining logs after deletion, got %d", remainingCount)
	}
}

func TestDatabaseLogger_GetStats(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}
	defer logger.Close()

	// 插入测试数据
	_, err = logger.db.Exec(`
		INSERT INTO request_logs (timestamp, method, url, response_code, duration_ms, error) VALUES
		(datetime('now'), 'GET', '/test1', 200, 100, NULL),
		(datetime('now'), 'POST', '/test2', 201, 200, NULL),
		(datetime('now'), 'PUT', '/test3', 500, 0, 'Error message'),
		(datetime('now'), 'DELETE', '/test4', 204, 150, NULL)
	`)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// 测试获取统计信息
	stats, err := logger.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats["total_logs"] != 4 {
		t.Errorf("Expected total_logs 4, got %v", stats["total_logs"])
	}
	if stats["error_logs"] != 1 {
		t.Errorf("Expected error_logs 1, got %v", stats["error_logs"])
	}
	if stats["today_logs"] != 4 {
		t.Errorf("Expected today_logs 4, got %v", stats["today_logs"])
	}
	// Average should be (100 + 200 + 150) / 3 = 150, excluding error log with 0 duration
	if stats["avg_duration_ms"] != 150.0 {
		t.Errorf("Expected avg_duration_ms 150.0, got %v", stats["avg_duration_ms"])
	}
}

func TestDatabaseLogger_ConcurrentWrites(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}
	defer logger.Close()

	// 并发写入测试
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			log := &DatabaseRequestLog{
				Method:    "GET",
				URL:       "/concurrent/" + string(rune('a'+id)),
				Timestamp: time.Now(),
			}
			logger.writeLog(log)
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证所有内容都被写入
	var count int
	err = logger.db.QueryRow("SELECT COUNT(*) FROM request_logs WHERE url LIKE '/concurrent/%'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	if count != 10 {
		t.Errorf("Expected 10 concurrent logs, got %d", count)
	}
}

func TestDatabaseLogger_Close(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}

	// 关闭数据库
	err = logger.Close()
	if err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	// 验证数据库连接已关闭
	var count int
	err = logger.db.QueryRow("SELECT COUNT(*) FROM request_logs").Scan(&count)
	if err == nil {
		t.Error("Expected error when querying closed database")
	}
}

func TestDatabaseLogger_Integration(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "integration_test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}
	defer logger.Close()

	// 完整的日志记录流程测试
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader(`{"message":"hello"}`))
	req.Header.Set("Authorization", "Bearer token123")

	body := []byte(`{"message":"hello"}`)
	log := logger.LogRequest(req, body)

	resp := &http.Response{StatusCode: http.StatusOK}
	responseBody := []byte(`{"reply":"world"}`)
	duration := 200 * time.Millisecond

	logger.LogResponse(log, resp, responseBody, false, duration)

	// 验证数据完整性
	var savedLog DatabaseRequestLog
	err = logger.db.QueryRow(`
		SELECT id, timestamp, method, url, headers, body, response_code, response, is_stream, duration_ms
		FROM request_logs WHERE url = '/api/test'
	`).Scan(
		&savedLog.ID,
		&savedLog.Timestamp,
		&savedLog.Method,
		&savedLog.URL,
		&savedLog.Headers,
		&savedLog.Body,
		&savedLog.ResponseCode,
		&savedLog.Response,
		&savedLog.IsStream,
		&savedLog.Duration,
	)

	if err != nil {
		t.Fatalf("Failed to retrieve saved log: %v", err)
	}

	if savedLog.Method != "POST" {
		t.Errorf("Expected method POST, got %s", savedLog.Method)
	}
	if savedLog.ResponseCode != 200 {
		t.Errorf("Expected response code 200, got %d", savedLog.ResponseCode)
	}
	if savedLog.Duration != 200 {
		t.Errorf("Expected duration 200ms, got %d", savedLog.Duration)
	}

	// 验证Headers JSON
	var headers map[string][]string
	err = json.Unmarshal([]byte(savedLog.Headers), &headers)
	if err != nil {
		t.Errorf("Failed to unmarshal headers: %v", err)
	}
	if headers["Authorization"][0] != "Bearer token123" {
		t.Errorf("Expected Authorization header to be preserved")
	}
}

func TestNewDatabaseLogger_ErrorHandling(t *testing.T) {
	// Test invalid database path - use a path that will fail on all platforms
	tempDir := t.TempDir()

	// Create a regular file where we expect a directory
	badPath := filepath.Join(tempDir, "baddir", "test.db")
	badDirPath := filepath.Dir(badPath)

	// Create a file with the directory name to cause a conflict
	file, err := os.Create(badDirPath)
	if err == nil {
		file.Close()

		// Now try to create the database with a path that requires this to be a directory
		_, err = NewDatabaseLogger(badPath)
		if err == nil {
			t.Error("Expected error when parent path is a file not directory")
		}
	}
}

func TestDatabaseLogger_writeLog_ErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}

	// Close the database to simulate write error
	logger.db.Close()

	// Try to write log - should handle error gracefully
	log := &DatabaseRequestLog{
		Method:    "GET",
		URL:       "/test",
		Timestamp: time.Now(),
	}

	// This should not panic, just print error message
	logger.writeLog(log)
}

func TestDatabaseLogger_GetLogs_ErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}

	// Close the database to simulate query error
	logger.db.Close()

	// Try to get logs - should return error
	_, err = logger.GetLogs(10, 0)
	if err == nil {
		t.Error("Expected error when querying closed database")
	}
}

func TestDatabaseLogger_GetErrorLogs_ErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}

	// Close the database to simulate query error
	logger.db.Close()

	// Try to get error logs - should return error
	_, err = logger.GetErrorLogs(10)
	if err == nil {
		t.Error("Expected error when querying closed database")
	}
}

func TestDatabaseLogger_GetStats_ErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}

	// Close the database to simulate query error
	logger.db.Close()

	// Try to get stats - should return error
	_, err = logger.GetStats()
	if err == nil {
		t.Error("Expected error when querying closed database")
	}
}

func TestDatabaseLogger_GetStats_NoAvgDuration(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}
	defer logger.Close()

	// Insert only error logs with 0 duration
	_, err = logger.db.Exec(`
		INSERT INTO request_logs (timestamp, method, url, response_code, duration_ms, error) VALUES
		(datetime('now'), 'GET', '/error1', 500, 0, 'Error 1'),
		(datetime('now'), 'POST', '/error2', 500, 0, 'Error 2')
	`)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Get stats - avg_duration_ms should be 0.0
	stats, err := logger.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats["avg_duration_ms"] != 0.0 {
		t.Errorf("Expected avg_duration_ms 0.0 when no valid durations, got %v", stats["avg_duration_ms"])
	}
}

func TestDatabaseLogger_Close_NilDB(t *testing.T) {
	logger := &DatabaseLogger{db: nil}

	// Should not panic when db is nil
	err := logger.Close()
	if err != nil {
		t.Errorf("Expected no error when closing nil db, got %v", err)
	}
}

func TestDatabaseLogger_LogError_NilError(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}
	defer logger.Close()

	log := &DatabaseRequestLog{
		Method:    "GET",
		URL:       "/test",
		Timestamp: time.Now(),
	}

	// Log with nil error
	logger.LogError(log, nil)

	// Verify empty error is stored
	var errorMsg sql.NullString
	err = logger.db.QueryRow("SELECT error FROM request_logs WHERE url = '/test'").Scan(&errorMsg)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	if errorMsg.Valid && errorMsg.String != "" {
		t.Errorf("Expected empty error for nil error, got %s", errorMsg.String)
	}
}

func TestDatabaseLogger_ScanError(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, err := NewDatabaseLogger(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database logger: %v", err)
	}
	defer logger.Close()

	// Create a corrupted table structure to trigger scan errors
	_, err = logger.db.Exec(`
		CREATE TABLE IF NOT EXISTS corrupted_logs (
			id INTEGER PRIMARY KEY,
			bad_column TEXT
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create corrupted table: %v", err)
	}

	// Try to query with incorrect structure
	query := `SELECT id, bad_column FROM corrupted_logs`
	rows, err := logger.db.Query(query)
	if err != nil {
		t.Fatalf("Failed to execute query: %v", err)
	}
	defer rows.Close()

	// This simulates scan error scenarios
	var logs []DatabaseRequestLog
	for rows.Next() {
		var log DatabaseRequestLog
		// This will fail because we're trying to scan wrong columns
		err := rows.Scan(&log.ID, &log.Timestamp)
		if err == nil {
			t.Error("Expected scan error for mismatched columns")
		}
		logs = append(logs, log)
	}
}
