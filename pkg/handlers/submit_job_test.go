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
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/store"
)

func TestSubmitHandler_HandlerSubmitJob(t *testing.T) {
	agentStore := &mocks.AgentsAPI{}

	agentStore.On("Get", "ted").Return(&store.AgentRecord{
		Name: "ted",
		AgentConfig: &api.Agent{
			AccessToken: "token123",
		},
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
		Codebuild: &bk.CodebuildWorkflowData{
			ProjectName: "testproject-1",
		},
	}

	sh := &SubmitJobHandler{
		cfg:          cfg,
		agentStore:   agentStore,
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
	require.NotNil(t, evt.Job)
	require.Equal(t, "abc123", got.Job.ID)
	require.Equal(t, "dev", got.Job.Env["ENVIRONMENT_NAME"])
	require.Equal(t, "1", got.Job.Env["ENVIRONMENT_NUMBER"])

}

func TestSubmitHandler_HandlerSubmitJob_ErrNotFound(t *testing.T) {
	agentStore := &mocks.AgentsAPI{}

	agentStore.On("Get", "ted").Return(&store.AgentRecord{
		Name: "ted",
		AgentConfig: &api.Agent{
			AccessToken: "token123",
		},
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
			Env: map[string]string{"CB_PROJECT_NAME": "testproject-1"},
		},
		Codebuild: &bk.CodebuildWorkflowData{
			ProjectName: "testproject-1",
		},
	}

	sh := &SubmitJobHandler{
		cfg:          cfg,
		agentStore:   agentStore,
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
