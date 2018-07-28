package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/codebuild"
	"github.com/aws/aws-sdk-go/service/codebuild/codebuildiface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/buildkite/agent/agent"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/cwlogs"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/params"
)

const codebuildProjectPrefix = "BuildkiteProject"

// BuildkiteSFNWorker handler for lambda events
type BuildkiteSFNWorker struct {
	cfg          *config.Config
	sess         *session.Session
	paramStore   *params.SSMStore
	codebuildSvc codebuildiface.CodeBuildAPI
}

// New create a new handler
func New(cfg *config.Config, sess *session.Session) *BuildkiteSFNWorker {
	ssmSvc := ssm.New(sess)
	codebuildSvc := codebuild.New(sess)

	return &BuildkiteSFNWorker{
		cfg:          cfg,
		sess:         sess,
		codebuildSvc: codebuildSvc,
		paramStore:   params.New(cfg, ssmSvc),
	}
}

// HandlerSubmitJob process the step function submit job event
func (bkw *BuildkiteSFNWorker) HandlerSubmitJob(ctx context.Context, evt *bk.WorkflowData) (*bk.WorkflowData, error) {

	projectName := fmt.Sprintf("%s-%s-%s", codebuildProjectPrefix, bkw.cfg.EnvironmentName, bkw.cfg.EnvironmentNumber)

	logger := logrus.WithFields(
		logrus.Fields{
			"projectName": projectName,
			"id":          evt.Job.ID,
		},
	)

	logger.Info("Starting job")

	client, agentConfig, err := bkw.getBKClient(evt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build buildkite client")
	}

	evt.Job.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)

	res, err := client.Jobs.Start(evt.Job)
	if err != nil {
		return nil, err
	}

	// we failed to start the job
	if res.StatusCode > 299 {
		return nil, errors.Errorf("failed to start job, returned status: %s", res.Status)
	}

	logger.Info("Starting codebuild task")

	codebuildEnv := convertEnvVars(evt.Job.Env)

	// merge in the agent endpoint so it can send back git information to buildkite
	codebuildEnv = append(codebuildEnv, &codebuild.EnvironmentVariable{
		Name:  aws.String("BUILDKITE_AGENT_ENDPOINT"),
		Value: aws.String(evt.Job.Endpoint),
	})

	// merge in the agent access token so it can send back git information to buildkite
	codebuildEnv = append(codebuildEnv, &codebuild.EnvironmentVariable{
		Name:  aws.String("BUILDKITE_AGENT_ACCESS_TOKEN"),
		Value: aws.String(agentConfig.AccessToken),
	})

	ov := overrides{logger: logger, env: evt.Job.Env}

	startBuildInput := &codebuild.StartBuildInput{
		ProjectName:                  aws.String(projectName),
		EnvironmentVariablesOverride: codebuildEnv,
		ImageOverride:                ov.String("CB_IMAGE_OVERRIDE"),
		ComputeTypeOverride:          ov.String("CB_COMPUTE_TYPE_OVERRIDE"),
		PrivilegedModeOverride:       ov.Bool("CB_PRIVILEGED_MODE_OVERRIDE"),
	}

	startResult, err := bkw.codebuildSvc.StartBuild(startBuildInput)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start codebuild job")
	}

	logger.WithFields(
		logrus.Fields{
			"build_id":     aws.StringValue(startResult.Build.Id),
			"build_status": aws.StringValue(startResult.Build.BuildStatus),
		},
	).Info("Started build")

	evt.BuildID = aws.StringValue(startResult.Build.Id)
	evt.BuildStatus = aws.StringValue(startResult.Build.BuildStatus)
	evt.WaitTime = 10

	return evt, nil
}

// HandlerCheckJob process the step function check job event
func (bkw *BuildkiteSFNWorker) HandlerCheckJob(ctx context.Context, evt *bk.WorkflowData) (*bk.WorkflowData, error) {

	logrus.Infof("%+v", evt)

	projectName := fmt.Sprintf("%s-%s-%s", codebuildProjectPrefix, bkw.cfg.EnvironmentName, bkw.cfg.EnvironmentNumber)

	logrus.WithField("projectName", projectName).Info("Getting build status")

	res, err := bkw.codebuildSvc.BatchGetBuilds(&codebuild.BatchGetBuildsInput{
		Ids: []*string{aws.String(evt.BuildID)},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to start codebuild job")
	}

	if len(res.Builds) != 1 {
		return nil, errors.Errorf("failed to locate build: %s", evt.BuildID)
	}

	evt.BuildStatus = aws.StringValue(res.Builds[0].BuildStatus)

	err = bkw.uploadLogChunks(evt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to upload log chunks")
	}

	client, _, err := bkw.getBKClient(evt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build buildkite client")
	}

	jobStatus, _, err := client.Jobs.GetState(evt.Job.ID)
	if err != nil {
		return nil, errors.Wrap(err, "call to the buildkite api failed")
	}

	logrus.WithFields(
		logrus.Fields{
			"projectName":     projectName,
			"id":              evt.BuildID,
			"CodebuildStatus": aws.StringValue(res.Builds[0].BuildStatus),
			"buildkiteStatus": jobStatus.State,
		},
	).Info("checked build")

	return evt, nil
}

// CompletedJobHandler process the step function event for completed jobs
func (bkw *BuildkiteSFNWorker) CompletedJobHandler(ctx context.Context, evt *bk.WorkflowData) (*bk.WorkflowData, error) {

	client, _, err := bkw.getBKClient(evt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build buildkite client")
	}

	evt.Job.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)

	switch evt.BuildStatus {
	case codebuild.StatusTypeFailed:
		evt.Job.ExitStatus = "-2"
	case codebuild.StatusTypeSucceeded:
		evt.Job.ExitStatus = "0"
	default:
		logrus.WithField("build_status", evt.BuildStatus).Error("Codebuild Job failed.")
		evt.Job.ExitStatus = "-4"
	}

	evt.Job.ChunksFailedCount = 0

	res, err := client.Jobs.Finish(evt.Job)
	if err != nil {
		return nil, errors.Wrap(err, "failed to finish job")
	}

	if res.StatusCode == 422 {
		return nil, errors.Errorf("Buildkite rejected the call to finish the job (%s)", res.Status)
	}

	logrus.WithField("ID", evt.Job.ID).Info("job completed!")

	err = bkw.uploadLogChunks(evt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to upload log chunks")
	}

	return evt, nil
}

