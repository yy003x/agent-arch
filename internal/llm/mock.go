package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type MockClient struct {
	Latency time.Duration
}

func NewMockClient(latency time.Duration) *MockClient {
	return &MockClient{Latency: latency}
}

func (m *MockClient) Generate(ctx context.Context, req Request) (Response, error) {
	if m.Latency <= 0 {
		m.Latency = 1500 * time.Millisecond
	}

	select {
	case <-time.After(m.Latency):
	case <-ctx.Done():
		return Response{}, ctx.Err()
	}

	last := ""
	if len(req.Messages) > 0 {
		last = strings.ToLower(req.Messages[len(req.Messages)-1].Content)
	}

	switch {
	case strings.Contains(last, "timeout"):
		<-ctx.Done()
		return Response{}, ctx.Err()
	case strings.Contains(last, "fail"):
		return Response{}, fmt.Errorf("%w: mock upstream rejected prompt", ErrUpstream)
	case strings.Contains(last, "cancel"):
		return Response{}, context.Canceled
	}

	done := req.Round >= 1
	return Response{
		Message: Message{
			Role:    "assistant",
			Content: fmt.Sprintf("round %d processed %d messages", req.Round+1, len(req.Messages)),
		},
		Done: done,
	}, nil
}

func IsTimeout(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}

func IsCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}
