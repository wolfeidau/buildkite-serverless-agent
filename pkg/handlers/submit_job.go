package handlers

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wolfeidau/aws-launch/pkg/launcher"
	"github.com/wolfeidau/aws-launch/pkg/launcher/codebuild"
	"github.com/wolfeidau/aws-launch/pkg/launcher/service"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/params"
)

// SubmitJobHandler submit job handler
type SubmitJobHandler struct {
	cfg          *config.Config
	paramStore   params.Store
	buildkiteAPI bk.API
	lch          codebuild.LauncherAPI
}

// NewSubmitJobHandler create a new handler for submit job
func NewSubmitJobHandler(cfg *config.Config, buildkiteAPI bk.API) *SubmitJobHandler {

	sess := session.Must(session.NewSession())
	config := aws.NewConfig()
	lch := service.New(config).Codebuild

	return &SubmitJobHandler{
		cfg:          cfg,
		paramStore:   params.New(cfg, sess),
		buildkiteAPI: buildkiteAPI,
		lch:          lch,
	}
}

// HandlerSubmitJob process the step function submit job event
func (sh *SubmitJobHandler) HandlerSubmitJob(ctx context.Context, evt *bk.WorkflowData) (*bk.WorkflowData, error) {

	logger := sh.getLog(evt)

	logger.Info("Starting job")

	token, agentConfig, err := getBKClient(evt.AgentName, sh.cfg, sh.paramStore)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build buildkite client")
	}

	err = sh.buildkiteAPI.StartJob(token, evt.Job)
	if err != nil {
		return nil, err
	}

	logger.Info("Starting codebuild task")

	// update the job with the agent access token
	evt.UpdateBuildJobCreds(agentConfig.AccessToken)

	// are we using the 2.x model of projects on demand?
	if sh.cfg.DefineAndStart == "true" {

		err = sh.defineCodebuildJob(evt)
		if err != nil {
			return nil, errors.Wrap(err, "failed to define job in codebuild")
		}

	} else {
		// assign the project name to the workflow and apply overrides from ENV vars
		evt.UpdateCodebuildProject(sh.getProjectName())
	}

	// start a build job
	build, err := sh.startBuildAndHandleAWSError(evt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start build in codebuild")
	}

	err = evt.UpdateCodebuildStatus(build.buildID, build.buildStatus, build.taskStatus)
	if err != nil {
		return nil, errors.Wrap(err, "failed to update event with codebuild info")
	}

	err = sh.uploadBuildMessage(token, build.buildMessage(), evt)
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

func (sh *SubmitJobHandler) getProjectName() string {
	return fmt.Sprintf("%s-%s-%s", codebuildProjectPrefix, sh.cfg.EnvironmentName, sh.cfg.EnvironmentNumber)
}
func (sh *SubmitJobHandler) getLog(evt *bk.WorkflowData) *logrus.Entry {
	return logrus.WithFields(
		logrus.Fields{
			"id": evt.Job.ID,
		},
	)
}

func (sh *SubmitJobHandler) defineCodebuildJob(evt *bk.WorkflowData) error {

	// TODO: Maximum length of 255
	evt.Codebuild = &bk.CodebuildWorkflowData{ProjectName: evt.GetBuildkitePipelineEnvString()}

	defParams := &codebuild.DefineTaskParams{
		ProjectName:    evt.Codebuild.ProjectName,
		ComputeType:    "BUILD_GENERAL1_SMALL",
		Image:          sh.cfg.DefaultDockerImage,
		Region:         sh.cfg.AwsRegion,
		ServiceRole:    sh.cfg.DefaultCodebuildProjectRole,
		Buildspec:      config.DefaultBuildSpec,
		PrivilegedMode: aws.Bool(true),
		Environment: map[string]string{
			"ENVIRONMENT_NAME":   sh.cfg.EnvironmentName,
			"ENVIRONMENT_NUMBER": sh.cfg.EnvironmentNumber,
		},
		Tags: map[string]string{
			"EnvironmentName":   sh.cfg.EnvironmentName,
			"EnvironmentNumber": sh.cfg.EnvironmentNumber,
			"CreatedBy":         "https://github.com/wolfeidau/buildkite-serverless-agent",
		},
	}

	sh.getLog(evt).WithField("params", defParams).Info("DefineTask")

	defRes, err := sh.lch.DefineTask(defParams)
	if err != nil {
		return errors.Wrap(err, "failed to define project in codebuild")
	}

	evt.Codebuild.LogGroupName = defRes.CloudwatchLogGroupName
	evt.Codebuild.LogStreamPrefix = defRes.CloudwatchStreamPrefix

	return nil
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
