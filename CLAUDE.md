# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go-based LLM API reverse proxy that forwards requests to target LLM APIs (like OpenAI) while providing comprehensive logging capabilities. Supports both file-based and SQLite database logging with streaming/non-streaming response handling.

## Architecture

- **Entry Point**: `main.go` - Configures and starts the proxy server
- **Configuration**: `config/config.go` - JSON/YAML config file and environment variable support
- **Proxy Logic**: `proxy/handler.go` and `proxy/handler_db.go` - HTTP request forwarding with logging
- **Logging**: 
  - `proxy/logger.go` - File-based JSON logging
  - `proxy/db_logger.go` - SQLite database logging with query capabilities

## Key Features

- **Dual Logging Modes**: File-based JSON logs vs SQLite database storage (configurable via `UseDB`)
- **Streaming Support**: Handles SSE (Server-Sent Events) for LLM streaming responses
- **Request/Response Logging**: Complete HTTP transaction logging including headers, body, and timing
- **Configuration**: Supports JSON/YAML config files plus environment variable overrides

## Development Commands

### Build & Run
```bash
# Build binary
go build -o llm_proxy

# Run with file logging (default)
go run main.go

# Run with database logging
USE_DB=true go run main.go

# Run with custom config
CONFIG_FILE=config.json go run main.go
```

### Environment Variables
- `SERVER_PORT`: Listen port (default: `:8080`)
- `TARGET_URL`: Target API URL (default: `https://api.aicodewith.com`)
- `LOG_DIR`: Log directory (default: `./logs`)
- `DB_PATH`: SQLite database path (default: `./logs/proxy.db`)
- `USE_DB`: Enable database logging (default: `false`)
- `CONFIG_FILE`: Config file path (default: `config.json`)

### Configuration Files
Create `config.json` or `config.yaml`:
```json
{
  "server_port": ":8080",
  "target_url": "https://api.openai.com",
  "log_dir": "./logs",
  "db_path": "./logs/proxy.db",
  "use_db": false
}
```

### Testing
```bash
# Test non-streaming
curl http://localhost:8080/v1/models -H "Authorization: Bearer YOUR_KEY"

# Test streaming
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_KEY" \
  -d '{"model": "gpt-3.5-turbo", "messages": [{"role": "user", "content": "Hello"}], "stream": true}'
```

## Dependencies

- Go 1.21+
- SQLite3 (for database logging mode)
- No external Go dependencies (uses standard library only)

## Database Schema

When `UseDB=true`, creates SQLite table:
```sql
CREATE TABLE request_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp DATETIME,
  method TEXT,
  url TEXT,
  headers TEXT,
  body TEXT,
  response_code INTEGER,
  response TEXT,
  is_stream BOOLEAN,
  error TEXT,
  duration_ms INTEGER
);
```

## File Structure

```
llm_reverse/
├── main.go              # Server entry point
├── config/
│   └── config.go        # Configuration management
├── proxy/
│   ├── handler.go       # File logging proxy handler
│   ├── handler_db.go    # Database logging proxy handler
│   ├── logger.go        # File-based logging
│   └── db_logger.go     # SQLite database logging
├── logs/                # Default log directory
├── config.json          # Configuration file
└── go.mod              # Go module definition
```