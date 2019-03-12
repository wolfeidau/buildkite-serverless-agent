package handlers

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/codebuild"
	"github.com/aws/aws-sdk-go/service/codebuild/codebuildiface"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/cwlogs"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/params"
)

// CheckJobHandler check handler
type CheckJobHandler struct {
	cfg          *config.Config
	sess         *session.Session
	paramStore   params.Store
	buildkiteAPI bk.API
	codebuildSvc codebuildiface.CodeBuildAPI
	logsReader   *cwlogs.CloudwatchLogsReader
}

// NewCheckJobHandler create a new handler
func NewCheckJobHandler(cfg *config.Config, sess *session.Session, buildkiteAPI bk.API) *CheckJobHandler {

	codebuildSvc := codebuild.New(sess)
	logsReader := cwlogs.NewCloudwatchLogsReader(cfg, cloudwatchlogs.New(sess))

	return &CheckJobHandler{
		cfg:          cfg,
		sess:         sess,
		paramStore:   params.New(cfg, sess),
		buildkiteAPI: buildkiteAPI,
		codebuildSvc: codebuildSvc,
		logsReader:   logsReader,
	}
}

// HandlerCheckJob process the step function check job event
func (ch *CheckJobHandler) HandlerCheckJob(ctx context.Context, evt *bk.WorkflowData) (*bk.WorkflowData, error) {

	logrus.Infof("%+v", evt)

	logrus.WithField("projectName", evt.CodeBuildProjectName).Info("Getting build status")

	res, err := ch.codebuildSvc.BatchGetBuilds(&codebuild.BatchGetBuildsInput{
		Ids: []*string{aws.String(evt.BuildID)},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to start codebuild job")
	}

	if len(res.Builds) != 1 {
		return nil, errors.Errorf("failed to locate build: %s", evt.BuildID)
	}

	evt.BuildStatus = aws.StringValue(res.Builds[0].BuildStatus)

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
			"projectName":     evt.CodeBuildProjectName,
			"id":              evt.BuildID,
			"CodebuildStatus": aws.StringValue(res.Builds[0].BuildStatus),
			"buildkiteStatus": jobStatus.State,
		},
	).Info("checked build")

	// if job status is canceled then we need to stop codebuild and mark the job as complete
	switch jobStatus.State {
	case "canceled":
		stopRes, err := ch.codebuildSvc.StopBuild(&codebuild.StopBuildInput{
			Id: res.Builds[0].Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to stop codebuild job")
		}

		logrus.WithFields(
			logrus.Fields{
				"projectName":     evt.CodeBuildProjectName,
				"id":              evt.BuildID,
				"CodebuildStatus": aws.StringValue(stopRes.Build.BuildStatus),
				"buildkiteStatus": jobStatus.State,
			},
		).Info("stopped canceled build")

		evt.BuildStatus = aws.StringValue(stopRes.Build.BuildStatus)
	}

	return evt, nil
}
