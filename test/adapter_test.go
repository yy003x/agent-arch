package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"agent-arch/internal/agent"
	"agent-arch/internal/config"
	"agent-arch/internal/llm"
	"agent-arch/internal/memory"
	"agent-arch/internal/persona"
	"agent-arch/internal/token"
)

type stubFactory struct {
	providers []string
}

func (s *stubFactory) New(provider string, cfg config.ProviderConfig) (llm.Client, error) {
	s.providers = append(s.providers, provider)
	return stubClient{}, nil
}

type stubClient struct{}

func (stubClient) Generate(ctx context.Context, req llm.Request) (llm.Response, error) {
	return llm.Response{OutputText: "ok"}, nil
}

func TestProviderSelectionFromPersona(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "writer.yaml"), []byte(`
id: "writer"
system_prompt: "system"
model_policy:
  provider: "anthropic"
  model: "claude-test"
  max_output_tokens: 256
memory_policy:
  summary_enabled: true
  retrieval_enabled: true
response_policy:
  format: "text"
  verbosity: "medium"
`), 0o644); err != nil {
		t.Fatalf("write persona: %v", err)
	}

	cfg := config.Config{
		Agent: config.AgentConfig{
			DefaultPersona:  "writer",
			DefaultProvider: "openai",
			DefaultModel:    "fallback-model",
		},
		Providers: map[string]config.ProviderConfig{
			"openai":    {Enabled: true, TimeoutSeconds: 1},
			"anthropic": {Enabled: true, TimeoutSeconds: 1},
		},
		Memory: config.MemoryConfig{
			MaxContextTokens:       1024,
			ResponseReservedTokens: 128,
			SafetyBufferTokens:     64,
			RecentTurnsReserved:    256,
			CompressionThreshold:   0.8,
			KeepRecentTurns:        4,
		},
	}

	manager := memory.NewManager(memory.NewInMemoryStore(), memory.StubRetriever{}, cfg.Memory, token.NewApproxCounter())
	factory := &stubFactory{}
	service := agent.NewServiceWithDeps(cfg, persona.NewLoader(dir), factory, manager)

	resp, err := service.CreateAgent(context.Background(), agent.CreateAgentRequest{PersonaID: "writer"})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}

	if resp.Provider != "anthropic" {
		t.Fatalf("expected anthropic provider, got %s", resp.Provider)
	}
	if resp.Model != "claude-test" {
		t.Fatalf("expected persona model, got %s", resp.Model)
	}
}
