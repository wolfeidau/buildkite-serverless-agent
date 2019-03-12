package handlers

import (
	"context"
	"time"

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

// CompletedJobHandler handler for lambda events
type CompletedJobHandler struct {
	cfg          *config.Config
	sess         *session.Session
	paramStore   params.Store
	buildkiteAPI bk.API
	codebuildSvc codebuildiface.CodeBuildAPI
	logsReader   *cwlogs.CloudwatchLogsReader
}

// NewCompletedJobHandler create a new handler
func NewCompletedJobHandler(cfg *config.Config, sess *session.Session, buildkiteAPI bk.API) *CompletedJobHandler {

	codebuildSvc := codebuild.New(sess)
	logsReader := cwlogs.NewCloudwatchLogsReader(cfg, cloudwatchlogs.New(sess))

	return &CompletedJobHandler{
		cfg:          cfg,
		sess:         sess,
		paramStore:   params.New(cfg, sess),
		buildkiteAPI: buildkiteAPI,
		codebuildSvc: codebuildSvc,
		logsReader:   logsReader,
	}
}

// HandlerCompletedJob process the step function event for completed jobs
func (bkw *CompletedJobHandler) HandlerCompletedJob(ctx context.Context, evt *bk.WorkflowData) (*bk.WorkflowData, error) {

	token, _, err := getBKClient(evt.AgentName, bkw.cfg, bkw.paramStore)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build buildkite client")
	}

	evt.Job.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)

	switch evt.BuildStatus {
	case codebuild.StatusTypeStopped:
		evt.Job.ExitStatus = "-3"
	case codebuild.StatusTypeFailed:
		evt.Job.ExitStatus = "-2"
	case codebuild.StatusTypeSucceeded:
		evt.Job.ExitStatus = "0"
	default:
		logrus.WithField("build_status", evt.BuildStatus).Error("Codebuild Job failed.")
		evt.Job.ExitStatus = "-4"
	}

	evt.Job.ChunksFailedCount = 0

	err = bkw.buildkiteAPI.FinishJob(token, evt.Job)
	if err != nil {
		return nil, errors.Wrap(err, "failed to finish job")
	}

	logrus.WithField("ID", evt.Job.ID).Info("job completed!")

	err = uploadLogChunks(token, bkw.buildkiteAPI, bkw.logsReader, evt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to upload log chunks")
	}

	return evt, nil
}
