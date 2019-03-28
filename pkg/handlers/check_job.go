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

// CheckJobHandler check handler
type CheckJobHandler struct {
	cfg          *config.Config
	sess         *session.Session
	paramStore   params.Store
	buildkiteAPI bk.API
	lch          codebuild.LauncherAPI
	logsReader   cwlogs.LogsReader
}

// NewCheckJobHandler create a new handler
func NewCheckJobHandler(cfg *config.Config, sess *session.Session, buildkiteAPI bk.API) *CheckJobHandler {

	config := aws.NewConfig()
	lch := service.New(config).Codebuild
	logsReader := cwlogs.NewCloudwatchLogsReader(config)

	return &CheckJobHandler{
		cfg:          cfg,
		sess:         sess,
		paramStore:   params.New(cfg, sess),
		buildkiteAPI: buildkiteAPI,
		logsReader:   logsReader,
		lch:          lch,
	}
}

// HandlerCheckJob process the step function check job event
func (ch *CheckJobHandler) HandlerCheckJob(ctx context.Context, evt *bk.WorkflowData) (*bk.WorkflowData, error) {

	logrus.Infof("%+v", evt)

	logrus.WithField("projectName", evt.Codebuild.ProjectName).Info("Getting build status")

	getStatus, err := ch.lch.GetTaskStatus(&codebuild.GetTaskStatusParams{
		ID: evt.Codebuild.BuildID,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve codebuild job status")
	}

	evt.UpdateCodebuildStatus(getStatus.ID, getStatus.BuildStatus, getStatus.TaskStatus)

	token, _, err := getBKClient(evt.AgentName, ch.cfg, ch.paramStore)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build buildkite client")
	}

	err = uploadLogChunks(token, ch.buildkiteAPI, ch.logsReader, evt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to upload log chunks")
	}

	jobStatus, err := ch.buildkiteAPI.GetStateJob(token, evt.Job.ID)
	if err != nil {
		return nil, errors.Wrap(err, "call to the buildkite api failed")
	}

	logrus.WithFields(
		logrus.Fields{
			"projectName":     evt.Codebuild.ProjectName,
			"id":              evt.Codebuild.BuildID,
			"CodebuildStatus": getStatus.BuildStatus,
			"TaskStatus":      getStatus.TaskStatus,
			"buildkiteStatus": jobStatus.State,
		},
	).Info("checked build")

	// if job status is canceled then we need to stop codebuild and mark the job as complete
	switch jobStatus.State {
	case "canceled":
		stopRes, err := ch.lch.StopTask(&codebuild.StopTaskParams{
			ID: getStatus.ID,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to stop codebuild job")
		}

		logrus.WithFields(
			logrus.Fields{
				"projectName":     evt.Codebuild.ProjectName,
				"id":              evt.Codebuild.BuildID,
				"CodebuildStatus": stopRes.BuildStatus,
				"buildkiteStatus": jobStatus.State,
			},
		).Info("stopped canceled build")

		evt.UpdateCodebuildStatus(getStatus.ID, stopRes.BuildStatus, stopRes.TaskStatus)
	}

	return evt, nil
}
