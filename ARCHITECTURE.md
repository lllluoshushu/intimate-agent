# 系统架构说明

## 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                      HTTP API Layer                         │
│            POST /chat | GET /status/:id | GET /health       │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│                   Agent Runtime (Orchestrator)               │
│  ┌───────────┐  ┌───────────┐  ┌────────────────────────┐   │
│  │   Input   │→ │   Tool    │→ │    State Manager       │   │
│  │ Processor │  │ Executor  │  │ (Session + Relationship)│   │
│  └───────────┘  └───────────┘  └────────────────────────┘   │
└───────────────────────────┬─────────────────────────────────┘
                            │
           ┌────────────────┼────────────────┐
           ▼                ▼                ▼
    ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
    │   Memory    │  │    Info     │  │    LLM      │
    │  R/W Tool   │  │ Extraction  │  │   Client    │
    │  (SQLite)   │  │    Tool     │  │  (Qwen API) │
    └─────────────┘  └─────────────┘  └─────────────┘
```

## 核心模块

### 1. Agent Runtime (`internal/agent/runtime.go`)

**职责**：系统的核心编排器，协调所有组件完成对话处理。

**关键特性**：
- 接收用户消息，执行完整的处理流水线
- 管理工具执行顺序和错误处理
- 生成执行轨迹（Execution Trace）
- 支持重试机制（最多 2 次）

**执行流程**：
```go
func (r *Runtime) ProcessMessage(ctx, userID, userMessage) (reply, trace, error) {
    // Step 1: 读取现有记忆
    // Step 2: 提取用户信息（调用 LLM）
    // Step 3: 冲突检测与解决
    // Step 4: 更新关系状态
    // Step 5: 生成回复（调用 LLM）
}
```

### 2. Memory System (`internal/memory/`)

**Store** (`store.go`)：
- SQLite 持久化存储
- 支持 CRUD 操作
- 索引优化（user_id, category, active）

**ConflictResolver** (`conflict.go`)：
- 检测新旧信息冲突
- 三种解决策略：
  - `update`：用新信息替代旧信息
  - `keep_old`：保留旧信息（新信息置信度低）
  - `ask_user`：标记为不确定，保留两条记录
- 使用 LLM 进行语义冲突判断

### 3. Tool System (`internal/tools/`)

**Tool 接口**：
```go
type Tool interface {
    Name() string
    Description() string
    Execute(ctx, input) (output, error)
}
```

**已实现工具**：
- **MemoryTool**：读取用户记忆
- **ExtractionTool**：从消息中提取结构化信息

**扩展方式**：实现 `Tool` 接口并注册到 `Registry`。

### 4. Session Manager (`internal/session/`)

**Manager** (`session.go`)：
- 内存中的会话状态管理
- 线程安全（sync.RWMutex）
- 管理消息历史和关系状态

**RelationshipEngine** (`relationship.go`)：
- 三维关系模型：熟悉度、信任度、亲密度
- 基于信号加权计算：
  - 信息提取 → 熟悉度 +
  - 情感分享 → 信任度 + 亲密度 +
  - 对话轮次 → 整体缓慢增长
- 关系等级：stranger → acquaintance → familiar → close → intimate
- 根据等级调整回复语气

### 5. LLM Client (`internal/llm/client.go`)

**接口设计**：
```go
type Client interface {
    Chat(ctx, messages) (string, error)
}
```

**实现**：
- OpenAI 兼容 API 客户端
- 支持任何兼容接口（OpenAI、Qwen、Claude 等）
- 30 秒超时

**Mock 实现**（测试用）：
- 返回预设响应，无需 API Key
- 支持模拟失败场景

### 6. HTTP Handler (`internal/handler/http.go`)

**端点**：
- `POST /chat`：处理对话请求
- `GET /status/:id`：查看用户状态
- `GET /health`：健康检查

**请求/响应格式**：
```json
// POST /chat
{
  "user_id": "user1",
  "message": "你好，我叫小明"
}

// Response
{
  "reply": "你好！很高兴认识你，小明！",
  "trace": {
    "steps": [
      {"step": 1, "name": "Step1: Read existing memories", "status": "success"},
      ...
    ]
  },
  "session": {
    "relationship": {"familiarity": 15, "trust": 3, "intimacy": 2}
  }
}
```

## 数据流

### 单轮对话数据流

```
用户消息
    │
    ▼
┌─────────────┐
│ Input       │ 解析请求，加载会话
│ Processor   │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Memory R/W  │ 读取用户现有记忆
│ Tool        │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Info        │ 调用 LLM 提取结构化信息
│ Extraction  │ (basic_info, preference, emotion, event, relationship_pref)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Conflict    │ 检测新旧信息冲突
│ Resolver    │ 决定：update / keep_old / ask_user
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Relationship│ 根据互动更新关系指标
│ Engine      │ familiarity + trust + intimacy
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Reply       │ 结合记忆 + 关系状态 + 历史消息
│ Generator   │ 调用 LLM 生成个性化回复
└──────┬──────┘
       │
       ▼
   回复 + 执行轨迹
```

## 错误处理策略

### 1. 工具执行失败
- 最多重试 2 次
- 失败后使用 fallback 响应
- 在执行轨迹中标记状态（success/fallback/error）

### 2. LLM 调用失败
- 信息提取失败：返回空列表，继续对话
- 回复生成失败：使用预设的 fallback 回复
- 冲突判断失败：基于置信度决定

### 3. 数据库错误
- 读取失败：返回空列表
- 写入失败：返回错误，不影响对话

## 性能考量

### 当前实现
- 内存中的会话状态（无持久化）
- SQLite 单机存储
- 同步处理

### 扩展方向
- **会话持久化**：Redis / PostgreSQL
- **记忆分层**：短期记忆（内存）+ 长期记忆（SQLite）
- **异步处理**：消息队列 + Worker
- **分布式**：多实例部署 + 共享存储

## 安全考虑

- API Key 通过环境变量传递，不硬编码
- 用户数据本地存储，不上传
- 无外部依赖（除 LLM API）
- 输入验证（user_id, message 非空）

## 测试策略

### 单元测试
- Mock LLM 客户端
- 内存数据库（`:memory:`）
- 独立测试每个模块

### 集成测试
- 测试完整对话流程
- 测试记忆冲突处理
- 测试错误降级

### 测试覆盖
- 正常流程（关系建立）
- 边界情况（记忆冲突）
- 异常情况（LLM 失败）

## 依赖

- `github.com/mattn/go-sqlite3`：SQLite 驱动
- `github.com/google/uuid`：ID 生成
- Go 标准库：net/http, encoding/json, sync

## 部署建议

### 开发环境
```bash
go run cmd/server/main.go
```

### 生产环境
```bash
# 编译
go build -o intimate-agent cmd/server/main.go

# 运行
./intimate-agent
```

### Docker
```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o server cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
CMD ["./server"]
```

## 未来改进

1. **记忆分层**：区分短期（会话内）和长期（跨会话）记忆
2. **情感分析**：更精细的情绪识别和响应
3. **多模态**：支持图片、语音输入
4. **个性化**：用户可自定义 AI 性格
5. **隐私保护**：记忆加密、用户可控的数据删除
6. **分布式**：支持多用户并发、水平扩展
