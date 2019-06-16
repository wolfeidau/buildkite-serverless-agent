package agentpool

import (
	"context"
	"time"

	"github.com/aws/aws-lambda-go/events"
	log "github.com/sirupsen/logrus"
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
	deadline = deadline.Add(-5 * time.Second)
	timeoutChannel := time.After(time.Until(deadline))

	log.Info("Loading agents")

	err := bkw.agentPool.LoadAgents()
	if err != nil {
		log.WithError(err).Error("failed to load agents from store")
	}

	log.WithField("deadline", deadline).Info("Register agents")

	err = bkw.agentPool.RegisterAgents(deadline)
	if err != nil {
		log.WithError(err).Error("failed to register agents")
	}

	log.WithField("deadline", deadline).Info("Polling agents")

LOOP:

	// loop until we are out of time, which is in our case is 60 seconds
	for {

		select {

		case <-timeoutChannel:

			log.Info("Poll agents finished")
			break LOOP

		default:

			err = bkw.agentPool.PollAgents(deadline)
			if err != nil {
				log.WithError(err).Error("failed to poll agents")
			}

			// is the current time before the deadline, if not skip the sleep
			if time.Now().Before(deadline) {
				time.Sleep(2 * time.Second)
			}
		}

	}

	deadline, _ = ctx.Deadline()

	err = bkw.agentPool.CleanupAgents(deadline)
	if err != nil {
		log.WithError(err).Error("failed to cleanup")
	}

	return nil
}
