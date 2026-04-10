package agent_test

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/young/go/agent-arch/internal/agent"
	"github.com/young/go/agent-arch/internal/contextx"
	"github.com/young/go/agent-arch/internal/history"
	"github.com/young/go/agent-arch/internal/llm"
	"github.com/young/go/agent-arch/internal/repo"
)

func TestAnthropicAgentMultiRound(t *testing.T) {
	if os.Getenv("RUN_LIVE_ANTHROPIC_TEST") != "1" {
		t.Skip("set RUN_LIVE_ANTHROPIC_TEST=1 to run the live Anthropic integration test")
	}

	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	authToken := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	if baseURL == "" || authToken == "" {
		t.Skip("ANTHROPIC_BASE_URL or ANTHROPIC_AUTH_TOKEN is not set")
	}

	store := repo.NewMemoryAgentRepo()
	manager := contextx.NewManager(contextx.ApproximateEstimator{})
	historyDir := filepath.Join(t.TempDir(), "history")
	historyStore := history.NewFileStore(historyDir)
	client := llm.NewAnthropicClient(llm.AnthropicConfig{
		BaseURL:   baseURL,
		AuthToken: authToken,
		Model:     os.Getenv("ANTHROPIC_MODEL"),
		Timeout:   60 * time.Second,
	})
	logger := log.New(io.Discard, "", 0)

	engine := agent.NewEngine(store, client, manager, historyStore, logger, agent.RuntimeConfig{
		MaxRounds:   4,
		TokenBudget: 12000,
		LLMTimeout:  45 * time.Second,
	})

	runID := "anthropic-live-run"
	firstQuestion := "第一轮问题：请直接回答字符串 FIRST_ROUND_OK，然后用一句中文说明你已经开始执行这个 agent run。"
	_, err := engine.CreateRun(context.Background(), runID, agent.AgentContext{
		SystemInstructions: []agent.Message{
			{
				Role:    "system",
				Content: "你正在一个 4 轮 agent runtime 中运行。每一轮回复都必须包含精确字符串 FIRST_ROUND_OK。第 1 轮必须直接回答用户的第一轮问题。第 2 到第 4 轮继续简短输出当前进度。第 4 轮再次明确重复第一轮答案。每轮输出不超过三句话。",
				Pinned:  true,
			},
		},
		Messages: []agent.Message{
			{Role: "user", Content: firstQuestion},
		},
	}, 4)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	if err := engine.Start(context.Background(), runID); err != nil {
		t.Fatalf("start run: %v", err)
	}

	snapshot := waitForState(t, engine, runID, agent.StateCompleted, 3*time.Minute)
	assistantMessages := collectAssistantMessages(snapshot.Context.Messages)
	t.Logf("assistant messages: %#v", assistantMessages)
	if len(assistantMessages) < 4 {
		t.Fatalf("expected at least 4 assistant messages, got %d", len(assistantMessages))
	}
	if !strings.Contains(assistantMessages[0], "FIRST_ROUND_OK") {
		t.Fatalf("first round did not answer the first question, got: %q", assistantMessages[0])
	}
	if !strings.Contains(assistantMessages[len(assistantMessages)-1], "FIRST_ROUND_OK") {
		t.Fatalf("final round did not retain first-round answer, got: %q", assistantMessages[len(assistantMessages)-1])
	}

	if _, err := historyStore.LoadContext(context.Background(), runID); err != nil {
		t.Fatalf("load persisted local history: %v", err)
	}
}

func collectAssistantMessages(messages []agent.Message) []string {
	result := make([]string, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "assistant" {
			result = append(result, msg.Content)
		}
	}
	return result
}
