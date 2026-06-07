# Intimate Relationship Agent Runtime

一个最小可用的人机亲密关系 Agent Runtime，能够与用户持续互动、记住用户信息、形成结构化关系记忆，并基于记忆调整回复策略。

## 功能特性

- **多轮关系型对话**：支持连续对话，后续回复体现对前文信息的利用
- **结构化记忆机制**：从对话中提取并维护用户信息（基本信息、偏好、情绪、事件等）
- **记忆更新与冲突处理**：自动检测并处理信息冲突（如用户搬家、改名等）
- **关系状态追踪**：基于互动计算熟悉度、信任度、亲密度三维指标
- **执行过程可解释**：每轮对话输出完整执行轨迹

## 快速开始

### 1. 环境准备

- Go 1.22+
- LLM API Key（支持 OpenAI 兼容接口，推荐 Qwen）

### 2. 配置环境变量

项目使用 `.env` 文件管理配置，不硬编码任何密钥。

```bash
# 复制模板并填入你的配置
cp .env.example .env
```

编辑 `.env` 文件：

```ini
# 必填：你的 LLM API Key 和接口信息
LLM_API_KEY=your-api-key-here
LLM_BASE_URL=https://your-llm-api-base-url/v1   # 任何 OpenAI 兼容接口均可
LLM_MODEL=your-model-name                        # 如 qwen-plus, gpt-4o 等

# 可选（以下为默认值）
SERVER_ADDR=:8080
DB_PATH=data/agent.db
```

> **说明**：
> - 系统使用 OpenAI 兼容协议，支持 Qwen、OpenAI、DeepSeek 等任意兼容接口
> - `.env` 已在 `.gitignore` 中，不会被提交到 Git
> - 环境变量优先级：系统环境变量 > `.env` 文件

### 3. 运行服务

```bash
# 下载依赖
go mod tidy

# 启动服务
go run cmd/server/main.go
```

服务启动后会显示：
```
Intimate Agent Runtime starting on :8080
LLM Model: <your-model-name>
Database: data/agent.db

Endpoints:
  POST http://localhost:8080/chat      - Send a message
  GET  http://localhost:8080/status/:id - View user status
  GET  http://localhost:8080/health     - Health check
```

### 4. API 调用示例

#### 发送消息

```bash
# 第一轮对话
curl -s -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user1", "message": "你好，我叫小明，在上海做程序员"}' | python3 -m json.tool --no-ensure-ascii

# 第二轮对话（系统会记住你的信息）
curl -s -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user1", "message": "最近加班很多，感觉有点累"}' | python3 -m json.tool --no-ensure-ascii

# 第三轮对话
curl -s -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user1", "message": "我喜欢打篮球和喝咖啡"}' | python3 -m json.tool --no-ensure-ascii
```

#### 查看用户状态

```bash
curl -s http://localhost:8080/status/user1 | python3 -m json.tool --no-ensure-ascii
```

返回内容包括：
- 用户基本信息
- 所有记忆条目
- 关系状态（熟悉度、信任度、亲密度）

### 5. 运行测试

```bash
go test ./tests/ -v
```

## 项目结构

```
intimate-agent/
├── cmd/server/main.go              # 入口，HTTP 服务启动
├── internal/
│   ├── agent/runtime.go            # 核心 Agent Runtime 编排器
│   ├── memory/
│   │   ├── store.go                # SQLite 记忆存储
│   │   └── conflict.go             # 冲突检测与解决
│   ├── tools/
│   │   ├── tool.go                 # Tool 接口定义
│   │   ├── memory_tool.go          # 记忆读写工具
│   │   └── extraction_tool.go      # 信息提取工具
│   ├── llm/
│   │   └── client.go               # LLM API 客户端
│   ├── session/
│   │   ├── session.go              # 会话状态管理
│   │   └── relationship.go         # 关系指标计算
│   ├── handler/
│   │   └── http.go                 # HTTP 处理器
│   └── models/
│       └── types.go                # 核心数据结构
├── config/config.go                # 配置管理
├── data/                           # SQLite 数据库（运行时创建）
├── tests/
│   └── agent_test.go               # 测试用例
├── .env                            # 环境变量配置（本地，不提交）
├── .env.example                    # 环境变量模板
├── .gitignore                      # Git 忽略规则
├── go.mod
└── go.sum
```

## 核心数据结构

### UserProfile
```go
type UserProfile struct {
    UserID     string
    Name       string
    Age        int
    Occupation string
    City       string
}
```

### MemoryItem
```go
type MemoryItem struct {
    ID           string
    UserID       string
    Category     string    // "basic_info", "preference", "emotion", "event", "relationship_pref"
    Key          string
    Value        string
    Source       string    // 原始用户发言
    Confidence   float64
    CreatedAt    time.Time
    UpdatedAt    time.Time
    SupersededBy string    // 被哪条新记忆替代
    Active       bool
}
```

### RelationshipState
```go
type RelationshipState struct {
    Familiarity float64  // 0-100，熟悉度
    Trust       float64  // 0-100，信任度
    Intimacy    float64  // 0-100，亲密度
}
```

