package agent_test

import (
	"testing"

	"github.com/young/go/agent-arch/internal/agent"
)

func TestValidateTransition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		from    agent.State
		to      agent.State
		wantErr bool
	}{
		{name: "created to running", from: agent.StateCreated, to: agent.StateRunning},
		{name: "waiting llm to waiting human", from: agent.StateWaitingLLM, to: agent.StateWaitingHuman},
		{name: "blocked to running", from: agent.StateBlocked, to: agent.StateRunning},
		{name: "completed to running invalid", from: agent.StateCompleted, to: agent.StateRunning, wantErr: true},
		{name: "waiting human to stopped invalid", from: agent.StateWaitingHuman, to: agent.StateStopped, wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := agent.ValidateTransition(tc.from, tc.to)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %s -> %s", tc.from, tc.to)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %s -> %s: %v", tc.from, tc.to, err)
			}
		})
	}
}
