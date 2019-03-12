package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/codebuild"
	"github.com/buildkite/agent/api"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

	codebuildSvc := &mocks.CodeBuildAPI{}
	codebuildSvc.On("StartBuild", mock.AnythingOfType("*codebuild.StartBuildInput")).Return(&codebuild.StartBuildOutput{
		Build: &codebuild.Build{
			Id:          aws.String("abcef-12345"),
			BuildStatus: aws.String(codebuild.StatusTypeInProgress),
		},
	}, nil)

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
			ID: "abc123",
		},
	}

	sh := &SubmitJobHandler{
		cfg:          cfg,
		paramStore:   store,
		codebuildSvc: codebuildSvc,
		buildkiteAPI: buildkiteAPI,
	}

	got, err := sh.HandlerSubmitJob(context.TODO(), evt)
	require.Nil(t, err)

	require.Equal(t, "abcef-12345", got.BuildID)
	require.Equal(t, "IN_PROGRESS", got.BuildStatus)
	require.Equal(t, 101, got.LogBytes)
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

	codebuildSvc := &mocks.CodeBuildAPI{}
	notFoundErr := awserr.New(codebuild.ErrCodeResourceNotFoundException, "woops", errors.New("woops"))
	codebuildSvc.On("StartBuild", mock.AnythingOfType("*codebuild.StartBuildInput")).Return(nil, notFoundErr)

	cfg := &config.Config{
		EnvironmentName:   "dev",
		EnvironmentNumber: "1",
	}

	evt := &bk.WorkflowData{
		AgentName: "ted",
		Job: &api.Job{
			ID: "abc123",
		},
	}

	sh := &SubmitJobHandler{
		cfg:          cfg,
		paramStore:   store,
		codebuildSvc: codebuildSvc,
		buildkiteAPI: buildkiteAPI,
	}

	got, err := sh.HandlerSubmitJob(context.TODO(), evt)
	require.Nil(t, err)

	require.Equal(t, "NA", got.BuildID)
	require.Equal(t, "FAILED", got.BuildStatus)
	require.Equal(t, 117, got.LogBytes)
	require.Equal(t, 1, got.LogSequence)
	require.Equal(t, "ted", got.AgentName)
	require.Equal(t, "abc123", got.Job.ID)
}
