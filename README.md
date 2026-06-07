# LLM Reverse Proxy

一个用 Go 编写的 LLM API 代理服务，支持请求转发、日志记录以及流式/非流式响应处理。

## 功能特性

- 🚀 支持流式（SSE）和非流式响应
- 📝 完整的请求/响应日志记录
- 🔄 自动请求转发到目标 API
- 🛡️ 保留原始请求头和响应头
- 📊 JSON 格式的结构化日志
- 🖥️ 内置观察面板：查看代理请求、SSE 事件、工具调用和响应时间

## 快速开始

### 编译运行

```bash
go build -o llm_proxy
./llm_proxy
```

### 配置文件

默认启动时，程序会按以下顺序读取配置文件：

1. `config.yaml`
2. `config.json`

如果设置了 `CONFIG_FILE`，则只读取该环境变量指定的文件。若默认配置文件都不存在，或 `CONFIG_FILE` 指定的文件不存在，程序会报告错误并退出。

可以从示例文件创建配置：

```bash
cp config.yaml.example config.yaml
# 或者
cp config.json.example config.json
```

### 环境变量配置

- `CONFIG_FILE`: 显式指定配置文件路径；未设置时优先读取 `config.yaml`，再读取 `config.json`
- `SERVER_PORT`: 代理服务监听端口 (默认: `:8080`)
- `TARGET_URL`: 目标 API 地址 (默认: `https://api.aicodewith.com`)
- `LOG_DIR`: 日志存储目录 (默认: `./logs`)
- `DB_PATH`: SQLite 数据库路径 (默认: `./logs/proxy.db`)
- `USE_DB`: 是否启用数据库日志 (`true`/`1` 开启，`false`/`0` 关闭；大小写不敏感，默认: `false`)

### 使用示例

```bash
# 设置环境变量
export SERVER_PORT=:8080
export TARGET_URL=https://api.openai.com
export LOG_DIR=./logs

# 运行代理
go run main.go
```

### 观察面板

数据库日志模式下启动后打开：

```bash
USE_DB=true go run main.go
# 浏览器访问 http://localhost:8080/_ui/
```

面板提供：
- 请求列表、状态码、耗时、stream/http 标记
- 请求体、响应体、请求头详情
- SSE event timeline，方便观察 agent 流式输出和 tool/harness 调用
- 总请求数、今日请求数、错误数、平均延迟统计


#### 非流式请求
```bash
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

#### 流式请求
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

## 日志格式

日志文件保存在配置的日志目录中，启动时会创建新的日志文件，文件名通常包含启动时间戳，避免复用或追加到旧文件。

每个日志条目包含：
- `timestamp`: 请求时间
- `method`: HTTP 方法
- `url`: 请求 URL
- `headers`: 请求头
- `body`: 请求体
- `response_code`: 响应状态码
- `response`: 响应内容
- `is_stream`: 是否为流式响应
- `error`: 错误信息（如有）

## 项目结构

```
llm_reverse/
├── main.go           # 主程序入口
├── config/
│   └── config.go     # 配置管理
├── proxy/
│   ├── handler.go    # 代理请求处理
│   └── logger.go     # 日志记录
├── logs/             # 日志文件目录（本地生成，git 忽略）
├── config.json.example  # JSON 配置示例
├── config.yaml.example  # YAML 配置示例
├── go.mod            # Go 模块文件
└── README.md         # 本文件
```