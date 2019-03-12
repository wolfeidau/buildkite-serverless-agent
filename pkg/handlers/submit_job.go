package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codebuild"
	"github.com/aws/aws-sdk-go/service/codebuild/codebuildiface"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/params"
)

// SubmitJobHandler submit job handler
type SubmitJobHandler struct {
	cfg          *config.Config
	paramStore   params.Store
	buildkiteAPI bk.API
	codebuildSvc codebuildiface.CodeBuildAPI
}

// NewSubmitJobHandler create a new handler for submit job
func NewSubmitJobHandler(cfg *config.Config, buildkiteAPI bk.API) *SubmitJobHandler {

	sess := session.Must(session.NewSession())
	codebuildSvc := codebuild.New(sess)

	return &SubmitJobHandler{
		cfg:          cfg,
		paramStore:   params.New(cfg, sess),
		buildkiteAPI: buildkiteAPI,
		codebuildSvc: codebuildSvc,
	}
}

// HandlerSubmitJob process the step function submit job event
func (sh *SubmitJobHandler) HandlerSubmitJob(ctx context.Context, evt *bk.WorkflowData) (*bk.WorkflowData, error) {

	projectName := fmt.Sprintf("%s-%s-%s", codebuildProjectPrefix, sh.cfg.EnvironmentName, sh.cfg.EnvironmentNumber)

	logger := logrus.WithFields(
		logrus.Fields{
			"projectName": projectName,
			"id":          evt.Job.ID,
		},
	)

	logger.Info("Starting job")

	token, agentConfig, err := getBKClient(evt.AgentName, sh.cfg, sh.paramStore)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build buildkite client")
	}

	evt.Job.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)

	err = sh.buildkiteAPI.StartJob(token, evt.Job)
	if err != nil {
		return nil, err
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

	if codebuildProj := ov.String("CB_PROJECT_NAME"); codebuildProj != nil {
		projectName = aws.StringValue(codebuildProj)
	}

	startBuildInput := &codebuild.StartBuildInput{
		ProjectName:                  aws.String(projectName),
		EnvironmentVariablesOverride: codebuildEnv,
		ImageOverride:                ov.String("CB_IMAGE_OVERRIDE"),
		ComputeTypeOverride:          ov.String("CB_COMPUTE_TYPE_OVERRIDE"),
		PrivilegedModeOverride:       ov.Bool("CB_PRIVILEGED_MODE_OVERRIDE"),
	}

	build, err := sh.startBuild(startBuildInput)
	if err != nil {
		return nil, err
	}

	logger.WithFields(
		logrus.Fields{
			"build_id":     build.buildID,
			"build_status": build.buildStatus,
		},
	).Info("Started build")

	evt.BuildID = build.buildID
	evt.BuildStatus = build.buildStatus
	evt.WaitTime = 10
	evt.CodeBuildProjectName = build.buildProjectName

	msg := fmt.Sprintf("--- %s\nbuild_project=%s\nbuild_id=%s\nbuild_status=%s\n", build.headerMsg, build.buildProjectName, build.buildID, build.buildStatus)

	err = sh.buildkiteAPI.ChunksUpload(token, evt.Job.ID, &api.Chunk{
		Data:     msg,
		Sequence: evt.LogSequence,
		Offset:   evt.LogBytes,
		Size:     len(msg),
	})
	if err != nil {
		return nil, err
	}

	// increment everything
	evt.LogSequence++
	evt.LogBytes += len(msg)

	return evt, nil
}

type buildResult struct {
	buildID          string
	buildStatus      string
	buildProjectName string
	headerMsg        string
}

func (sh *SubmitJobHandler) startBuild(startBuildInput *codebuild.StartBuildInput) (*buildResult, error) {
	// Returned Error Codes:
	//   * ErrCodeInvalidInputException "InvalidInputException"
	//   The input value that was provided is not valid.
	//
	//   * ErrCodeResourceNotFoundException "ResourceNotFoundException"
	//   The specified AWS resource cannot be found.
	//
	//   * ErrCodeAccountLimitExceededException "AccountLimitExceededException"
	//   An AWS service limit was exceeded for the calling AWS account.
	startResult, err := sh.codebuildSvc.StartBuild(startBuildInput)
	if err != nil {
		// Cast err to awserr.Error and return it as a message in buildkite.
		aerr, ok := err.(awserr.Error)
		if ok {
			return &buildResult{
				buildID:          "NA",
				buildStatus:      codebuild.StatusTypeFailed,
				buildProjectName: "NA",
				headerMsg:        fmt.Sprintf("Failed to start job in codebuild with %s", aerr.Code()),
			}, nil
		}
		return nil, errors.Wrap(err, "failed to start codebuild job")
	}

	return &buildResult{
		buildID:          aws.StringValue(startResult.Build.Id),
		buildStatus:      aws.StringValue(startResult.Build.BuildStatus),
		buildProjectName: aws.StringValue(startResult.Build.ProjectName),
		headerMsg:        "Started a job in codebuild on :aws:",
	}, nil
}
