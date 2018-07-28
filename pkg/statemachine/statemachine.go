package statemachine

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
)

// Executor run background jobs to track a build job
type Executor interface {
	RunningForAgent(agentName string) (int, error)
	StartExecution(agentName string, job *api.Job, jsonData []byte) error
}

// SFNExecutor run jobs in step functions
type SFNExecutor struct {
	cfg    *config.Config
	sfnSvc sfniface.SFNAPI
}

// NewSFNExecutor create a new step function executor
func NewSFNExecutor(cfg *config.Config, sess *session.Session) *SFNExecutor {
	sfnSvc := sfn.New(sess)
	return &SFNExecutor{
		cfg:    cfg,
		sfnSvc: sfnSvc,
	}
}

// RunningForAgent can we run anymore jobs for a given agent with a max of 1 concurrent job per agent
func (sfne *SFNExecutor) RunningForAgent(agentName string) (int, error) {

	listResult, err := sfne.sfnSvc.ListExecutions(&sfn.ListExecutionsInput{
		StateMachineArn: aws.String(sfne.cfg.SfnCodebuildJobMonitorArn),
		StatusFilter:    aws.String(sfn.ExecutionStatusRunning),
	})
	if err != nil {
		return 0, errors.Wrap(err, "failed to locate step function")
	}

	i := 0

	for _, exec := range listResult.Executions {
		if strings.Contains(aws.StringValue(exec.Name), agentName) {
			i++
		}
	}

	logrus.WithFields(logrus.Fields{
		"total":     len(listResult.Executions),
		"agent":     i,
		"agentName": agentName,
	}).Info("Running executions")

	return i, nil
}

// StartExecution start a step function execution
func (sfne *SFNExecutor) StartExecution(agentName string, job *api.Job, jsonData []byte) error {

	pipelineSlug := job.Env["BUILDKITE_PIPELINE_SLUG"]

	// first 60 characters of the pipeline slug
	if len(pipelineSlug) > 60 {
		pipelineSlug = pipelineSlug[0:60]
	}

	execName := fmt.Sprintf("%s_%s_%s", pipelineSlug, agentName, time.Now().Format("2006-01-02T150405Z"))

	execResult, err := sfne.sfnSvc.StartExecution(&sfn.StartExecutionInput{
		StateMachineArn: aws.String(sfne.cfg.SfnCodebuildJobMonitorArn),
		Input:           aws.String(string(jsonData)),
		Name:            aws.String(execName),
	})
	if err != nil {
		return errors.Wrap(err, "failed to exec step function")
	}

	logrus.WithFields(logrus.Fields{
		"ID":           job.ID,
		"Name":         execName,
		"ExecutionArn": aws.StringValue(execResult.ExecutionArn),
	}).Info("started execution")

	return nil
}
