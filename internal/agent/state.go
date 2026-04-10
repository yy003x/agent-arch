package agent

import "fmt"

type State string

const (
	StateCreated      State = "created"
	StateRunning      State = "running"
	StateWaitingLLM   State = "waiting_llm"
	StateWaitingHuman State = "waiting_human"
	StateBlocked      State = "blocked"
	StateStopped      State = "stopped"
	StateCompleted    State = "completed"
	StateCancelled    State = "cancelled"
	StateFailed       State = "failed"
)

func IsTerminalState(state State) bool {
	switch state {
	case StateCompleted, StateCancelled, StateFailed:
		return true
	default:
		return false
	}
}

func ValidateTransition(from, to State) error {
	if from == to {
		return nil
	}

	var allowed bool
	switch from {
	case StateCreated:
		allowed = to == StateRunning || to == StateCancelled
	case StateRunning:
		allowed = to == StateWaitingLLM || to == StateBlocked || to == StateStopped || to == StateCancelled || to == StateCompleted || to == StateFailed
	case StateWaitingLLM:
		allowed = to == StateRunning || to == StateWaitingHuman || to == StateBlocked || to == StateStopped || to == StateCancelled || to == StateFailed || to == StateCompleted
	case StateWaitingHuman:
		allowed = to == StateRunning || to == StateCancelled
	case StateBlocked:
		allowed = to == StateRunning || to == StateCancelled
	case StateStopped:
		allowed = to == StateRunning || to == StateCancelled
	case StateCompleted, StateCancelled, StateFailed:
		allowed = false
	}

	if allowed {
		return nil
	}

	return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, to)
}
