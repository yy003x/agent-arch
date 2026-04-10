# AGENTS.md

## 1. Mission

Build a production-oriented Agent Runtime in Go.

The system must support:

- long-running agent execution across multiple rounds
- explicit state management
- human-in-the-loop intervention
- pause / block / stop / continue / cancel controls
- LLM timeout/failure recovery by allowing human context patching and resume
- clean architecture without relying on heavy frameworks

The implementation should prioritize correctness, readability, extensibility, and operational safety.

---

## 2. Product Goal

Implement an agent runtime that supports:

- LLM call latency around 2–5 seconds per request
- context window up to 128K
- up to 10 rounds of execution per run
- persistent agent state
- human intervention when LLM fails or times out
- ability to modify context and retry
- runtime controls:
  - block
  - stop
  - cancel
  - continue
- clear state transitions and resumable execution

Do not build a toy demo only.
Deliver code with production-style boundaries and clear extension points.

---

## 3. Hard Constraints

### 3.1 Language & stack

- Use Go only
- Prefer standard library first
- Do not introduce heavy frameworks
- Do not use Temporal, LangGraph, Kratos, Gin, or any workflow framework unless explicitly asked
- Small helper libraries are acceptable only when clearly necessary

### 3.2 Architecture

Must use a clean modular design.

Required modules:

- `runtime`
- `state machine`
- `llm client abstraction`
- `repository abstraction`
- `context manager`
- `control plane`
- `event log / execution history`

### 3.3 Runtime model

Each agent run should have a clear execution owner.

Preferred model:

- one run = one runtime instance
- state transitions must be serialized
- avoid unsafe concurrent state mutation
- prefer event-loop / actor-like control if suitable

### 3.4 State management

State must be explicit and finite.

At minimum support:

- `created`
- `running`
- `waiting_llm`
- `waiting_human`
- `blocked`
- `stopped`
- `completed`
- `cancelled`
- `failed`

State transition rules must be explicit in code.

### 3.5 Human intervention

When LLM times out or fails:

- do not silently discard the run
- move the run into `waiting_human`
- preserve current context, last error, current round, and execution history
- allow operator to patch context
- allow resume after patch

### 3.6 Control operations

Must support APIs or service methods for:

- `Start`
- `Block`
- `Stop`
- `Cancel`
- `Continue`
- `PatchContextAndResume`
- `GetSnapshot`

### 3.7 Context handling

Context must be managed carefully.

Requirements:

- support message-based context
- preserve system instructions
- support append and replace patch operations
- include token-budget-aware truncation strategy
- do not blindly send the whole history forever
- keep recent messages and pinned instructions preferentially

For now, token estimation can be approximate, but code must be designed so real tokenizer integration can be added later.

### 3.8 Persistence

Do not hardcode everything in memory.

Required:

- define repository interfaces first
- provide at least an in-memory implementation
- design code so MySQL / Redis / Mongo can be plugged in later

### 3.9 Failure handling

Must distinguish:

- timeout
- cancellation
- upstream LLM error
- invalid state transition
- repository persistence failure

Do not swallow errors.

### 3.10 Code quality

All code must be:

- idiomatic Go
- small cohesive files
- clear naming
- minimal but meaningful comments
- no over-abstraction
- no speculative generic frameworks

---

## 4. Coding Style

Follow these rules strictly.

### 4.1 General

- Favor simple structures over clever abstractions
- Prefer explicit code over magic
- Keep functions focused
- Avoid giant files when code grows
- Avoid deeply nested logic where possible

### 4.2 Interfaces

Define interfaces at the consumer side, not the producer side.

Examples:

- `LLMClient`
- `AgentRepo`

Do not create interfaces for everything.

### 4.3 Context usage

- Pass `context.Context` explicitly where needed
- Never store request-scoped context in struct fields permanently
- Timeouts for LLM calls must use derived contexts

### 4.4 Concurrency

- Be conservative with goroutines
- Concurrency must not break state consistency
- State transitions should happen in one serialized path
- If using channels, document ownership clearly

### 4.5 Logging

Use simple structured logging abstraction or standard log package.

Log important events:

- run start
- llm request start/end
- timeout/error
- state transition
- human patch
- stop/block/cancel/continue

### 4.6 Errors

- Wrap errors with context
- Return actionable errors
- Use sentinel errors only when useful
- Invalid transitions must be explicit

---

## 5. Directory Layout

Use this layout unless there is a strong reason not to:

```text
.
├── AGENTS.md
├── cmd/
│   └── agentd/
│       └── main.go
├── internal/
│   ├── agent/
│   │   ├── engine.go
│   │   ├── runtime.go
│   │   ├── state.go
│   │   ├── command.go
│   │   ├── snapshot.go
│   │   ├── event.go
│   │   └── errors.go
│   ├── contextx/
│   │   ├── manager.go
│   │   ├── truncate.go
│   │   └── token_estimator.go
│   ├── llm/
│   │   ├── client.go
│   │   └── mock.go
│   ├── repo/
│   │   ├── repo.go
│   │   └── memory.go
│   └── api/
│       ├── service.go
│       └── dto.go
├── go.mod
└── README.md