# Agent Runtime

面向生产场景的 Go Agent Runtime，强调显式状态机、可恢复执行、人机协作修复和清晰的模块边界。

## 能力范围

- 单 run 单 runtime 实例
- 有限且显式的状态机
- 最多多轮执行，可按轮推进
- LLM 超时或上游失败后进入 `waiting_human`
- 支持 `Start`、`Block`、`Stop`、`Cancel`、`Continue`、`PatchContextAndResume`、`GetSnapshot`
- 基于消息的上下文管理
- 保留系统指令并按 token 预算截断上下文
- 事件日志和执行快照持久化抽象
- 当前提供内存仓储和 mock LLM

## 目录

```text
.
├── cmd/
│   └── agentd/
│       └── main.go
├── internal/
│   ├── agent/
│   ├── api/
│   ├── contextx/
│   ├── llm/
│   └── repo/
├── AGENTS.md
├── go.mod
└── README.md
```

## 模块说明

- `internal/agent`: 运行时、状态机、事件、快照、错误和引擎
- `internal/contextx`: 上下文 patch、token 估算和截断策略
- `internal/llm`: LLM 请求/响应模型与 mock 客户端
- `internal/repo`: 仓储实现，当前为内存版
- `internal/api`: 面向控制面的 service 和 DTO
- `cmd/agentd`: 单进程 demo

## 状态机

支持状态：

- `created`
- `running`
- `waiting_llm`
- `waiting_human`
- `blocked`
- `stopped`
- `completed`
- `cancelled`
- `failed`

状态迁移规则编码在 [state.go](/Users/young/go/agent-arch/internal/agent/state.go)。

## 运行模型

- `Engine` 负责创建和定位 run 对应的 `Runtime`
- `Runtime` 持有单个 run 的内存态和串行化控制锁
- LLM 调用期间状态切到 `waiting_llm`
- 调用成功后追加 assistant 消息并推进轮次
- 调用超时或上游失败后切到 `waiting_human`
- 人工补丁通过 `PatchContextAndResume` 写回上下文并恢复执行

## 快速运行

```bash
go run ./cmd/agentd
```

程序会启动一个 demo run，等待 mock LLM 执行完成后输出 JSON 快照。

## 测试

沙箱环境下建议指定可写缓存目录：

```bash
env GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./...
```

## 后续扩展方向

- 增加 MySQL / Redis / Mongo 仓储实现
- 抽出 HTTP 或 gRPC 控制面
- 接入真实 tokenizer
- 接入真实 LLM provider
- 增加更细粒度的事件订阅和运行指标
