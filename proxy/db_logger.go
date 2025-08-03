package proxy

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type DatabaseRequestLog struct {
	ID           int64     `json:"id"`
	RequestUUID  string    `json:"request_uuid"`
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

type SSEEvent struct {
	ID          int64     `json:"id"`
	RequestUUID string    `json:"request_uuid"`
	Timestamp   time.Time `json:"timestamp"`
	EventType   string    `json:"event_type"`
	Data        string    `json:"data"`
	Sequence    int       `json:"sequence"`
}

type ProcessedSSEResponse struct {
	MessageStart  map[string]interface{}   `json:"message_start,omitempty"`
	ContentBlocks []map[string]interface{} `json:"content_blocks,omitempty"`
	MessageDelta  map[string]interface{}   `json:"message_delta,omitempty"`
	MessageStop   map[string]interface{}   `json:"message_stop,omitempty"`
	ProcessedText string                   `json:"processed_text,omitempty"`
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
		request_uuid TEXT NOT NULL UNIQUE,
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
	
	CREATE TABLE IF NOT EXISTS sse_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		request_uuid TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		event_type TEXT NOT NULL,
		data TEXT,
		sequence INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	);
	
	CREATE INDEX IF NOT EXISTS idx_request_logs_timestamp ON request_logs(timestamp);
	CREATE INDEX IF NOT EXISTS idx_request_logs_url ON request_logs(url);
	CREATE INDEX IF NOT EXISTS idx_request_logs_method ON request_logs(method);
	CREATE INDEX IF NOT EXISTS idx_request_logs_response_code ON request_logs(response_code);
	CREATE INDEX IF NOT EXISTS idx_request_logs_uuid ON request_logs(request_uuid);
	
	CREATE INDEX IF NOT EXISTS idx_sse_events_request_uuid ON sse_events(request_uuid);
	CREATE INDEX IF NOT EXISTS idx_sse_events_timestamp ON sse_events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_sse_events_sequence ON sse_events(request_uuid, sequence);
	`

	_, err := db.Exec(query)
	return err
}

func (dl *DatabaseLogger) LogRequest(req *http.Request, body []byte) *DatabaseRequestLog {
	headersJSON, _ := json.Marshal(req.Header)

	log := &DatabaseRequestLog{
		RequestUUID: uuid.New().String(),
		Timestamp:   time.Now(),
		Method:      req.Method,
		URL:         req.URL.String(),
		Headers:     string(headersJSON),
		Body:        string(body),
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

func (dl *DatabaseLogger) LogSSEEvent(requestUUID string, eventType string, data string, sequence int) {
	event := &SSEEvent{
		RequestUUID: requestUUID,
		Timestamp:   time.Now(),
		EventType:   eventType,
		Data:        data,
		Sequence:    sequence,
	}
	dl.writeSSEEvent(event)
}

func (dl *DatabaseLogger) LogStreamChunk(log *DatabaseRequestLog, chunk string, duration time.Duration) {
	// For stream chunks, we don't create new log entries
	// Instead, we just accumulate the chunks for the final response
	// This method is kept for backward compatibility but doesn't write to database
}

func (dl *DatabaseLogger) writeLog(log *DatabaseRequestLog) {
	query := `
	INSERT INTO request_logs (request_uuid, timestamp, method, url, headers, body, response_code, response, is_stream, error, duration_ms)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := dl.db.Exec(query,
		log.RequestUUID,
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

func (dl *DatabaseLogger) writeSSEEvent(event *SSEEvent) {
	query := `
	INSERT INTO sse_events (request_uuid, timestamp, event_type, data, sequence)
	VALUES (?, ?, ?, ?, ?)
	`

	_, err := dl.db.Exec(query,
		event.RequestUUID,
		event.Timestamp,
		event.EventType,
		event.Data,
		event.Sequence,
	)

	if err != nil {
		fmt.Printf("Failed to write SSE event to database: %v\n", err)
	}
}

func (dl *DatabaseLogger) ProcessSSEEvents(requestUUID string) (*ProcessedSSEResponse, error) {
	query := `
	SELECT event_type, data, sequence 
	FROM sse_events 
	WHERE request_uuid = ? 
	ORDER BY sequence ASC
	`

	rows, err := dl.db.Query(query, requestUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	processed := &ProcessedSSEResponse{
		ContentBlocks: make([]map[string]interface{}, 0),
	}

	var textParts []string
	contentBlockIndex := 0

	for rows.Next() {
		var eventType, data string
		var sequence int
		if err := rows.Scan(&eventType, &data, &sequence); err != nil {
			continue
		}

		if eventType == "message_start" {
			var messageStart map[string]interface{}
			if err := json.Unmarshal([]byte(data), &messageStart); err == nil {
				processed.MessageStart = messageStart
			}
		} else if eventType == "content_block_start" {
			var contentBlockStart map[string]interface{}
			if err := json.Unmarshal([]byte(data), &contentBlockStart); err == nil {
				processed.ContentBlocks = append(processed.ContentBlocks, contentBlockStart)
				contentBlockIndex = len(processed.ContentBlocks) - 1
			}
		} else if eventType == "content_block_delta" {
			var delta map[string]interface{}
			if err := json.Unmarshal([]byte(data), &delta); err == nil {
				if deltaInfo, ok := delta["delta"].(map[string]interface{}); ok {
					if text, ok := deltaInfo["text"].(string); ok {
						textParts = append(textParts, text)
					}
					if partialJSON, ok := deltaInfo["partial_json"].(string); ok {
						textParts = append(textParts, partialJSON)
					}
				}
				if contentBlockIndex < len(processed.ContentBlocks) {
					if processed.ContentBlocks[contentBlockIndex]["deltas"] == nil {
						processed.ContentBlocks[contentBlockIndex]["deltas"] = make([]map[string]interface{}, 0)
					}
					deltas := processed.ContentBlocks[contentBlockIndex]["deltas"].([]map[string]interface{})
					processed.ContentBlocks[contentBlockIndex]["deltas"] = append(deltas, delta)
				}
			}
		} else if eventType == "message_delta" {
			var messageDelta map[string]interface{}
			if err := json.Unmarshal([]byte(data), &messageDelta); err == nil {
				processed.MessageDelta = messageDelta
			}
		} else if eventType == "message_stop" {
			var messageStop map[string]interface{}
			if err := json.Unmarshal([]byte(data), &messageStop); err == nil {
				processed.MessageStop = messageStop
			}
		}
	}

	processed.ProcessedText = strings.Join(textParts, "")
	return processed, nil
}

func (dl *DatabaseLogger) GetLogs(limit int, offset int) ([]DatabaseRequestLog, error) {
	query := `
	SELECT id, request_uuid, timestamp, method, url, headers, body, response_code, response, is_stream, error, duration_ms
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
			&log.RequestUUID,
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
	SELECT id, request_uuid, timestamp, method, url, headers, body, response_code, response, is_stream, error, duration_ms
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
			&log.RequestUUID,
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
