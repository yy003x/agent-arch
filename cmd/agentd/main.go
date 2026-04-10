package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/young/go/agent-arch/internal/agent"
	"github.com/young/go/agent-arch/internal/api"
	"github.com/young/go/agent-arch/internal/contextx"
	"github.com/young/go/agent-arch/internal/history"
	"github.com/young/go/agent-arch/internal/llm"
	"github.com/young/go/agent-arch/internal/repo"
)

func main() {
	logger := log.New(os.Stdout, "agentd ", log.LstdFlags|log.Lmicroseconds)
	store := repo.NewMemoryAgentRepo()
	manager := contextx.NewManager(contextx.ApproximateEstimator{})
	historyStore := history.NewFileStore(".cache/history")
	client := llm.NewMockClient(2 * time.Second)

	if os.Getenv("ANTHROPIC_BASE_URL") != "" && os.Getenv("ANTHROPIC_AUTH_TOKEN") != "" {
		client = nil
	}

	var runtimeClient agent.LLMClient
	if client != nil {
		runtimeClient = client
	} else {
		runtimeClient = llm.NewAnthropicClient(llm.AnthropicConfig{
			BaseURL:   os.Getenv("ANTHROPIC_BASE_URL"),
			AuthToken: os.Getenv("ANTHROPIC_AUTH_TOKEN"),
			Model:     os.Getenv("ANTHROPIC_MODEL"),
			Timeout:   10 * time.Second,
		})
	}

	engine := agent.NewEngine(store, runtimeClient, manager, historyStore, logger, agent.RuntimeConfig{
		MaxRounds:   3,
		TokenBudget: 2048,
		LLMTimeout:  3 * time.Second,
	})
	service := api.NewService(engine)
	server := api.NewHTTPServer(service)

	if os.Getenv("BOOTSTRAP_DEMO_RUN") == "1" {
		if _, err := service.Start(context.Background(), api.StartRequest{
			RunID:     "demo-run",
			MaxRounds: 3,
			Context: agent.AgentContext{
				SystemInstructions: []agent.Message{
					{Role: "system", Content: "You are a careful agent runtime demo.", Pinned: true},
				},
				Messages: []agent.Message{
					{Role: "user", Content: "Summarize current progress."},
				},
			},
		}); err != nil {
			logger.Fatalf("bootstrap demo run: %v", err)
		}
	}

	addr := os.Getenv("AGENTD_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	logger.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		logger.Fatalf("listen and serve: %v", err)
	}
}
