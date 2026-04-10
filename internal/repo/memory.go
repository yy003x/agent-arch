package repo

import (
	"context"
	"fmt"
	"sync"

	"github.com/young/go/agent-arch/internal/agent"
)

type MemoryAgentRepo struct {
	mu    sync.RWMutex
	store map[string]agent.Snapshot
}

func NewMemoryAgentRepo() *MemoryAgentRepo {
	return &MemoryAgentRepo{store: make(map[string]agent.Snapshot)}
}

func (r *MemoryAgentRepo) Create(_ context.Context, snapshot agent.Snapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.store[snapshot.RunID]; exists {
		return fmt.Errorf("%w: %s", agent.ErrRunExists, snapshot.RunID)
	}

	r.store[snapshot.RunID] = clone(snapshot)
	return nil
}

func (r *MemoryAgentRepo) Save(_ context.Context, snapshot agent.Snapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.store[snapshot.RunID] = clone(snapshot)
	return nil
}

func (r *MemoryAgentRepo) Get(_ context.Context, runID string) (agent.Snapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot, ok := r.store[runID]
	if !ok {
		return agent.Snapshot{}, fmt.Errorf("%w: %s", agent.ErrRunNotFound, runID)
	}
	return clone(snapshot), nil
}
