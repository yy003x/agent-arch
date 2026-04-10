package contextx

import "github.com/young/go/agent-arch/internal/agent"

type TokenEstimator interface {
	EstimateMessages(messages []agent.Message) int
}

type ApproximateEstimator struct{}

func (ApproximateEstimator) EstimateMessages(messages []agent.Message) int {
	total := 0
	for _, msg := range messages {
		total += 4 + len(msg.Content)/4
	}
	return total
}
