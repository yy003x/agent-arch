package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type Engine struct {
	repo       AgentRepo
	llmClient  LLMClient
	ctxManager ContextManager
	history    HistoryStore
	logger     *log.Logger
	config     RuntimeConfig

	mu       sync.Mutex
	runtimes map[string]*Runtime
}

func NewEngine(repo AgentRepo, llmClient LLMClient, ctxManager ContextManager, history HistoryStore, logger *log.Logger, config RuntimeConfig) *Engine {
	if logger == nil {
		logger = log.Default()
	}
	return &Engine{
		repo:       repo,
		llmClient:  llmClient,
		ctxManager: ctxManager,
		history:    history,
		logger:     logger,
		config:     config,
		runtimes:   make(map[string]*Runtime),
	}
}

func (e *Engine) CreateRun(ctx context.Context, runID string, initial AgentContext, maxRounds int) (Snapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.runtimes[runID]; exists {
		return Snapshot{}, fmt.Errorf("%w: %s", ErrRunExists, runID)
	}
	if maxRounds <= 0 {
		maxRounds = e.config.MaxRounds
		if maxRounds <= 0 {
			maxRounds = 10
		}
	}

	now := time.Now().UTC()
	snapshot := Snapshot{
		RunID:     runID,
		State:     StateCreated,
		MaxRounds: maxRounds,
		Context:   initial,
		CreatedAt: now,
		UpdatedAt: now,
	}
	snapshot.Events = append(snapshot.Events, Event{
		Sequence:  1,
		Type:      EventRunCreated,
		Time:      now,
		FromState: StateCreated,
		ToState:   StateCreated,
		Message:   "run created",
	})

	if err := e.repo.Create(ctx, snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("%w: create run %s: %v", ErrRepository, runID, err)
	}
	if e.history != nil {
		if err := e.history.SaveContext(ctx, runID, snapshot.Context); err != nil {
			return Snapshot{}, fmt.Errorf("%w: seed history %s: %v", ErrRepository, runID, err)
		}
	}

	runtime := NewRuntime(e.repo, e.llmClient, e.ctxManager, e.history, e.logger, snapshot, e.config)
	e.runtimes[runID] = runtime
	return runtime.GetSnapshot(), nil
}

func (e *Engine) Start(ctx context.Context, runID string) error {
	runtime, err := e.getRuntime(ctx, runID)
	if err != nil {
		return err
	}
	return runtime.Start(ctx)
}

func (e *Engine) Block(ctx context.Context, runID, reason string) error {
	runtime, err := e.getRuntime(ctx, runID)
	if err != nil {
		return err
	}
	return runtime.Block(ctx, reason)
}

func (e *Engine) Stop(ctx context.Context, runID, reason string) error {
	runtime, err := e.getRuntime(ctx, runID)
	if err != nil {
		return err
	}
	return runtime.Stop(ctx, reason)
}

func (e *Engine) Cancel(ctx context.Context, runID, reason string) error {
	runtime, err := e.getRuntime(ctx, runID)
	if err != nil {
		return err
	}
	return runtime.Cancel(ctx, reason)
}

func (e *Engine) Continue(ctx context.Context, runID string) error {
	runtime, err := e.getRuntime(ctx, runID)
	if err != nil {
		return err
	}
	return runtime.Continue(ctx)
}

func (e *Engine) PatchContextAndResume(ctx context.Context, runID string, patch ContextPatch) error {
	runtime, err := e.getRuntime(ctx, runID)
	if err != nil {
		return err
	}
	return runtime.PatchContextAndResume(ctx, patch)
}

func (e *Engine) GetSnapshot(ctx context.Context, runID string) (Snapshot, error) {
	runtime, err := e.getRuntime(ctx, runID)
	if err == nil {
		return runtime.GetSnapshot(), nil
	}
	snapshot, repoErr := e.repo.Get(ctx, runID)
	if repoErr != nil {
		return Snapshot{}, repoErr
	}
	return snapshot, nil
}

func (e *Engine) getRuntime(ctx context.Context, runID string) (*Runtime, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if runtime, ok := e.runtimes[runID]; ok {
		return runtime, nil
	}

	snapshot, err := e.repo.Get(ctx, runID)
	if err != nil {
		return nil, err
	}
	if e.history != nil {
		historyContext, historyErr := e.history.LoadContext(ctx, runID)
		if historyErr == nil {
			snapshot.Context = historyContext
		}
	}

	runtime := NewRuntime(e.repo, e.llmClient, e.ctxManager, e.history, e.logger, snapshot, e.config)
	e.runtimes[runID] = runtime
	return runtime, nil
}
