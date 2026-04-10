package agent

import "errors"

var (
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrRunNotFound       = errors.New("run not found")
	ErrRunExists         = errors.New("run already exists")
	ErrRepository        = errors.New("repository failure")
	ErrLLMTimeout        = errors.New("llm timeout")
	ErrLLMUpstream       = errors.New("llm upstream error")
)
