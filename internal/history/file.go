package history

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/young/go/agent-arch/internal/agent"
)

type FileStore struct {
	dir string
}

func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir}
}

func (s *FileStore) SaveContext(_ context.Context, runID string, agentContext agent.AgentContext) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("mkdir history dir: %w", err)
	}

	payload, err := json.MarshalIndent(agentContext, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal history: %w", err)
	}

	if err := os.WriteFile(s.path(runID), payload, 0o644); err != nil {
		return fmt.Errorf("write history: %w", err)
	}
	return nil
}

func (s *FileStore) LoadContext(_ context.Context, runID string) (agent.AgentContext, error) {
	payload, err := os.ReadFile(s.path(runID))
	if err != nil {
		return agent.AgentContext{}, err
	}

	var agentContext agent.AgentContext
	if err := json.Unmarshal(payload, &agentContext); err != nil {
		return agent.AgentContext{}, fmt.Errorf("unmarshal history: %w", err)
	}
	return agentContext, nil
}

func (s *FileStore) path(runID string) string {
	return filepath.Join(s.dir, runID+".json")
}
