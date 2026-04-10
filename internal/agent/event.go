package agent

import "time"

type EventType string

const (
	EventRunCreated        EventType = "run_created"
	EventRunStarted        EventType = "run_started"
	EventStateTransition   EventType = "state_transition"
	EventLLMRequestStarted EventType = "llm_request_started"
	EventLLMRequestEnded   EventType = "llm_request_ended"
	EventLLMRequestFailed  EventType = "llm_request_failed"
	EventHumanPatched      EventType = "human_patched"
	EventBlocked           EventType = "blocked"
	EventStopped           EventType = "stopped"
	EventCancelled         EventType = "cancelled"
	EventCompleted         EventType = "completed"
)

type Event struct {
	Sequence  int64
	Type      EventType
	Time      time.Time
	FromState State
	ToState   State
	Round     int
	Message   string
	Error     string
}
