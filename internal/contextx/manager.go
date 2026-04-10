package contextx

import (
	"fmt"

	"github.com/young/go/agent-arch/internal/agent"
)

type Manager struct {
	Estimator TokenEstimator
}

func NewManager(estimator TokenEstimator) *Manager {
	if estimator == nil {
		estimator = ApproximateEstimator{}
	}
	return &Manager{Estimator: estimator}
}

func (m *Manager) BuildPrompt(ctx agent.AgentContext, tokenBudget int) ([]agent.Message, error) {
	if tokenBudget <= 0 {
		return nil, fmt.Errorf("invalid token budget: %d", tokenBudget)
	}
	return TruncateMessages(ctx.SystemInstructions, ctx.Messages, tokenBudget, m.Estimator), nil
}

func (m *Manager) ApplyPatch(ctx agent.AgentContext, patch agent.ContextPatch) (agent.AgentContext, error) {
	next := agent.AgentContext{
		SystemInstructions: append([]agent.Message(nil), ctx.SystemInstructions...),
		Messages:           append([]agent.Message(nil), ctx.Messages...),
	}

	switch patch.Operation {
	case agent.PatchAppend:
		next.SystemInstructions = append(next.SystemInstructions, patch.SystemInstructions...)
		next.Messages = append(next.Messages, patch.Messages...)
	case agent.PatchReplace:
		if patch.SystemInstructions != nil {
			next.SystemInstructions = append([]agent.Message(nil), patch.SystemInstructions...)
		}
		if patch.Messages != nil {
			next.Messages = append([]agent.Message(nil), patch.Messages...)
		}
	default:
		return agent.AgentContext{}, fmt.Errorf("unsupported patch operation: %s", patch.Operation)
	}

	return next, nil
}