func (bkw *BuildkiteSFNWorker) uploadLogChunks(evt *bk.WorkflowData) error {

	client, _, err := bkw.getBKClient(evt)
	if err != nil {
		return errors.Wrap(err, "failed to build buildkite client")
	}

	logrus.WithFields(logrus.Fields{
		"BuildID":   evt.BuildID,
		"NextToken": evt.NextToken,
	}).Info("ReadLogs")

	logsReader := cwlogs.NewCloudwatchLogsReader(bkw.cfg, cloudwatchlogs.New(bkw.sess))

	nextToken, pageData, err := logsReader.ReadLogs(evt.BuildID, evt.NextToken)
	if err != nil {
		return errors.Wrap(err, "failed to finish job")
	}

	logrus.WithFields(logrus.Fields{
		"BuildID":   evt.BuildID,
		"ChunkLen":  len(pageData),
		"NextToken": nextToken,
	}).Info("ReadLogs Complete")

	if len(pageData) > 0 {

		buf := bytes.NewBuffer(pageData)

		p := make([]byte, evt.Job.ChunksMaxSizeBytes)
		for {
			n, err := buf.Read(p)
			if err == io.EOF {
				break
			}

			logrus.WithFields(logrus.Fields{
				"BuildID": evt.BuildID,
				"n":       n,
			}).Info("Read Chunk")

			res, err := client.Chunks.Upload(evt.Job.ID, &api.Chunk{
				Data:     string(p[:n]),
				Sequence: evt.LogSequence,
				Offset:   evt.LogBytes,
				Size:     n,
			})
			if err != nil {
				return errors.Wrap(err, "failed to write chunk to buildkite API")
			}

			logrus.WithFields(logrus.Fields{
				"BuildID":     evt.BuildID,
				"Status":      res.Status,
				"LogSequence": evt.LogSequence,
				"LogBytes":    evt.LogBytes,
			}).Info("Wrote Chunk")

			// increment everything
			evt.LogSequence++
			evt.LogBytes += n
		}

	}

	evt.NextToken = nextToken

	return nil
}

func (bkw *BuildkiteSFNWorker) getBKClient(evt *bk.WorkflowData) (*api.Client, *api.Agent, error) {
	agentSSMConfigKey := fmt.Sprintf("/%s/%s/%s", bkw.cfg.EnvironmentName, bkw.cfg.EnvironmentNumber, evt.AgentName)

	agentConfig, err := bkw.paramStore.GetAgentConfig(agentSSMConfigKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to load agent configuration")
	}
	return agent.APIClient{Endpoint: bk.DefaultAPIEndpoint, Token: agentConfig.AccessToken}.Create(), agentConfig, nil
}

func convertEnvVars(env map[string]string) []*codebuild.EnvironmentVariable {

	codebuildEnv := make([]*codebuild.EnvironmentVariable, 0)

	for k, v := range env {
		codebuildEnv = append(codebuildEnv, &codebuild.EnvironmentVariable{
			Name:  aws.String(k),
			Value: aws.String(v),
		})
	}

	return codebuildEnv
}

type overrides struct {
	logger logrus.FieldLogger
	env    map[string]string
}

func (ov *overrides) String(key string) *string {
	if val, ok := ov.env[key]; ok {
		ov.logger.Infof("updating %s to %s", key, val)
		return aws.String(val)
	}

	return nil
}

func (ov *overrides) Bool(key string) *bool {
	if val, ok := ov.env[key]; !ok {

		b, err := strconv.ParseBool(val)
		if err != nil {
			ov.logger.Warnf("failed to update %s to %s", key, val)
			return nil
		}

		ov.logger.Infof("updating %s to %t", key, b)
		return aws.Bool(b)
	}

	return nil
}
