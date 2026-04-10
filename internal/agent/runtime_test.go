package agent_test

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"github.com/young/go/agent-arch/internal/agent"
	"github.com/young/go/agent-arch/internal/contextx"
	"github.com/young/go/agent-arch/internal/llm"
	"github.com/young/go/agent-arch/internal/repo"
)

func TestRuntimeTimeoutPatchAndResume(t *testing.T) {
	t.Parallel()

	store := repo.NewMemoryAgentRepo()
	manager := contextx.NewManager(contextx.ApproximateEstimator{})
	client := llm.NewMockClient(5 * time.Millisecond)
	logger := log.New(io.Discard, "", 0)

	engine := agent.NewEngine(store, client, manager, nil, logger, agent.RuntimeConfig{
		MaxRounds:   3,
		TokenBudget: 4096,
		LLMTimeout:  10 * time.Millisecond,
	})

	ctx := context.Background()
	_, err := engine.CreateRun(ctx, "timeout-run", agent.AgentContext{
		SystemInstructions: []agent.Message{
			{Role: "system", Content: "stay precise", Pinned: true},
		},
		Messages: []agent.Message{
			{Role: "user", Content: "please timeout"},
		},
	}, 3)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	if err := engine.Start(ctx, "timeout-run"); err != nil {
		t.Fatalf("start run: %v", err)
	}

	snapshot := waitForState(t, engine, "timeout-run", agent.StateWaitingHuman, 2*time.Second)
	if snapshot.LastError == "" {
		t.Fatalf("expected last error to be preserved")
	}

	err = engine.PatchContextAndResume(ctx, "timeout-run", agent.ContextPatch{
		Operation: agent.PatchAppend,
		Messages: []agent.Message{
			{Role: "user", Content: "recover and continue"},
		},
	})
	if err != nil {
		t.Fatalf("patch and resume: %v", err)
	}

	snapshot = waitForState(t, engine, "timeout-run", agent.StateCompleted, 2*time.Second)
	if snapshot.Round < 2 {
		t.Fatalf("expected runtime to finish multiple rounds, got %d", snapshot.Round)
	}
	if snapshot.LastError != "" {
		t.Fatalf("expected last error to be cleared, got %q", snapshot.LastError)
	}
}

func waitForState(t *testing.T, engine *agent.Engine, runID string, want agent.State, timeout time.Duration) agent.Snapshot {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snapshot, err := engine.GetSnapshot(context.Background(), runID)
		if err != nil {
			t.Fatalf("get snapshot: %v", err)
		}
		if snapshot.State == want {
			return snapshot
		}
		time.Sleep(10 * time.Millisecond)
	}

	snapshot, err := engine.GetSnapshot(context.Background(), runID)
	if err != nil {
		t.Fatalf("get snapshot after timeout: %v", err)
	}
	t.Fatalf("timed out waiting for state %s, got %s", want, snapshot.State)
	return agent.Snapshot{}
}
