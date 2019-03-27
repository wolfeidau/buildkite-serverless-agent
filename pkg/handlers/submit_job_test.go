package handlers

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/codebuild"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/wolfeidau/aws-launch/mocks/codebuildmock"
	cblauncher "github.com/wolfeidau/aws-launch/pkg/launcher/codebuild"
	"github.com/wolfeidau/buildkite-serverless-agent/mocks"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
)

func TestSubmitHandler_HandlerSubmitJob(t *testing.T) {
	store := &mocks.Store{}

	store.On("GetAgentConfig", "/dev/1/ted").Return(&api.Agent{
		Name:        "ted",
		AccessToken: "token123",
	}, nil)

	lch := new(codebuildmock.LauncherAPI)
	lch.On("LaunchTask", mock.AnythingOfType("*codebuild.LaunchTaskParams")).Return(
		&cblauncher.LaunchTaskResult{
			ID:          "buildkite-dev-1:58df10ab-9dc5-4c7f-b0c3-6a02b63306ba",
			TaskStatus:  codebuild.StatusTypeInProgress,
			BuildStatus: codebuild.StatusTypeInProgress,
		}, nil,
	)

	buildkiteAPI := &mocks.API{}
	buildkiteAPI.On("StartJob", "token123", mock.AnythingOfType("*api.Job")).Return(nil)
	buildkiteAPI.On("ChunksUpload", "token123", "abc123", mock.AnythingOfType("*api.Chunk")).Return(nil)

	cfg := &config.Config{
		EnvironmentName:   "dev",
		EnvironmentNumber: "1",
	}

	evt := &bk.WorkflowData{
		AgentName: "ted",
		Job: &api.Job{
			ID:  "abc123",
			Env: map[string]string{},
		},
	}

	sh := &SubmitJobHandler{
		cfg:          cfg,
		paramStore:   store,
		buildkiteAPI: buildkiteAPI,
		lch:          lch,
	}

	got, err := sh.HandlerSubmitJob(context.TODO(), evt)
	require.Nil(t, err)

	require.Equal(t, "buildkite-dev-1:58df10ab-9dc5-4c7f-b0c3-6a02b63306ba", got.Codebuild.BuildID)
	require.Equal(t, "IN_PROGRESS", got.Codebuild.BuildStatus)
	require.Equal(t, 127, got.LogBytes)
	require.Equal(t, 1, got.LogSequence)
	require.Equal(t, "ted", got.AgentName)
	require.Equal(t, "abc123", got.Job.ID)
}

func TestSubmitHandler_HandlerSubmitJobDefine(t *testing.T) {
	store := &mocks.Store{}

	store.On("GetAgentConfig", "/dev/1/ted").Return(&api.Agent{
		Name:        "ted",
		AccessToken: "token123",
	}, nil)

	lch := new(codebuildmock.LauncherAPI)
	lch.On("LaunchTask", mock.AnythingOfType("*codebuild.LaunchTaskParams")).Return(
		&cblauncher.LaunchTaskResult{
			ID:          "buildkite-dev-1:58df10ab-9dc5-4c7f-b0c3-6a02b63306ba",
			TaskStatus:  codebuild.StatusTypeInProgress,
			BuildStatus: codebuild.StatusTypeInProgress,
		}, nil,
	)
	lch.On("DefineTask", mock.AnythingOfType("*codebuild.DefineTaskParams")).Return(&cblauncher.DefineTaskResult{
		CloudwatchLogGroupName: "/aws/codebuild",
		CloudwatchStreamPrefix: "prefix",
	}, nil)

	buildkiteAPI := &mocks.API{}
	buildkiteAPI.On("StartJob", "token123", mock.AnythingOfType("*api.Job")).Return(nil)
	buildkiteAPI.On("ChunksUpload", "token123", "abc123", mock.AnythingOfType("*api.Chunk")).Return(nil)

	cfg := &config.Config{
		EnvironmentName:   "dev",
		EnvironmentNumber: "1",
		DefineAndStart:    "true",
	}

	evt := &bk.WorkflowData{
		AgentName: "ted",
		Job: &api.Job{
			ID: "abc123",
			Env: map[string]string{
				"BUILDKITE_BUILD_ID": "fc09170b-a6e6-4a77-a9c8-055bf99c7698",
			},
		},
	}

	sh := &SubmitJobHandler{
		cfg:          cfg,
		paramStore:   store,
		buildkiteAPI: buildkiteAPI,
		lch:          lch,
	}

	got, err := sh.HandlerSubmitJob(context.TODO(), evt)
	require.Nil(t, err)

	require.Equal(t, "buildkite-dev-1:58df10ab-9dc5-4c7f-b0c3-6a02b63306ba", got.Codebuild.BuildID)
	require.Equal(t, "IN_PROGRESS", got.Codebuild.BuildStatus)
	require.Equal(t, "/aws/codebuild", got.Codebuild.LogGroupName)
	require.Equal(t, "prefix/58df10ab-9dc5-4c7f-b0c3-6a02b63306ba", got.Codebuild.LogStreamName)
	require.Equal(t, "prefix", got.Codebuild.LogStreamPrefix)
	require.Equal(t, 127, got.LogBytes)
	require.Equal(t, 1, got.LogSequence)
	require.Equal(t, "ted", got.AgentName)
	require.Equal(t, "abc123", got.Job.ID)
}

func TestSubmitHandler_HandlerSubmitJob_ErrNotFound(t *testing.T) {
	store := &mocks.Store{}

	store.On("GetAgentConfig", "/dev/1/ted").Return(&api.Agent{
		Name:        "ted",
		AccessToken: "token123",
	}, nil)

	buildkiteAPI := &mocks.API{}
	buildkiteAPI.On("StartJob", "token123", mock.AnythingOfType("*api.Job")).Return(nil)
	buildkiteAPI.On("ChunksUpload", "token123", "abc123", mock.AnythingOfType("*api.Chunk")).Return(nil)

	lch := new(codebuildmock.LauncherAPI)
	notFoundErr := awserr.New(codebuild.ErrCodeResourceNotFoundException, "woops", errors.New("woops"))
	lch.On("LaunchTask", mock.AnythingOfType("*codebuild.LaunchTaskParams")).Return(
		nil, errors.Wrap(notFoundErr, "check cause"),
	)

	cfg := &config.Config{
		EnvironmentName:   "dev",
		EnvironmentNumber: "1",
	}

	evt := &bk.WorkflowData{
		AgentName: "ted",
		Job: &api.Job{
			ID:  "abc123",
			Env: make(map[string]string),
		},
	}

	sh := &SubmitJobHandler{
		cfg:          cfg,
		paramStore:   store,
		buildkiteAPI: buildkiteAPI,
		lch:          lch,
	}

	got, err := sh.HandlerSubmitJob(context.TODO(), evt)
	require.Nil(t, err)

	require.Equal(t, "NA:NA", got.Codebuild.BuildID)
	require.Equal(t, "FAILED", got.Codebuild.BuildStatus)
	require.Equal(t, 103, got.LogBytes)
	require.Equal(t, 1, got.LogSequence)
	require.Equal(t, "ted", got.AgentName)
	require.Equal(t, "abc123", got.Job.ID)
}
