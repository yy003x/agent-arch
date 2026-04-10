package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/young/go/agent-arch/internal/llm"
)

type AgentRepo interface {
	Create(ctx context.Context, snapshot Snapshot) error
	Save(ctx context.Context, snapshot Snapshot) error
	Get(ctx context.Context, runID string) (Snapshot, error)
}

type ContextManager interface {
	BuildPrompt(ctx AgentContext, tokenBudget int) ([]Message, error)
	ApplyPatch(ctx AgentContext, patch ContextPatch) (AgentContext, error)
}

type LLMClient interface {
	Generate(ctx context.Context, req llm.Request) (llm.Response, error)
}

type HistoryStore interface {
	SaveContext(ctx context.Context, runID string, agentContext AgentContext) error
	LoadContext(ctx context.Context, runID string) (AgentContext, error)
}

type Runtime struct {
	repo       AgentRepo
	llmClient  LLMClient
	ctxManager ContextManager
	history    HistoryStore
	logger     *log.Logger
	config     RuntimeConfig

	mu           sync.Mutex
	snapshot     Snapshot
	inflightStop context.CancelFunc
}

func NewRuntime(repo AgentRepo, llmClient LLMClient, ctxManager ContextManager, history HistoryStore, logger *log.Logger, snapshot Snapshot, config RuntimeConfig) *Runtime {
	if logger == nil {
		logger = log.Default()
	}
	if config.MaxRounds <= 0 {
		config.MaxRounds = 10
	}
	if config.TokenBudget <= 0 {
		config.TokenBudget = 128000
	}
	if config.LLMTimeout <= 0 {
		config.LLMTimeout = 5 * time.Second
	}

	return &Runtime{
		repo:       repo,
		llmClient:  llmClient,
		ctxManager: ctxManager,
		history:    history,
		logger:     logger,
		config:     config,
		snapshot:   CloneSnapshot(snapshot),
	}
}

func (r *Runtime) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := ValidateTransition(r.snapshot.State, StateRunning); err != nil {
		return err
	}

	r.transitionLocked(StateRunning, EventRunStarted, "run started", "")
	if err := r.saveLocked(ctx); err != nil {
		return err
	}
	go r.loop()
	return nil
}

func (r *Runtime) Block(ctx context.Context, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.snapshot.State != StateRunning && r.snapshot.State != StateWaitingLLM {
		return fmt.Errorf("%w: block requires running or waiting_llm, got %s", ErrInvalidTransition, r.snapshot.State)
	}

	r.transitionLocked(StateBlocked, EventBlocked, reason, "")
	r.cancelInflightLocked()
	return r.saveLocked(ctx)
}

func (r *Runtime) Stop(ctx context.Context, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.snapshot.State != StateRunning && r.snapshot.State != StateWaitingLLM {
		return fmt.Errorf("%w: stop requires running or waiting_llm, got %s", ErrInvalidTransition, r.snapshot.State)
	}

	r.transitionLocked(StateStopped, EventStopped, reason, "")
	r.cancelInflightLocked()
	return r.saveLocked(ctx)
}

func (r *Runtime) Cancel(ctx context.Context, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if IsTerminalState(r.snapshot.State) {
		return fmt.Errorf("%w: cannot cancel terminal state %s", ErrInvalidTransition, r.snapshot.State)
	}

	r.transitionLocked(StateCancelled, EventCancelled, reason, "")
	r.cancelInflightLocked()
	return r.saveLocked(ctx)
}

func (r *Runtime) Continue(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.snapshot.State != StateBlocked && r.snapshot.State != StateStopped {
		return fmt.Errorf("%w: continue requires blocked or stopped, got %s", ErrInvalidTransition, r.snapshot.State)
	}

	r.transitionLocked(StateRunning, EventStateTransition, "run continued", "")
	if err := r.saveLocked(ctx); err != nil {
		return err
	}
	go r.loop()
	return nil
}

func (r *Runtime) PatchContextAndResume(ctx context.Context, patch ContextPatch) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.snapshot.State != StateWaitingHuman {
		return fmt.Errorf("%w: patch and resume requires waiting_human, got %s", ErrInvalidTransition, r.snapshot.State)
	}

	nextContext, err := r.ctxManager.ApplyPatch(r.snapshot.Context, patch)
	if err != nil {
		return fmt.Errorf("apply patch: %w", err)
	}
	r.snapshot.Context = nextContext
	r.snapshot.LastError = ""
	r.snapshot.UpdatedAt = time.Now().UTC()
	r.appendEventLocked(EventHumanPatched, r.snapshot.State, r.snapshot.State, "human patched context", "")
	r.transitionLocked(StateRunning, EventStateTransition, "resumed after human patch", "")
	if err := r.saveLocked(ctx); err != nil {
		return err
	}
	go r.loop()
	return nil
}

func (r *Runtime) GetSnapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return CloneSnapshot(r.snapshot)
}

func (r *Runtime) loop() {
	ctx := context.Background()
	for {
		state, done, err := r.executeRound(ctx)
		if err != nil {
			r.logger.Printf("run_id=%s round=%d state=%s err=%v", r.snapshot.RunID, r.snapshot.Round, state, err)
			return
		}
		if done {
			return
		}
	}
}

