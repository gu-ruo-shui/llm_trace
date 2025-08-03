package proxy

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DatabaseRequestLog struct {
	ID           int64     `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	Method       string    `json:"method"`
	URL          string    `json:"url"`
	Headers      string    `json:"headers"`
	Body         string    `json:"body,omitempty"`
	ResponseCode int       `json:"response_code,omitempty"`
	Response     string    `json:"response,omitempty"`
	IsStream     bool      `json:"is_stream"`
	Error        string    `json:"error,omitempty"`
	Duration     int64     `json:"duration_ms"`
}

type DatabaseLogger struct {
	db *sql.DB
}

func NewDatabaseLogger(dbPath string) (*DatabaseLogger, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return &DatabaseLogger{db: db}, nil
}

func createTables(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS request_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		method TEXT NOT NULL,
		url TEXT NOT NULL,
		headers TEXT,
		body TEXT,
		response_code INTEGER,
		response TEXT,
		is_stream BOOLEAN DEFAULT FALSE,
		error TEXT,
		duration_ms INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_request_logs_timestamp ON request_logs(timestamp);
	CREATE INDEX IF NOT EXISTS idx_request_logs_url ON request_logs(url);
	CREATE INDEX IF NOT EXISTS idx_request_logs_method ON request_logs(method);
	CREATE INDEX IF NOT EXISTS idx_request_logs_response_code ON request_logs(response_code);
	`

	_, err := db.Exec(query)
	return err
}

func (dl *DatabaseLogger) LogRequest(req *http.Request, body []byte) *DatabaseRequestLog {
	headersJSON, _ := json.Marshal(req.Header)

	log := &DatabaseRequestLog{
		Timestamp: time.Now(),
		Method:    req.Method,
		URL:       req.URL.String(),
		Headers:   string(headersJSON),
		Body:      string(body),
	}

	return log
}

func (dl *DatabaseLogger) LogResponse(log *DatabaseRequestLog, resp *http.Response, body []byte, isStream bool, duration time.Duration) {
	if resp != nil {
		log.ResponseCode = resp.StatusCode
	}
	log.Response = string(body)
	log.IsStream = isStream
	log.Duration = duration.Milliseconds()

	dl.writeLog(log)
}

func (dl *DatabaseLogger) LogError(log *DatabaseRequestLog, err error) {
	if err != nil {
		log.Error = err.Error()
	}
	dl.writeLog(log)
}

func (dl *DatabaseLogger) LogStreamChunk(log *DatabaseRequestLog, chunk string, duration time.Duration) {
	streamLog := &DatabaseRequestLog{
		Timestamp: time.Now(),
		Method:    log.Method,
		URL:       log.URL,
		Response:  chunk,
		IsStream:  true,
		Duration:  duration.Milliseconds(),
	}
	dl.writeLog(streamLog)
}

