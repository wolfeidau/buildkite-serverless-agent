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

// MaxPipelineSlugLength Maximum characters to take from the pipeline slug value before it is truncated
const MaxPipelineSlugLength = 32
const MaxAgentNameLength = 28

// used to inject a static time for testing
var nowFunc = time.Now

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

	// ppc-ping-stack-pipeline-deploy-s_serverless-agent-sandpit-1_2_2019-01-04T003421Z

	// truncate the pipeline slug if longer than MaxPipelineSlugLength
	if len(pipelineSlug) > MaxPipelineSlugLength {
		pipelineSlug = pipelineSlug[0:MaxPipelineSlugLength]
	}

	// truncate the agent name if longer than MaxAgentNameLength
	if len(agentName) > MaxAgentNameLength {
		agentName = agentName[0:MaxAgentNameLength]
	}

	execName := fmt.Sprintf("%s_%s_%s", pipelineSlug, agentName, nowFunc().Format("2006-01-02T150405Z"))

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