func (r *Runtime) executeRound(ctx context.Context) (State, bool, error) {
	r.mu.Lock()
	if r.snapshot.State != StateRunning {
		state := r.snapshot.State
		r.mu.Unlock()
		return state, true, nil
	}
	if r.snapshot.Round >= r.snapshot.MaxRounds {
		r.transitionLocked(StateCompleted, EventCompleted, "max rounds reached", "")
		err := r.saveLocked(ctx)
		state := r.snapshot.State
		r.mu.Unlock()
		return state, true, err
	}

	prompt, err := r.ctxManager.BuildPrompt(r.snapshot.Context, r.config.TokenBudget)
	if err != nil {
		r.transitionLocked(StateFailed, EventStateTransition, "context build failed", err.Error())
		saveErr := r.saveLocked(ctx)
		state := r.snapshot.State
		r.mu.Unlock()
		if saveErr != nil {
			return state, true, saveErr
		}
		return state, true, err
	}

	r.transitionLocked(StateWaitingLLM, EventLLMRequestStarted, "llm request started", "")
	reqCtx, cancel := context.WithTimeout(ctx, r.config.LLMTimeout)
	r.inflightStop = cancel
	request := llm.Request{
		RunID:    r.snapshot.RunID,
		Round:    r.snapshot.Round,
		Messages: toLLMMessages(prompt),
	}
	if err := r.saveLocked(ctx); err != nil {
		r.inflightStop = nil
		cancel()
		state := r.snapshot.State
		r.mu.Unlock()
		return state, true, err
	}
	runID := r.snapshot.RunID
	round := r.snapshot.Round
	r.logger.Printf("run_id=%s round=%d event=llm_request_started", runID, round)
	r.mu.Unlock()

	resp, llmErr := r.llmClient.Generate(reqCtx, request)

	r.mu.Lock()
	if r.inflightStop != nil {
		r.inflightStop()
		r.inflightStop = nil
	}

	if r.snapshot.State != StateWaitingLLM {
		state := r.snapshot.State
		r.mu.Unlock()
		return state, true, nil
	}

	if llmErr != nil {
		return r.handleLLMErrorLocked(ctx, llmErr)
	}

	r.snapshot.Context.Messages = append(r.snapshot.Context.Messages, Message{
		Role:    resp.Message.Role,
		Content: resp.Message.Content,
		Pinned:  resp.Message.Pinned,
	})
	r.snapshot.Round++
	r.snapshot.LastError = ""
	r.appendEventLocked(EventLLMRequestEnded, StateWaitingLLM, StateRunning, "llm request completed", "")

	if resp.Done || r.snapshot.Round >= r.snapshot.MaxRounds {
		r.transitionLocked(StateCompleted, EventCompleted, "run completed", "")
		err := r.saveLocked(ctx)
		state := r.snapshot.State
		r.mu.Unlock()
		return state, true, err
	}

	r.transitionLocked(StateRunning, EventStateTransition, "next round scheduled", "")
	err = r.saveLocked(ctx)
	state := r.snapshot.State
	r.mu.Unlock()
	return state, false, err
}

func (r *Runtime) handleLLMErrorLocked(ctx context.Context, llmErr error) (State, bool, error) {
	switch {
	case llm.IsCanceled(llmErr):
		state := r.snapshot.State
		r.mu.Unlock()
		return state, true, nil
	case llm.IsTimeout(llmErr):
		r.transitionLocked(StateWaitingHuman, EventLLMRequestFailed, "llm timeout", ErrLLMTimeout.Error())
	case errors.Is(llmErr, llm.ErrUpstream):
		r.transitionLocked(StateWaitingHuman, EventLLMRequestFailed, "llm upstream error", llmErr.Error())
	default:
		r.transitionLocked(StateWaitingHuman, EventLLMRequestFailed, "llm request failed", llmErr.Error())
	}

	err := r.saveLocked(ctx)
	state := r.snapshot.State
	r.mu.Unlock()
	if err != nil {
		return state, true, err
	}
	return state, true, nil
}

func (r *Runtime) transitionLocked(next State, eventType EventType, message, eventErr string) {
	prev := r.snapshot.State
	if err := ValidateTransition(prev, next); err != nil {
		panic(err)
	}
	r.snapshot.State = next
	r.snapshot.UpdatedAt = time.Now().UTC()
	if eventErr != "" {
		r.snapshot.LastError = eventErr
	}
	r.appendEventLocked(eventType, prev, next, message, eventErr)
	r.logger.Printf("run_id=%s from=%s to=%s message=%q", r.snapshot.RunID, prev, next, message)
}

func (r *Runtime) appendEventLocked(eventType EventType, from, to State, message, eventErr string) {
	r.snapshot.Events = append(r.snapshot.Events, Event{
		Sequence:  int64(len(r.snapshot.Events) + 1),
		Type:      eventType,
		Time:      time.Now().UTC(),
		FromState: from,
		ToState:   to,
		Round:     r.snapshot.Round,
		Message:   message,
		Error:     eventErr,
	})
}

func (r *Runtime) saveLocked(ctx context.Context) error {
	r.snapshot.UpdatedAt = time.Now().UTC()
	if err := r.repo.Save(ctx, r.snapshot); err != nil {
		return fmt.Errorf("%w: save snapshot %s: %v", ErrRepository, r.snapshot.RunID, err)
	}
	if r.history != nil {
		if err := r.history.SaveContext(ctx, r.snapshot.RunID, r.snapshot.Context); err != nil {
			return fmt.Errorf("%w: save history %s: %v", ErrRepository, r.snapshot.RunID, err)
		}
	}
	return nil
}

func (r *Runtime) cancelInflightLocked() {
	if r.inflightStop != nil {
		r.inflightStop()
		r.inflightStop = nil
	}
}

func toLLMMessages(messages []Message) []llm.Message {
	result := make([]llm.Message, 0, len(messages))
	for _, msg := range messages {
		result = append(result, llm.Message{
			Role:    msg.Role,
			Content: msg.Content,
			Pinned:  msg.Pinned,
		})
	}
	return result
}
