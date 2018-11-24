package agentpool

import (
	"context"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/sirupsen/logrus"
)

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
func (bkw *BuildkiteWorker) Handler(ctx context.Context, evt *events.CloudWatchEvent) error {

	deadline, _ := ctx.Deadline()
	deadline = deadline.Add(-3 * time.Second)
	timeoutChannel := time.After(time.Until(deadline))

	logrus.WithField("deadline", deadline).Info("Polling agents")

LOOP:

	// loop until we are out of time, which is in our case is 60 seconds
	for {

		select {

		case <-timeoutChannel:

			logrus.Info("Poll agents finished")
			break LOOP

		default:

			err := bkw.agentPool.PollAgents()
			if err != nil {
				logrus.WithError(err).Error("failed to poll")
			}

			time.Sleep(2 * time.Second)
		}

	}

	return nil
}
