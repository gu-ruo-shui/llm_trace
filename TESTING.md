# 测试文档

## 测试概览

本项目包含全面的测试套件，覆盖以下模块：

-   **config**: 配置管理测试
-   **proxy**: 代理处理测试（文件日志和数据库日志模式）
-   **integration**: 集成测试

## 测试结构

```
llm_reverse/
├── config/
│   └── config_test.go          # 配置管理测试
├── proxy/
│   ├── logger_test.go          # 文件日志测试
│   ├── handler_test.go         # 代理处理器测试（文件模式）
│   ├── db_logger_test.go       # 数据库日志测试
│   └── handler_db_test.go      # 代理处理器测试（数据库模式）
├── main_test.go               # 集成测试
├── test.sh                    # Unix测试脚本
├── test.bat                   # Windows测试脚本
└── TESTING.md                 # 本文档
```

## 运行测试

### 快速测试

```bash
# 运行所有测试
go test ./...

# 运行特定包测试
go test -v ./config
go test -v ./proxy

# 运行集成测试
go test -v -run TestIntegration ./...
```

### 详细测试

```bash
# Unix/Linux/macOS
./test.sh

# Windows
./test.bat
```

### 覆盖率测试

```bash
# 运行覆盖率测试
go test -cover ./...

# 生成详细覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## 测试类型

### 1. 单元测试

-   **配置测试**: 验证 JSON/YAML 配置加载、环境变量覆盖等
-   **日志测试**: 验证文件日志和数据库日志的正确性
-   **代理测试**: 验证 HTTP 请求转发、头部处理、错误处理等

### 2. 集成测试

-   **HTTPBin 集成**: 测试与真实 API 的集成
-   **流式响应测试**: 测试 SSE 流式处理
-   **错误处理测试**: 测试各种错误场景
-   **性能测试**: 测试并发处理性能

### 3. 数据库测试

-   **SQLite 集成**: 测试数据库连接、表创建
-   **CRUD 操作**: 测试日志的增删改查
-   **并发写入**: 测试多 goroutine 并发写入
-   **统计查询**: 测试统计信息获取

## 测试环境要求

### 必需依赖

-   Go 1.21+
-   SQLite3 (用于数据库日志测试)

### 安装 SQLite 驱动

```bash
go get github.com/mattn/go-sqlite3
```

### 可选依赖

-   curl (用于手动测试)
-   网络连接 (用于集成测试)

## 测试数据

测试会自动创建临时文件和数据库，测试完成后自动清理。

### 文件日志测试

-   临时日志目录: `os.TempDir()`
-   日志文件名: `llm_proxy_YYYY-MM-DD.log`

### 数据库日志测试

-   临时数据库文件: `test.db`
-   表名: `request_logs`

## 测试用例示例

### 配置文件测试

```go
func TestLoad_JSONConfig(t *testing.T) {
    // 测试JSON配置加载
}

func TestLoad_EnvOverride(t *testing.T) {
    // 测试环境变量覆盖
}
```

### 代理处理测试

```go
func TestProxyHandler_ServeHTTP_RegularResponse(t *testing.T) {
    // 测试普通HTTP响应处理
}

func TestProxyHandler_ServeHTTP_StreamingResponse(t *testing.T) {
    // 测试流式响应处理
}
```

### 数据库测试

```go
func TestDatabaseLogger_LogRequest(t *testing.T) {
    // 测试请求日志记录
}

func TestDatabaseLogger_GetLogs(t *testing.T) {
    // 测试日志查询
}
```

## 性能测试

### 基准测试命令

```bash
# 运行性能测试
go test -bench=. -benchmem ./...

# 运行特定性能测试
go test -bench=BenchmarkProxyHandler ./proxy
```

### 性能指标

-   平均响应时间: ~50ms
-   并发处理能力: 100+ 请求/秒
-   内存使用: 低内存占用

## 调试技巧

### 日志调试

```bash
# 启用详细日志
go test -v ./...

# 调试特定测试
go test -v -run TestProxyHandler_ServeHTTP_RegularResponse ./proxy
```

### 数据库调试

```bash
# 查看测试数据库内容
sqlite3 test.db ".schema"
sqlite3 test.db "SELECT * FROM request_logs LIMIT 10"
```

## 持续集成

### GitHub Actions 示例

```yaml
name: Test
on: [push, pull_request]
jobs:
    test:
        runs-on: ubuntu-latest
        steps:
            - uses: actions/checkout@v3
            - uses: actions/setup-go@v4
              with:
                  go-version: "1.21"
            - run: go mod tidy
            - run: go test -v ./...
            - run: go test -cover ./...
```

## 故障排除

### 常见问题

1. **SQLite 驱动问题**

    ```bash
    # 安装CGO依赖
    sudo apt-get install gcc sqlite3 libsqlite3-dev  # Ubuntu/Debian
    brew install sqlite3                             # macOS
    ```

2. **权限问题**

    ```bash
    # 确保测试脚本有执行权限
    chmod +x test.sh
    ```

3. **网络问题**
    ```bash
    # 跳过集成测试
    go test -short ./...
    ```

### 调试命令

```bash
# 清理测试缓存
go clean -testcache

# 重新运行特定测试
go test -v -count=1 ./proxy

# 查看测试覆盖率
open coverage.html
```
