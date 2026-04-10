package history_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/young/go/agent-arch/internal/agent"
	"github.com/young/go/agent-arch/internal/history"
)

func TestFileStoreRoundTrip(t *testing.T) {
	t.Parallel()

	store := history.NewFileStore(filepath.Join(t.TempDir(), "history"))
	want := agent.AgentContext{
		SystemInstructions: []agent.Message{{Role: "system", Content: "keep this", Pinned: true}},
		Messages:           []agent.Message{{Role: "user", Content: "hello"}},
	}

	if err := store.SaveContext(context.Background(), "run-1", want); err != nil {
		t.Fatalf("save context: %v", err)
	}

	got, err := store.LoadContext(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("load context: %v", err)
	}

	if len(got.Messages) != 1 || got.Messages[0].Content != "hello" {
		t.Fatalf("unexpected roundtrip result: %+v", got)
	}
}
