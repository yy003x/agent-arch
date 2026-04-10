package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/young/go/agent-arch/internal/agent"
	"github.com/young/go/agent-arch/internal/api"
	"github.com/young/go/agent-arch/internal/contextx"
	"github.com/young/go/agent-arch/internal/llm"
	"github.com/young/go/agent-arch/internal/repo"
)

func TestHTTPStartAndGetSnapshot(t *testing.T) {
	t.Parallel()

	handler := newTestHandler()

	startBody := api.StartRequest{
		RunID:     "http-run",
		MaxRounds: 2,
		Context: agent.AgentContext{
			SystemInstructions: []agent.Message{{Role: "system", Content: "be concise", Pinned: true}},
			Messages:           []agent.Message{{Role: "user", Content: "hello"}},
		},
	}
	resp := doJSON(t, handler, http.MethodPost, "/runs", startBody)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	waitForHTTPState(t, handler, "/runs/http-run", agent.StateCompleted, 2*time.Second)
}

func TestHTTPPatchResumeFlow(t *testing.T) {
	t.Parallel()

	store := repo.NewMemoryAgentRepo()
	manager := contextx.NewManager(contextx.ApproximateEstimator{})
	client := llm.NewMockClient(5 * time.Millisecond)
	engine := agent.NewEngine(store, client, manager, nil, log.New(io.Discard, "", 0), agent.RuntimeConfig{
		MaxRounds:   3,
		TokenBudget: 4096,
		LLMTimeout:  50 * time.Millisecond,
	})
	service := api.NewService(engine)
	handler := api.NewHTTPServer(service).Handler()

	startBody := api.StartRequest{
		RunID: "patch-run",
		Context: agent.AgentContext{
			SystemInstructions: []agent.Message{{Role: "system", Content: "be careful", Pinned: true}},
			Messages:           []agent.Message{{Role: "user", Content: "please fail"}},
		},
	}
	resp := doJSON(t, handler, http.MethodPost, "/runs", startBody)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected start status: %d", resp.StatusCode)
	}

	waitForHTTPState(t, handler, "/runs/patch-run", agent.StateWaitingHuman, 2*time.Second)

	patchBody := api.PatchAndResumeRequest{
		Patch: agent.ContextPatch{
			Operation: agent.PatchAppend,
			Messages:  []agent.Message{{Role: "user", Content: "recover and continue"}},
		},
	}
	resp = doJSON(t, handler, http.MethodPost, "/runs/patch-run/patch-resume", patchBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected patch status: %d", resp.StatusCode)
	}

	waitForHTTPState(t, handler, "/runs/patch-run", agent.StateCompleted, 2*time.Second)
}

func newTestHandler() http.Handler {
	store := repo.NewMemoryAgentRepo()
	manager := contextx.NewManager(contextx.ApproximateEstimator{})
	client := llm.NewMockClient(5 * time.Millisecond)
	engine := agent.NewEngine(store, client, manager, nil, log.New(io.Discard, "", 0), agent.RuntimeConfig{
		MaxRounds:   2,
		TokenBudget: 4096,
		LLMTimeout:  50 * time.Millisecond,
	})
	service := api.NewService(engine)
	return api.NewHTTPServer(service).Handler()
}

func doJSON(t *testing.T, handler http.Handler, method, path string, body any) *http.Response {
	t.Helper()

	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}

	req, err := http.NewRequestWithContext(context.Background(), method, path, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("content-type", "application/json")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder.Result()
}

func waitForHTTPState(t *testing.T, handler http.Handler, path string, want agent.State, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		if err != nil {
			t.Fatalf("new get request: %v", err)
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		resp := recorder.Result()

		var snapshot api.SnapshotResponse
		if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
			resp.Body.Close()
			t.Fatalf("decode snapshot: %v", err)
		}
		resp.Body.Close()

		if snapshot.State == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", want)
}