func (dl *DatabaseLogger) writeLog(log *DatabaseRequestLog) {
	query := `
	INSERT INTO request_logs (timestamp, method, url, headers, body, response_code, response, is_stream, error, duration_ms)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := dl.db.Exec(query,
		log.Timestamp,
		log.Method,
		log.URL,
		log.Headers,
		log.Body,
		log.ResponseCode,
		log.Response,
		log.IsStream,
		log.Error,
		log.Duration,
	)

	if err != nil {
		fmt.Printf("Failed to write log to database: %v\n", err)
	}
}

func (dl *DatabaseLogger) GetLogs(limit int, offset int) ([]DatabaseRequestLog, error) {
	query := `
	SELECT id, timestamp, method, url, headers, body, response_code, response, is_stream, error, duration_ms
	FROM request_logs
	ORDER BY timestamp DESC
	LIMIT ? OFFSET ?
	`

	rows, err := dl.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []DatabaseRequestLog
	for rows.Next() {
		var log DatabaseRequestLog
		var headers, body, response, errorMsg sql.NullString
		err := rows.Scan(
			&log.ID,
			&log.Timestamp,
			&log.Method,
			&log.URL,
			&headers,
			&body,
			&log.ResponseCode,
			&response,
			&log.IsStream,
			&errorMsg,
			&log.Duration,
		)
		if err != nil {
			return nil, err
		}
		log.Headers = headers.String
		log.Body = body.String
		log.Response = response.String
		log.Error = errorMsg.String
		logs = append(logs, log)
	}

	return logs, nil
}

func (dl *DatabaseLogger) GetLogsByURL(url string, limit int) ([]DatabaseRequestLog, error) {
	query := `
	SELECT id, timestamp, method, url, headers, body, response_code, response, is_stream, error, duration_ms
	FROM request_logs
	WHERE url = ?
	ORDER BY timestamp DESC
	LIMIT ?
	`

	rows, err := dl.db.Query(query, url, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []DatabaseRequestLog
	for rows.Next() {
		var log DatabaseRequestLog
		var headers, body, response, errorMsg sql.NullString
		err := rows.Scan(
			&log.ID,
			&log.Timestamp,
			&log.Method,
			&log.URL,
			&headers,
			&body,
			&log.ResponseCode,
			&response,
			&log.IsStream,
			&errorMsg,
			&log.Duration,
		)
		if err != nil {
			return nil, err
		}
		log.Headers = headers.String
		log.Body = body.String
		log.Response = response.String
		log.Error = errorMsg.String
		logs = append(logs, log)
	}

	return logs, nil
}

func (dl *DatabaseLogger) GetErrorLogs(limit int) ([]DatabaseRequestLog, error) {
	query := `
	SELECT id, timestamp, method, url, headers, body, response_code, response, is_stream, error, duration_ms
	FROM request_logs
	WHERE error IS NOT NULL AND error != ''
	ORDER BY timestamp DESC
	LIMIT ?
	`

	rows, err := dl.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []DatabaseRequestLog
	for rows.Next() {
		var log DatabaseRequestLog
		var headers, body, response, errorMsg sql.NullString
		err := rows.Scan(
			&log.ID,
			&log.Timestamp,
			&log.Method,
			&log.URL,
			&headers,
			&body,
			&log.ResponseCode,
			&response,
			&log.IsStream,
			&errorMsg,
			&log.Duration,
		)
		if err != nil {
			return nil, err
		}
		log.Headers = headers.String
		log.Body = body.String
		log.Response = response.String
		log.Error = errorMsg.String
		logs = append(logs, log)
	}

	return logs, nil
}

func (dl *DatabaseLogger) DeleteOldLogs(days int) error {
	query := `
	DELETE FROM request_logs
	WHERE timestamp < datetime('now', '-' || ? || ' days')
	`

	_, err := dl.db.Exec(query, days)
	return err
}

func (dl *DatabaseLogger) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total count
	var totalCount int
	err := dl.db.QueryRow("SELECT COUNT(*) FROM request_logs").Scan(&totalCount)
	if err != nil {
		return nil, err
	}
	stats["total_logs"] = totalCount

	// Error count
	var errorCount int
	err = dl.db.QueryRow("SELECT COUNT(*) FROM request_logs WHERE error IS NOT NULL AND error != ''").Scan(&errorCount)
	if err != nil {
		return nil, err
	}
	stats["error_logs"] = errorCount

	// Today's count
	var todayCount int
	err = dl.db.QueryRow("SELECT COUNT(*) FROM request_logs WHERE date(timestamp) = date('now')").Scan(&todayCount)
	if err != nil {
		return nil, err
	}
	stats["today_logs"] = todayCount

	// Average response time (excluding error logs with 0 duration)
	var avgDuration sql.NullFloat64
	err = dl.db.QueryRow("SELECT AVG(duration_ms) FROM request_logs WHERE duration_ms > 0 AND (error IS NULL OR error = '')").Scan(&avgDuration)
	if err != nil {
		return nil, err
	}
	if avgDuration.Valid {
		stats["avg_duration_ms"] = avgDuration.Float64
	} else {
		stats["avg_duration_ms"] = 0.0
	}

	return stats, nil
}

func (dl *DatabaseLogger) Close() error {
	if dl.db != nil {
		return dl.db.Close()
	}
	return nil
}
