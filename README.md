# agent-arch

A Go-based persona-driven single-agent runtime.

agent-arch/
├── AGENTS.md
├── README.md
├── go.mod
├── cmd/
│   └── server/
│       └── main.go
├── configs/
│   ├── config.yaml
│   └── personas/
│       ├── default.yaml
│       └── coder.yaml
├── internal/
│   ├── agent/
│   │   ├── assembler.go
│   │   ├── factory.go
│   │   ├── runtime.go
│   │   ├── service.go
│   │   └── types.go
│   ├── config/
│   │   └── config.go
│   ├── llm/
│   │   ├── client.go
│   │   ├── types.go
│   │   ├── openai/
│   │   │   └── client.go
│   │   └── anthropic/
│   │       └── client.go
│   ├── memory/
│   │   ├── manager.go
│   │   ├── policy.go
│   │   ├── retriever.go
│   │   ├── shortterm.go
│   │   ├── store.go
│   │   └── summary.go
│   ├── persona/
│   │   ├── loader.go
│   │   ├── model.go
│   │   └── renderer.go
│   ├── token/
│   │   └── counter.go
│   └── transport/
│       ├── dto.go
│       └── http.go
└── test/
    ├── adapter_test.go
    ├── memory_test.go
    └── persona_test.go

## Features
- Create agent from persona profile
- Unified OpenAI / Anthropic adapter
- Context memory management up to 128K token budget
- Short-term + rolling summary + retrieval stub
- Config-driven provider/model switching

## Current Scope
This repository implements an MVP only:
- single agent
- no tools
- no MCP
- no workflow engine
- no vector DB

## Architecture
- `internal/persona`: persona loading and rendering
- `internal/llm`: unified client abstraction and provider adapters
- `internal/memory`: short-term memory, rolling summary, context assembly
- `internal/agent`: runtime orchestration
- `internal/transport`: minimal HTTP API

## APIs
- `POST /v1/agents`
- `POST /v1/chat`
- `GET /v1/sessions/{session_id}/memory`

## Run
```bash
go run ./cmd/server