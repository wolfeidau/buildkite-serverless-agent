package handlers

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wolfeidau/aws-launch/pkg/launcher"
	"github.com/wolfeidau/aws-launch/pkg/launcher/codebuild"
	"github.com/wolfeidau/aws-launch/pkg/launcher/service"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/store"
)

// SubmitJobHandler submit job handler
type SubmitJobHandler struct {
	cfg          *config.Config
	agentStore   store.AgentsAPI
	buildkiteAPI bk.API
	lch          codebuild.LauncherAPI
}

// NewSubmitJobHandler create a new handler for submit job
func NewSubmitJobHandler(cfg *config.Config, buildkiteAPI bk.API) *SubmitJobHandler {

	config := aws.NewConfig()
	lch := service.New(config).Codebuild

	return &SubmitJobHandler{
		cfg:          cfg,
		agentStore:   store.NewAgents(cfg),
		buildkiteAPI: buildkiteAPI,
		lch:          lch,
	}
}

// HandlerSubmitJob process the step function submit job event
func (sh *SubmitJobHandler) HandlerSubmitJob(ctx context.Context, evt *bk.WorkflowData) (*bk.WorkflowData, error) {

	logger := sh.getLog(evt)

	logger.Info("Starting job")

	agent, err := sh.agentStore.Get(evt.AgentName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load agent from store")
	}

	err = sh.buildkiteAPI.StartJob(agent.AgentConfig.AccessToken, evt.Job)
	if err != nil {
		return nil, err
	}

	logger.Info("Starting codebuild task")

	// update the job with the agent access token
	evt.UpdateBuildJobCreds(agent.AgentConfig.AccessToken)

	// update the environment information
	evt.UpdateEnvironment(sh.cfg.EnvironmentName, sh.cfg.EnvironmentNumber)

	// start a build job
	build, err := sh.startBuildAndHandleAWSError(evt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start build in codebuild")
	}

	evt.UpdateCodebuildStatus(build.buildID, build.buildStatus, build.taskStatus)

	err = evt.UpdateCloudwatchLogs(build.buildID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to update cloudwatch logs group and stream names")
	}

	err = sh.uploadBuildMessage(agent.AgentConfig.AccessToken, build.buildMessage(), evt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to upload build message logs")
	}

	return evt, nil
}

type buildResult struct {
	buildID     string
	buildStatus string
	taskStatus  string
	headerMsg   string
}

func (br *buildResult) buildMessage() string {
	return fmt.Sprintf("--- %s\nbuild_id=%s\nbuild_status=%s\n", br.headerMsg, br.buildID, br.buildStatus)
}

func (sh *SubmitJobHandler) getLog(evt *bk.WorkflowData) *logrus.Entry {
	return logrus.WithFields(
		logrus.Fields{
			"id": evt.Job.ID,
		},
	)
}

func (sh *SubmitJobHandler) startBuildAndHandleAWSError(evt *bk.WorkflowData) (*buildResult, error) {

	startBuildInput := &codebuild.LaunchTaskParams{
		Environment:    evt.Job.Env,
		ProjectName:    evt.Codebuild.ProjectName,
		Image:          evt.GetJobEnvString("CB_IMAGE_OVERRIDE"),
		ComputeType:    evt.GetJobEnvString("CB_COMPUTE_TYPE_OVERRIDE"),
		PrivilegedMode: evt.GetJobEnvBool("CB_PRIVILEGED_MODE_OVERRIDE"),
	}

	sh.getLog(evt).WithField("params", startBuildInput).Info("LaunchTask")

	startResult, err := sh.lch.LaunchTask(startBuildInput)
	if err != nil {
		// extract the cause using pkg/errors as this may be a part of an error trace created by this library
		switch err := errors.Cause(err).(type) {
		case awserr.Error:
			// Cast err to awserr.Error and return it as a message in buildkite.
			aerr, ok := err.(awserr.Error)
			if ok {
				return &buildResult{
					buildID:     "NA:NA",
					buildStatus: launcher.TaskFailed,
					taskStatus:  launcher.TaskFailed,
					headerMsg:   fmt.Sprintf("Failed to start job in codebuild with %s", aerr.Code()),
				}, nil
			}
		default:
			// unknown error
			return nil, errors.Wrap(err, "failed to start codebuild job")
		}
	}

	return &buildResult{
		buildID:     startResult.ID,
		buildStatus: startResult.BuildStatus, // sfn is currently using codebuild statuses
		taskStatus:  startResult.TaskStatus,  // moving to the new normalised task statuses
		headerMsg:   "Started a job in codebuild on :aws:",
	}, nil
}

func (sh *SubmitJobHandler) uploadBuildMessage(token, msg string, evt *bk.WorkflowData) error {
	err := sh.buildkiteAPI.ChunksUpload(token, evt.Job.ID, &api.Chunk{
		Data:     msg,
		Sequence: evt.LogSequence,
		Offset:   evt.LogBytes,
		Size:     len(msg),
	})
	if err != nil {
		return err
	}

	// increment everything
	evt.LogSequence++
	evt.LogBytes += len(msg)

	return nil
}
