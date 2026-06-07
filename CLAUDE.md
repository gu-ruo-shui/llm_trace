# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based LLM API reverse proxy server that forwards requests to LLM endpoints (default: api.aicodewith.com) with comprehensive logging capabilities. It supports both file-based JSON logging and SQLite database logging, handles SSE streaming responses, and provides request/response analytics.

## Essential Commands

### Build
```bash
go build -o llm_proxy          # Unix/Linux/Mac
go build -o llm_reverse.exe    # Windows
```

### Test
```bash
go test ./...                  # Run all tests on Unix/Linux/Mac
go test -v ./...               # Verbose test output on Unix/Linux/Mac
go test -cover ./...           # Run with coverage on Unix/Linux/Mac
go test -run TestName ./...    # Run specific test on Unix/Linux/Mac
./test.sh                      # Full test suite (Unix/Linux/Mac)
```

```powershell
go test ./...                  # Run all tests on Windows; use PowerShell
go test -v ./...               # Verbose test output on Windows
go test -cover ./...           # Run with coverage on Windows
go test -run TestName ./...    # Run specific test on Windows
.\test.bat                     # Full test suite (Windows)
```

### Run
```bash
./llm_proxy                    # Run with default config
CONFIG_FILE=config.yaml.example ./llm_proxy  # Use YAML config
USE_DB=true ./llm_proxy        # Enable database logging
```

## Architecture

### Core Components

1. **Proxy Handlers** (`proxy/` package):
   - `handler.go`: File-based logging proxy - writes JSON logs to daily-rotated files
   - `handler_db.go`: Database logging proxy - stores requests/responses in SQLite with indexing
   - Both handlers preserve headers, handle SSE streaming, and forward requests transparently

2. **Logging System**:
   - `proxy/logger.go`: Thread-safe file logger that creates a new file on each start (timestamped `llm_proxy_YYYY-MM-DD-HHMMSS[-n].log`)
   - `proxy/db_logger.go`: SQLite logger with query capabilities, automatic cleanup, and performance metrics
   - Logs capture: timestamps, URLs, methods, headers, bodies, response times, error states

3. **Configuration** (`config/` package):
   - Supports JSON/YAML files with environment variable overrides
   - Key settings: SERVER_PORT, TARGET_URL, LOG_DIR, DB_PATH, USE_DB
   - Loads `config.yaml` by default, falls back to `config.json`; override with `CONFIG_FILE` env var; commit only `config.json.example` / `config.yaml.example`

### Request Flow

1. Client sends request to proxy server (default :8080)
2. Proxy handler intercepts and logs request details
3. Request forwarded to target LLM API (preserving headers)
4. Response received and logged (handles both regular and SSE streaming)
5. Response forwarded to client with original headers
6. Log entry written to file or database based on configuration

### Database Schema (when USE_DB=true)

```sql
CREATE TABLE logs (
    id INTEGER PRIMARY KEY,
    timestamp TEXT,
    method TEXT,
    url TEXT,
    request_headers TEXT,
    request_body TEXT,
    response_status INTEGER,
    response_headers TEXT,
    response_body TEXT,
    response_time_ms INTEGER,
    error TEXT
);
-- Indexes on timestamp, url, response_status for query performance
```

## Testing Approach

- Unit tests for individual components (config, loggers, handlers)
- Integration tests using httpbin.org for real HTTP scenarios
- Performance tests for concurrent request handling
- Database tests for SQLite operations and concurrent writes
- Mock implementations in `proxy/test_utils.go` for isolated testing

## Key Configuration Options

- `SERVER_PORT`: Proxy listening port (default: `:8080`)
- `TARGET_URL`: Target LLM API endpoint (default: `https://api.aicodewith.com`)
- `LOG_DIR`: Directory for file logs (default: `./logs`)
- `DB_PATH`: SQLite database path (default: `./logs/proxy.db`)
- `USE_DB`: Toggle database vs file logging (default: `false`)
- `LOG_MAX_AGE_DAYS`: Days to retain logs (database mode only)

## Development Notes

- Go 1.21+ required for modern features and SQLite driver
- Uses `github.com/mattn/go-sqlite3` for database functionality
- HTTP transport configured with connection pooling and timeouts for production use
- Graceful shutdown handling with signal interrupts (SIGINT/SIGTERM)
- Thread-safe implementations for concurrent request handling