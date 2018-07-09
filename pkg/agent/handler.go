package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/buildkite/agent/agent"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/params"
)

// BuildkiteWorker handler for lambda events
type BuildkiteWorker struct {
	cfg    *config.Config
	ssmSvc ssmiface.SSMAPI
	sfnSvc sfniface.SFNAPI
}

// New create a new handler
func New(cfg *config.Config, sess *session.Session) *BuildkiteWorker {
	ssmSvc := ssm.New(sess)
	sfnSvc := sfn.New(sess)

	return &BuildkiteWorker{
		cfg:    cfg,
		ssmSvc: ssmSvc,
		sfnSvc: sfnSvc,
	}
}

// Handler process the cloudwatch scheduled event
func (bkw *BuildkiteWorker) Handler(ctx context.Context, evt *events.CloudWatchEvent) error {

	agentConfig, err := params.New(bkw.cfg, bkw.ssmSvc).GetAgentConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load agent configuration")
	}

	client := agent.APIClient{Endpoint: bk.DefaultAPIEndpoint, Token: agentConfig.AccessToken}.Create()

	beat, _, err := client.Heartbeats.Beat()
	if err != nil {
		return errors.Wrap(err, "failed to register agent")
	}

	logrus.Infof("Heartbeat sent at %s and received at %s", beat.SentAt, beat.ReceivedAt)

	listResult, err := bkw.sfnSvc.ListExecutions(&sfn.ListExecutionsInput{
		StateMachineArn: aws.String(bkw.cfg.SfnCodebuildJobMonitorArn),
		StatusFilter:    aws.String(sfn.ExecutionStatusRunning),
	})
	if err != nil {
		return errors.Wrap(err, "failed to locate step function")
	}

	// we we running any jobs at the moment?
	if len(listResult.Executions) >= bkw.cfg.GetConcurrentBuilds() {

		logrus.Infof("Running %d executions so not retrieving a job", len(listResult.Executions))
		return nil // we are done as there is already a job running
	}

	ping, _, err := client.Pings.Get()
	if err != nil {
		return errors.Wrap(err, "failed to ping endpoint")
	}

	if ping.Job == nil {
		logrus.Info("Ping to endpoint returned no job")

		return nil // we are done
	}

	job, _, err := client.Jobs.Accept(ping.Job)
	if err != nil {
		return errors.Wrap(err, "failed to accept job from endpoint")
	}

	wd := &bk.WorkflowData{Job: job}

	data, err := json.Marshal(wd)
	if err != nil {
		return errors.Wrap(err, "failed to marshal job")
	}

	execName := fmt.Sprintf("%s_%d", job.ID, time.Now().Unix())

	execResult, err := bkw.sfnSvc.StartExecution(&sfn.StartExecutionInput{
		StateMachineArn: aws.String(bkw.cfg.SfnCodebuildJobMonitorArn),
		Input:           aws.String(string(data)),
		Name:            aws.String(execName),
	})
	if err != nil {
		return errors.Wrap(err, "failed to exec step function")
	}

	logrus.WithFields(logrus.Fields{
		"ID":           job.ID,
		"ExecutionArn": aws.StringValue(execResult.ExecutionArn),
	}).Info("started execution")

	return nil
}
