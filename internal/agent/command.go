package agent

import "time"

type RuntimeConfig struct {
	MaxRounds   int
	TokenBudget int
	LLMTimeout  time.Duration
}

type PatchOperation string

const (
	PatchAppend  PatchOperation = "append"
	PatchReplace PatchOperation = "replace"
)

type ContextPatch struct {
	Operation          PatchOperation
	SystemInstructions []Message
	Messages           []Message
}
