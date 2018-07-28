package agentpool

import (
	"context"

	"github.com/sirupsen/logrus"
)

// PollerData used to track iterations in the poller
type PollerData struct {
	Index    int  `json:"index"`
	Continue bool `json:"continue"`
	Count    int  `json:"count"`
}

// BuildkiteWorker handler for lambda events
type BuildkiteWorker struct {
	agentPool *AgentPool
}

// NewBuildkiteWorker create a new handler
func NewBuildkiteWorker(agentPool *AgentPool) *BuildkiteWorker {
	return &BuildkiteWorker{
		agentPool: agentPool,
	}
}

// Handler process the cloudwatch scheduled event
func (bkw *BuildkiteWorker) Handler(ctx context.Context, evt *PollerData) (*PollerData, error) {
	logrus.Info("Poll agents")

	evt.Index++
	evt.Continue = evt.Index < evt.Count

	return evt, bkw.agentPool.PollAgents()
}
