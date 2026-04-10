package llm

import "errors"

var ErrUpstream = errors.New("llm upstream error")

type Message struct {
	Role    string
	Content string
	Pinned  bool
}

type Request struct {
	RunID    string
	Round    int
	Messages []Message
}

type Response struct {
	Message Message
	Done    bool
}
