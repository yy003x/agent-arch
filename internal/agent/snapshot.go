package agent

import "time"

type Message struct {
	Role    string
	Content string
	Pinned  bool
}

type AgentContext struct {
	SystemInstructions []Message
	Messages           []Message
}

type Snapshot struct {
	RunID     string
	State     State
	Round     int
	MaxRounds int
	LastError string
	Context   AgentContext
	Events    []Event
	CreatedAt time.Time
	UpdatedAt time.Time
}

func CloneSnapshot(src Snapshot) Snapshot {
	dst := src
	dst.Context = AgentContext{
		SystemInstructions: append([]Message(nil), src.Context.SystemInstructions...),
		Messages:           append([]Message(nil), src.Context.Messages...),
	}
	dst.Events = append([]Event(nil), src.Events...)
	return dst
}
