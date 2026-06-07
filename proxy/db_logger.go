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
	ToolName      string `json:"tool_name,omitempty"`
	ProcessedText string `json:"processed_text,omitempty"`
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
		request_uuid TEXT NOT NULL,
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
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
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
	log.Response = formatResponseBodyForLog(resp, body)
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
	if log.RequestUUID != "" {
		updateQuery := `
		UPDATE request_logs
		SET timestamp = ?, method = ?, url = ?, headers = ?, body = ?, response_code = ?, response = ?, is_stream = ?, error = ?, duration_ms = ?
		WHERE request_uuid = ?
		`

		result, err := dl.db.Exec(updateQuery,
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
			log.RequestUUID,
		)
		if err != nil {
			fmt.Printf("Failed to update log in database: %v\n", err)
			return
		}
		if rowsAffected, err := result.RowsAffected(); err == nil && rowsAffected > 0 {
			return
		}
	}

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
	WHERE request_uuid = ? AND event_type != 'ping'
	ORDER BY sequence ASC
	`

	rows, err := dl.db.Query(query, requestUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	processed := &ProcessedSSEResponse{}
	var textParts []string

	for rows.Next() {
		var eventType, data string
		var sequence int
		if err := rows.Scan(&eventType, &data, &sequence); err != nil {
			continue
		}

		data = strings.TrimSpace(data)
		if data == "" || data == "[DONE]" {
			continue
		}

		var eventData map[string]interface{}
		if err := json.Unmarshal([]byte(data), &eventData); err != nil {
			// Some providers stream data-only text. Preserve those chunks as
			// processed text instead of returning an empty SSE summary.
			if eventType == "message" {
				textParts = append(textParts, data)
			}
			continue
		}

		updateProcessedToolName(processed, eventData)
		textParts = append(textParts, extractProcessedTextParts(eventType, eventData)...)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	processed.ProcessedText = strings.Join(textParts, "")
	return processed, nil
}

func updateProcessedToolName(processed *ProcessedSSEResponse, eventData map[string]interface{}) {
	if processed == nil {
		return
	}

	if contentBlock, ok := eventData["content_block"].(map[string]interface{}); ok {
		if name, ok := contentBlock["name"].(string); ok && name != "" {
			processed.ToolName = name
		}
	}

	if item, ok := eventData["item"].(map[string]interface{}); ok {
		if name, ok := item["name"].(string); ok && name != "" {
			processed.ToolName = name
		}
	}

	if name, ok := eventData["name"].(string); ok && name != "" {
		processed.ToolName = name
	}

	for _, choice := range asMapSlice(eventData["choices"]) {
		delta, _ := choice["delta"].(map[string]interface{})
		for _, toolCall := range asMapSlice(delta["tool_calls"]) {
			function, _ := toolCall["function"].(map[string]interface{})
			if name, ok := function["name"].(string); ok && name != "" {
				processed.ToolName = name
			}
		}
	}
}

func extractProcessedTextParts(eventType string, eventData map[string]interface{}) []string {
	var parts []string

	if delta, ok := eventData["delta"].(map[string]interface{}); ok {
		appendStringField(&parts, delta, "text")
		appendStringField(&parts, delta, "content")
		appendStringField(&parts, delta, "partial_json")
	}

	// OpenAI Responses API emits string deltas for output text, reasoning
	// summaries, and function-call arguments.
	streamEventType := eventType
	if typeValue, ok := eventData["type"].(string); ok && typeValue != "" {
		streamEventType = typeValue
	}
	if delta, ok := eventData["delta"].(string); ok && delta != "" && strings.Contains(streamEventType, ".delta") {
		parts = append(parts, delta)
	}

	// OpenAI Chat Completions-compatible streams emit data-only JSON with
	// choices[].delta.content and choices[].delta.tool_calls[].function.arguments.
	for _, choice := range asMapSlice(eventData["choices"]) {
		if delta, ok := choice["delta"].(map[string]interface{}); ok {
			appendStringField(&parts, delta, "content")
			appendStringField(&parts, delta, "reasoning_content")
			for _, toolCall := range asMapSlice(delta["tool_calls"]) {
				function, _ := toolCall["function"].(map[string]interface{})
				appendStringField(&parts, function, "arguments")
			}
		}
		if message, ok := choice["message"].(map[string]interface{}); ok {
			appendStringField(&parts, message, "content")
		}
	}

	// Gemini-style streams may send candidates[].content.parts[].text.
	for _, candidate := range asMapSlice(eventData["candidates"]) {
		content, _ := candidate["content"].(map[string]interface{})
		for _, part := range asMapSlice(content["parts"]) {
			appendStringField(&parts, part, "text")
		}
	}

	return parts
}

func appendStringField(parts *[]string, values map[string]interface{}, key string) {
	if values == nil {
		return
	}
	if value, ok := values[key].(string); ok && value != "" {
		*parts = append(*parts, value)
	}
}

func asMapSlice(value interface{}) []map[string]interface{} {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}

	maps := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if itemMap, ok := item.(map[string]interface{}); ok {
			maps = append(maps, itemMap)
		}
	}
	return maps
}

func scanRequestLog(row interface {
	Scan(dest ...interface{}) error
}) (DatabaseRequestLog, error) {
	var log DatabaseRequestLog
	var headers, body, response, errorMsg sql.NullString
	err := row.Scan(
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
		return log, err
	}
	log.Headers = headers.String
	log.Body = body.String
	log.Response = response.String
	log.Error = errorMsg.String
	return log, nil
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
		log, err := scanRequestLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, rows.Err()
}

func (dl *DatabaseLogger) GetLogByUUID(requestUUID string) (*DatabaseRequestLog, error) {
	query := `
	SELECT id, request_uuid, timestamp, method, url, headers, body, response_code, response, is_stream, error, duration_ms
	FROM request_logs
	WHERE request_uuid = ?
	ORDER BY CASE WHEN response IS NOT NULL AND response != '' THEN 0 ELSE 1 END, id DESC
	LIMIT 1
	`

	log, err := scanRequestLog(dl.db.QueryRow(query, requestUUID))
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (dl *DatabaseLogger) GetSSEEvents(requestUUID string) ([]SSEEvent, error) {
	query := `
	SELECT id, request_uuid, timestamp, event_type, data, sequence
	FROM sse_events
	WHERE request_uuid = ?
	ORDER BY sequence ASC
	`

	rows, err := dl.db.Query(query, requestUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []SSEEvent
	for rows.Next() {
		var event SSEEvent
		var data sql.NullString
		if err := rows.Scan(&event.ID, &event.RequestUUID, &event.Timestamp, &event.EventType, &data, &event.Sequence); err != nil {
			return nil, err
		}
		event.Data = data.String
		events = append(events, event)
	}
	return events, rows.Err()
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
		log, err := scanRequestLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, rows.Err()
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
