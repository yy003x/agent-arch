package api

import "github.com/young/go/agent-arch/internal/agent"

type StartRequest struct {
	RunID     string
	MaxRounds int
	Context   agent.AgentContext
}

type ReasonRequest struct {
	Reason string
}

type PatchAndResumeRequest struct {
	Patch agent.ContextPatch
}

type SnapshotResponse struct {
	RunID     string             `json:"run_id"`
	State     agent.State        `json:"state"`
	Round     int                `json:"round"`
	MaxRounds int                `json:"max_rounds"`
	LastError string             `json:"last_error,omitempty"`
	Context   agent.AgentContext `json:"context"`
	Events    []agent.Event      `json:"events"`
	CreatedAt string             `json:"created_at"`
	UpdatedAt string             `json:"updated_at"`
}
