package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
)

// BuildkiteWorker handler for lambda events
type BuildkiteWorker struct {
	cfg    *config.Config
	sfnSvc sfniface.SFNAPI
}

// New create a new handler
func New(cfg *config.Config, sess *session.Session) *BuildkiteWorker {
	sfnSvc := sfn.New(sess)

	return &BuildkiteWorker{
		cfg:    cfg,
		sfnSvc: sfnSvc,
	}
}

// Handler process the cloudwatch scheduled event
func (bkw *BuildkiteWorker) Handler(ctx context.Context, evt *events.CloudWatchEvent) error {
	log.Info("trigger agent poll")

	count, err := bkw.countRunningStateMachines(bkw.cfg.SfnAgentPollerArn)
	if err != nil {
		return err
	}

	// only allow one execution
	if count >= 1 {
		return nil
	}

	return bkw.startStateMachines(bkw.cfg.SfnAgentPollerArn)
}

func (bkw *BuildkiteWorker) countRunningStateMachines(name string) (int, error) {
	listResult, err := bkw.sfnSvc.ListExecutions(&sfn.ListExecutionsInput{
		StateMachineArn: aws.String(name),
		StatusFilter:    aws.String(sfn.ExecutionStatusRunning),
	})
	if err != nil {
		return 0, errors.Wrap(err, "failed to locate step function")
	}

	log.WithFields(log.Fields{
		"total": len(listResult.Executions),
	}).Info("Running executions")

	return len(listResult.Executions), nil
}

func (bkw *BuildkiteWorker) startStateMachines(name string) error {

	execName := fmt.Sprintf("run_%s", time.Now().Format("2006-01-02T150405Z"))

	execResult, err := bkw.sfnSvc.StartExecution(&sfn.StartExecutionInput{
		StateMachineArn: aws.String(name),
		Input:           aws.String(`{}`),
		Name:            aws.String(execName),
	})
	if err != nil {
		return errors.Wrap(err, "failed to exec step function")
	}

	log.WithFields(log.Fields{
		"Name":         execName,
		"ExecutionArn": aws.StringValue(execResult.ExecutionArn),
	}).Info("started execution")

	return nil

}
