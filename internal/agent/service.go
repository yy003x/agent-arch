package agent

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"agent-arch/internal/config"
	"agent-arch/internal/llm"
	"agent-arch/internal/llm/anthropic"
	"agent-arch/internal/llm/openai"
	"agent-arch/internal/memory"
	"agent-arch/internal/persona"
	"agent-arch/internal/token"
)

type ClientFactory interface {
	New(provider string, cfg config.ProviderConfig) (llm.Client, error)
}

type DefaultClientFactory struct{}

func (DefaultClientFactory) New(provider string, cfg config.ProviderConfig) (llm.Client, error) {
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
	}

	switch provider {
	case "openai":
		return openai.NewClient(cfg.BaseURL, cfg.APIKey, httpClient), nil
	case "anthropic":
		return anthropic.NewClient(cfg.BaseURL, cfg.APIKey, httpClient), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", provider)
	}
}

type Service struct {
	cfg      config.Config
	loader   *persona.Loader
	factory  ClientFactory
	memory   *memory.Manager
	mu       sync.RWMutex
	sessions map[string]Session
}

func NewService(ctx context.Context, cfg config.Config, personaDir string) (*Service, error) {
	_ = ctx

	store := memory.NewInMemoryStore()
	manager := memory.NewManager(store, memory.StubRetriever{}, cfg.Memory, token.NewApproxCounter())

	return &Service{
		cfg:      cfg,
		loader:   persona.NewLoader(personaDir),
		factory:  DefaultClientFactory{},
		memory:   manager,
		sessions: make(map[string]Session),
	}, nil
}

func NewServiceWithDeps(cfg config.Config, loader *persona.Loader, factory ClientFactory, manager *memory.Manager) *Service {
	return &Service{
		cfg:      cfg,
		loader:   loader,
		factory:  factory,
		memory:   manager,
		sessions: make(map[string]Session),
	}
}

func (s *Service) CreateAgent(ctx context.Context, req CreateAgentRequest) (CreateAgentResponse, error) {
	personaID := req.PersonaID
	if personaID == "" {
		personaID = s.cfg.Agent.DefaultPersona
	}

	p, err := s.loader.Load(ctx, personaID)
	if err != nil {
		return CreateAgentResponse{}, fmt.Errorf("load persona: %w", err)
	}

	provider := req.Provider
	if provider == "" {
		provider = p.ModelPolicy.Provider
	}
	if provider == "" {
		provider = s.cfg.Agent.DefaultProvider
	}

	model := req.Model
	if model == "" {
		model = p.ModelPolicy.Model
	}
	if model == "" {
		if providerCfg, ok := s.cfg.Providers[provider]; ok && providerCfg.Model != "" {
			model = providerCfg.Model
		}
	}
	if model == "" {
		model = s.cfg.Agent.DefaultModel
	}

	if _, err := s.newClient(provider); err != nil {
		return CreateAgentResponse{}, err
	}

	sessionID := fmt.Sprintf("sess_%d", time.Now().UnixNano())
	session := Session{
		ID:       sessionID,
		Persona:  p,
		Provider: provider,
		Model:    model,
	}

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	if err := s.memory.InitSession(ctx, sessionID); err != nil {
		return CreateAgentResponse{}, fmt.Errorf("init session memory: %w", err)
	}

	return CreateAgentResponse{
		SessionID: sessionID,
		PersonaID: p.ID,
		Provider:  provider,
		Model:     model,
	}, nil
}

func (s *Service) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	session, err := s.getSession(req.SessionID)
	if err != nil {
		return ChatResponse{}, err
	}

	client, err := s.newClient(session.Provider)
	if err != nil {
		return ChatResponse{}, err
	}

	if err := s.memory.AppendTurn(ctx, req.SessionID, memory.Turn{
		Role:    "user",
		Content: req.Message,
	}); err != nil {
		return ChatResponse{}, fmt.Errorf("append user turn: %w", err)
	}

	system := persona.RenderSystem(session.Persona)
	contextMessages, _, err := s.memory.BuildContext(ctx, req.SessionID, req.Message, system)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("build context: %w", err)
	}
	augmentedSystem, conversationMessages := splitContextMessages(system, contextMessages)

	llmReq := llm.Request{
		Model:           session.Model,
		System:          augmentedSystem,
		Messages:        conversationMessages,
		Temperature:     session.Persona.ModelPolicy.Temperature,
		MaxOutputTokens: session.Persona.ModelPolicy.MaxOutputTokens,
	}
	if llmReq.MaxOutputTokens == 0 {
		llmReq.MaxOutputTokens = s.cfg.Memory.ResponseReservedTokens
	}

	resp, err := client.Generate(ctx, llmReq)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("generate response: %w", err)
	}

	if err := s.memory.AppendTurn(ctx, req.SessionID, memory.Turn{
		Role:    "assistant",
		Content: resp.OutputText,
	}); err != nil {
		return ChatResponse{}, fmt.Errorf("append assistant turn: %w", err)
	}

	return ChatResponse{
		SessionID: req.SessionID,
		Message:   resp.OutputText,
	}, nil
}

func (s *Service) GetMemory(ctx context.Context, sessionID string) (*memory.SessionMemory, error) {
	return s.memory.Get(ctx, sessionID)
}

func (s *Service) newClient(provider string) (llm.Client, error) {
	providerCfg, ok := s.cfg.Providers[provider]
	if !ok {
		return nil, fmt.Errorf("provider %q not configured", provider)
	}
	if !providerCfg.Enabled {
		return nil, fmt.Errorf("provider %q disabled", provider)
	}
	if providerCfg.TimeoutSeconds == 0 {
		providerCfg.TimeoutSeconds = 30
	}
	return s.factory.New(provider, providerCfg)
}

func (s *Service) getSession(sessionID string) (Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return Session{}, fmt.Errorf("session %q not found", sessionID)
	}
	return session, nil
}

func splitContextMessages(baseSystem string, messages []llm.Message) (string, []llm.Message) {
	systemParts := []string{baseSystem}
	out := make([]llm.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "system" {
			if msg.Content != "" {
				systemParts = append(systemParts, msg.Content)
			}
			continue
		}
		out = append(out, msg)
	}
	return strings.Join(systemParts, "\n\n"), out
}
