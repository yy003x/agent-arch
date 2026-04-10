package contextx

import "github.com/young/go/agent-arch/internal/agent"

func TruncateMessages(system []agent.Message, messages []agent.Message, budget int, estimator TokenEstimator) []agent.Message {
	base := make([]agent.Message, 0, len(system)+len(messages))
	base = append(base, system...)

	pinned := make([]agent.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Pinned {
			pinned = append(pinned, msg)
		}
	}
	base = append(base, pinned...)

	recent := make([]agent.Message, 0, len(messages))
	start := len(messages) - 1
	for start >= 0 {
		msg := messages[start]
		if msg.Pinned {
			start--
			continue
		}

		candidate := make([]agent.Message, 0, len(base)+len(recent)+1)
		candidate = append(candidate, base...)
		candidate = append(candidate, msg)
		candidate = append(candidate, recent...)
		if estimator.EstimateMessages(candidate) > budget {
			break
		}
		recent = append([]agent.Message{msg}, recent...)
		start--
	}

	result := make([]agent.Message, 0, len(base)+len(recent))
	result = append(result, base...)
	result = append(result, recent...)
	return result
}
