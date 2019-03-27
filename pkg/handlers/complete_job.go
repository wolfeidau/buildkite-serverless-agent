package handlers

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wolfeidau/aws-launch/pkg/cwlogs"
	"github.com/wolfeidau/aws-launch/pkg/launcher/codebuild"
	"github.com/wolfeidau/aws-launch/pkg/launcher/service"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/params"
)

// CompletedJobHandler handler for lambda events
type CompletedJobHandler struct {
	cfg          *config.Config
	sess         *session.Session
	paramStore   params.Store
	buildkiteAPI bk.API
	logsReader   cwlogs.LogsReader
	lch          codebuild.LauncherAPI
}

// NewCompletedJobHandler create a new handler
func NewCompletedJobHandler(cfg *config.Config, sess *session.Session, buildkiteAPI bk.API) *CompletedJobHandler {

	logsReader := cwlogs.NewCloudwatchLogsReader()
	config := aws.NewConfig()
	lch := service.New(config).Codebuild

	return &CompletedJobHandler{
		cfg:          cfg,
		sess:         sess,
		paramStore:   params.New(cfg, sess),
		buildkiteAPI: buildkiteAPI,
		logsReader:   logsReader,
		lch:          lch,
	}
}

// HandlerCompletedJob process the step function event for completed jobs
func (bkw *CompletedJobHandler) HandlerCompletedJob(ctx context.Context, evt *bk.WorkflowData) (*bk.WorkflowData, error) {

	token, _, err := getBKClient(evt.AgentName, bkw.cfg, bkw.paramStore)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build buildkite client")
	}

	err = evt.UpdateJobExitCode()
	if err != nil {
		return nil, errors.Wrap(err, "failed to update job exit code")
	}

	err = bkw.buildkiteAPI.FinishJob(token, evt.Job)
	if err != nil {
		return nil, errors.Wrap(err, "failed to finish job")
	}

	logrus.WithField("ID", evt.Job.ID).Info("job completed!")

	err = uploadLogChunks(token, bkw.buildkiteAPI, bkw.logsReader, evt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to upload log chunks")
	}

	// are we using the 2.x model of projects on demand?
	if bkw.cfg.DefineAndStart == "true" {
		_, err = bkw.lch.CleanupTask(&codebuild.CleanupTaskParams{
			ProjectName: evt.Codebuild.ProjectName,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to cleanup task")
		}
	}

	return evt, nil
}
