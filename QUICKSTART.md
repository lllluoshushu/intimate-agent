# Intimate Agent 从零开始运行指南

## 前提条件

- Linux 系统（已测试 Slurm 服务器）
- 已安装 Go 1.22.5（如未安装，见下方说明）

---

## 第一步：安装 Go（如已安装可跳过）

```bash
# 1. 下载 Go 1.22.5
cd ~
wget https://go.dev/dl/go1.22.5.linux-amd64.tar.gz

# 2. 解压到 ~/sdk/go
mkdir -p ~/sdk
tar -C ~/sdk -xzf go1.22.5.linux-amd64.tar.gz

# 3. 添加到 PATH（永久生效）
echo 'export PATH=$HOME/sdk/go/bin:$PATH' >> ~/.bashrc
source ~/.bashrc

# 4. 验证安装
go version
# 输出: go version go1.22.5 linux/amd64
```

---

## 第二步：进入项目目录

```bash
cd /path/to/intimate-agent
```

---

## 第三步：配置模型

```bash
# 复制配置模板
cp .env.example .env

# 编辑 .env 文件，填入你的 LLM 配置
vi .env
```

`.env` 文件内容示例：
```ini
LLM_API_KEY=your-api-key-here
LLM_BASE_URL=https://your-llm-api-base-url/v1
LLM_MODEL=your-model-name
SERVER_ADDR=:8080
DB_PATH=data/agent.db
```

> `.env` 文件已在 `.gitignore` 中，不会被提交到 Git。

---

## 第四步：下载依赖

```bash
# 设置 Go 代理（国内加速）
export GOPROXY=https://goproxy.cn,direct

# 下载依赖
go mod tidy
```

---

## 第五步：编译程序

```bash
go build -o intimate-agent ./cmd/server/
```

---

## 第六步：运行程序

```bash
# 方式一：直接运行编译后的程序
./intimate-agent

# 方式二：使用 go run（每次都会重新编译）
go run cmd/server/main.go
```

程序启动后会显示：
```
Intimate Agent Runtime starting on :9090
LLM Model: your-model-name
Database: data/agent.db

Endpoints:
  POST http://localhost:9090/chat      - Send a message
  GET  http://localhost:9090/status/:id - View user status
  GET  http://localhost:9090/health     - Health check
```

---

## 第七步：测试程序

打开**新的终端窗口**，执行以下命令：

### 1. 健康检查

```bash
curl http://localhost:9090/health
# 输出: {"status":"ok"}
```

### 2. 发送消息

```bash
curl -s -X POST http://localhost:9090/chat \
  -H "Content-Type: application/json" \
  -d '{"user_id": "test_user", "message": "你好，我叫小明"}' | python3 -m json.tool --no-ensure-ascii
```

### 3. 查看用户状态

```bash
curl -s http://localhost:9090/status/test_user | python3 -m json.tool --no-ensure-ascii
```

---

## 完整一键执行脚本

项目已提供 `start.sh`，直接运行即可（会自动从 `.env` 读取配置）：

```bash
chmod +x start.sh
./start.sh
```

---

## 在后台运行（推荐）

如果想让程序在后台持续运行：

```bash
# 使用 nohup 后台运行
nohup ./intimate-agent > server.log 2>&1 &

# 查看日志
tail -f server.log

# 停止程序
pkill -f intimate-agent
```

---

## 常见问题

### 1. 端口被占用

```bash
# 查看占用端口的进程
ss -tlnp | grep 9090

# 杀掉占用端口的进程
kill <PID>

# 或者使用其他端口
export SERVER_ADDR=":9091"
./intimate-agent
```

### 2. API Key 无效

检查 `.env` 文件中的配置是否正确，可以直接测试你的 LLM 接口：
```bash
# 加载 .env 配置
source .env

curl -X POST "$LLM_BASE_URL/chat/completions" \
  -H "Authorization: Bearer $LLM_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"model\": \"$LLM_MODEL\", \"messages\": [{\"role\": \"user\", \"content\": \"hello\"}], \"max_completion_tokens\": 100}"
```

### 3. Go 命令找不到

```bash
# 确认 Go 已安装
ls ~/sdk/go/bin/go

# 添加到 PATH
export PATH=$HOME/sdk/go/bin:$PATH
```

---

## API 使用示例

### Python 示例

```python
import requests

# 发送消息
response = requests.post(
    "http://localhost:9090/chat",
    json={
        "user_id": "python_user",
        "message": "你好，我叫小红，我今天心情很好"
    }
)
print(response.json())

# 查看状态
status = requests.get("http://localhost:9090/status/python_user")
print(status.json())
```

### JavaScript 示例

```javascript
// 发送消息
const response = await fetch('http://localhost:9090/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
        user_id: 'js_user',
        message: '你好，我叫小明'
    })
});
const data = await response.json();
console.log(data);
```

---

## 项目结构

```
intimate-agent/
├── cmd/server/main.go      # 程序入口
├── config/config.go        # 配置管理
├── internal/
│   ├── models/types.go     # 数据结构
│   ├── llm/client.go       # LLM 客户端
│   ├── memory/             # 记忆存储和冲突解决
│   ├── tools/              # 工具系统
│   ├── session/            # 会话和关系管理
│   ├── agent/runtime.go    # 核心运行时
│   └── handler/http.go     # HTTP 处理器
├── data/                   # SQLite 数据库目录
├── tests/                  # 测试文件
├── go.mod                  # Go 模块文件
└── README.md               # 项目说明
```

---

## 停止程序

```bash
# 方式一：Ctrl+C（如果在前台运行）

# 方式二：杀掉进程
pkill -f intimate-agent
```
