package handlers

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wolfeidau/aws-launch/pkg/cwlogs"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/store"
)

// CompletedJobHandler handler for lambda events
type CompletedJobHandler struct {
	cfg          *config.Config
	sess         *session.Session
	agentStore   store.AgentsAPI
	buildkiteAPI bk.API
	logsReader   cwlogs.LogsReader
}

// NewCompletedJobHandler create a new handler
func NewCompletedJobHandler(cfg *config.Config, sess *session.Session, buildkiteAPI bk.API) *CompletedJobHandler {

	logsReader := cwlogs.NewCloudwatchLogsReader()

	return &CompletedJobHandler{
		cfg:          cfg,
		sess:         sess,
		agentStore:   store.NewAgents(cfg),
		buildkiteAPI: buildkiteAPI,
		logsReader:   logsReader,
	}
}

// HandlerCompletedJob process the step function event for completed jobs
func (bkw *CompletedJobHandler) HandlerCompletedJob(ctx context.Context, evt *bk.WorkflowData) (*bk.WorkflowData, error) {

	agent, err := bkw.agentStore.Get(evt.AgentName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load agent from store")
	}

	err = evt.UpdateJobExitCode()
	if err != nil {
		return nil, errors.Wrap(err, "failed to update job exit code")
	}

	err = bkw.buildkiteAPI.FinishJob(agent.AgentConfig.AccessToken, evt.Job)
	if err != nil {
		return nil, errors.Wrap(err, "failed to finish job")
	}

	logrus.WithField("ID", evt.Job.ID).Info("job completed!")

	err = uploadLogChunks(agent.AgentConfig.AccessToken, bkw.buildkiteAPI, bkw.logsReader, evt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to upload log chunks")
	}

	return evt, nil
}