### SessionState
```go
type SessionState struct {
    SessionID    string
    UserID       string
    Messages     []Message
    Relationship RelationshipState
    TurnCount    int
}
```

## 执行流程

每轮对话的执行流程：

1. **Step1: 读取记忆** - 从 SQLite 加载用户现有记忆
2. **Step2: 信息提取** - 调用 LLM 从用户消息中提取结构化信息
3. **Step3: 冲突解决** - 检测并处理新旧信息冲突
4. **Step4: 关系更新** - 根据互动更新关系指标
5. **Step5: 生成回复** - 结合记忆和关系状态生成回复

## 测试案例

### 1. 正常建立关系
```go
// 用户连续提供信息，系统逐步建立记忆
Turn 1: "你好，我叫小明，在上海做程序员"
Turn 2: "最近加班很多，感觉有点累"
Turn 3: "我喜欢打篮球和喝咖啡"
// 验证：记忆正确提取，关系指标递增
```

### 2. 记忆冲突处理
```go
// 用户更新信息，系统正确处理冲突
Turn 1: "我在北京工作"
Turn 2: "其实我上个月搬到深圳了"
// 验证：城市记忆更新，旧记忆被标记为 superseded
```

### 3. 错误处理
```go
// LLM 调用失败时的降级处理
// 验证：系统仍能正常回复，使用 fallback 响应
```

## 配置说明

所有配置通过 `.env` 文件管理（首次使用请 `cp .env.example .env`）。

| 配置项 | 必填 | 默认值 | 说明 |
|--------|------|--------|------|
| LLM_API_KEY | ✅ | - | LLM API 密钥 |
| LLM_BASE_URL | ✅ | - | OpenAI 兼容接口地址 |
| LLM_MODEL | ✅ | - | 使用的模型名称 |
| SERVER_ADDR | | :8080 | 服务监听地址 |
| DB_PATH | | data/agent.db | SQLite 数据库路径 |

> 优先级：系统环境变量 > `.env` 文件

## 扩展性

- **工具系统**：通过实现 `Tool` 接口可轻松添加新工具
- **存储后端**：可替换 SQLite 为 PostgreSQL、Redis 等
- **LLM 提供商**：支持任何 OpenAI 兼容的 API
- **关系模型**：可在 `relationship.go` 中调整指标计算逻辑

## 注意事项

- 首次运行会自动创建 `data/` 目录和数据库文件
- 建议使用 Qwen 系列模型以获得最佳中文效果
- 所有记忆数据存储在本地 SQLite，无外部依赖
- 测试使用 Mock LLM，无需 API Key

## 系统反思与扩展分析

### 1. 系统最容易出错的地方

- **LLM 抽取的准确性**：`ExtractionTool` 完全依赖 LLM 返回结构化 JSON。虽然代码中有 `strings.Index("{")` 的容错截取，但当 LLM 返回嵌套 JSON、注释、或多段文本时，解析仍可能失败。目前的 fallback 是返回空列表（不崩溃），但会丢失该轮的信息提取。
- **冲突解决的误判**：`ConflictResolver` 将新旧信息交给 LLM 判断是否构成冲突。对于语义相似但含义不同的表述（如"我喜欢咖啡" vs "我最近开始喝咖啡了"），LLM 可能误判为"非冲突"而忽略用户的偏好变化。
- **会话状态丢失**：`session.Manager` 仅在内存中维护对话历史和关系状态，服务器重启后归零。长期记忆（SQLite）不受影响，但用户重启后会体验到"AI 失忆"——关系等级从 `stranger` 重新开始，丢失对话上下文。

### 2. 用户量扩大到 10 万+ 的瓶颈

- **SQLite 单写锁**：SQLite 采用单写多读模型，高并发写入时会产生锁等待，成为吞吐量瓶颈。
- **内存占用**：每个用户对应一个 `SessionState`（含完整消息历史），10 万用户 × 平均 50 条消息，内存消耗显著。
- **LLM 调用延迟**：每轮对话涉及 2-3 次 LLM 调用（信息抽取 + 冲突判断 + 回复生成），10 万并发场景下 LLM API 的延迟和配额将成为核心瓶颈。

### 3. 优化方向

| 层面 | 优化措施 |
|------|---------|
| **存储** | 将 SQLite 替换为 PostgreSQL 支持高并发写入；会话状态持久化到 Redis，避免内存溢出 |
| **LLM 调用** | 合并"信息抽取"和"冲突判断"为单次 LLM 调用，将每轮对话的 LLM 请求从 3 次降为 2 次；非关键调用（如冲突判断）异步化 |
| **记忆检索** | 引入 embedding 向量索引（如 pgvector），支持语义检索而非全量加载，减少 prompt 长度 |
| **削峰** | 引入消息队列（如 Kafka/NATS），将 LLM 调用从同步改为异步，Worker 池化处理 |
| **部署** | 多实例水平部署 + 共享存储层，Session 通过 Redis 共享，实现无状态服务节点 |
