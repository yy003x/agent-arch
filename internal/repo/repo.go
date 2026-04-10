package repo

import "github.com/young/go/agent-arch/internal/agent"

func clone(snapshot agent.Snapshot) agent.Snapshot {
	return agent.CloneSnapshot(snapshot)
}
