package handlers

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/codebuild"
	"github.com/buildkite/agent/api"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/wolfeidau/buildkite-serverless-agent/mocks"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/cwlogs"
)

func TestCheckJobHandler_HandlerCheckJob_Running(t *testing.T) {

	paramStore := &mocks.Store{}
	paramStore.On("GetAgentConfig", "/dev/1/buildkite").Return(&api.Agent{
		Name:        "buildkite",
		AccessToken: "token123",
	}, nil)

	cwlogsSvc := &mocks.CloudWatchLogsAPI{}
	cwlogsSvc.On("GetLogEvents", mock.AnythingOfType("*cloudwatchlogs.GetLogEventsInput")).Return(&cloudwatchlogs.GetLogEventsOutput{}, nil)

	codebuildSvc := &mocks.CodeBuildAPI{}
	codebuildSvc.On("BatchGetBuilds", mock.AnythingOfType("*codebuild.BatchGetBuildsInput")).Return(
		&codebuild.BatchGetBuildsOutput{
			Builds: []*codebuild.Build{
				&codebuild.Build{
					BuildStatus: aws.String(codebuild.StatusTypeSucceeded),
				},
			},
		}, nil,
	)

	buildkiteAPI := &mocks.API{}
	buildkiteAPI.On("GetStateJob", "token123", "abc123").Return(&api.JobState{
		State: "running",
	}, nil)

	cfg := &config.Config{
		EnvironmentName:   "dev",
		EnvironmentNumber: "1",
	}

	evt := &bk.WorkflowData{
		AgentName: "buildkite",
		BuildID:   "buildkite-dev-1:58df10ab-9dc5-4c7f-b0c3-6a02b63306ba",
		Job: &api.Job{
			ID:                 "abc123",
			ChunksMaxSizeBytes: 102400,
		},
		NextToken:            "nextToken",
		CodeBuildProjectName: "whatever",
		BuildStatus:          codebuild.StatusTypeSucceeded,
	}

	logsReader := cwlogs.NewCloudwatchLogsReader(cfg, cwlogsSvc)

	ch := &CheckJobHandler{
		cfg:          cfg,
		paramStore:   paramStore,
		buildkiteAPI: buildkiteAPI,
		codebuildSvc: codebuildSvc,
		logsReader:   logsReader,
	}
	got, err := ch.HandlerCheckJob(context.TODO(), evt)
	require.Nil(t, err)
	require.Equal(t, codebuild.StatusTypeSucceeded, got.BuildStatus)
}

func TestCheckJobHandler_HandlerCheckJob_Cancelled(t *testing.T) {

	paramStore := &mocks.Store{}
	paramStore.On("GetAgentConfig", "/dev/1/buildkite").Return(&api.Agent{
		Name:        "buildkite",
		AccessToken: "token123",
	}, nil)

	cwlogsSvc := &mocks.CloudWatchLogsAPI{}
	cwlogsSvc.On("GetLogEvents", mock.AnythingOfType("*cloudwatchlogs.GetLogEventsInput")).Return(&cloudwatchlogs.GetLogEventsOutput{}, nil)

	codebuildSvc := &mocks.CodeBuildAPI{}
	codebuildSvc.On("BatchGetBuilds", mock.AnythingOfType("*codebuild.BatchGetBuildsInput")).Return(
		&codebuild.BatchGetBuildsOutput{
			Builds: []*codebuild.Build{
				&codebuild.Build{
					BuildStatus: aws.String(codebuild.StatusTypeSucceeded),
				},
			},
		}, nil,
	)
	codebuildSvc.On("StopBuild", mock.AnythingOfType("*codebuild.StopBuildInput")).Return(&codebuild.StopBuildOutput{
		Build: &codebuild.Build{
			BuildStatus: aws.String(codebuild.StatusTypeStopped),
		},
	}, nil)

	buildkiteAPI := &mocks.API{}
	buildkiteAPI.On("GetStateJob", "token123", "abc123").Return(&api.JobState{
		State: "canceled",
	}, nil)

	cfg := &config.Config{
		EnvironmentName:   "dev",
		EnvironmentNumber: "1",
	}

	evt := &bk.WorkflowData{
		AgentName: "buildkite",
		BuildID:   "buildkite-dev-1:58df10ab-9dc5-4c7f-b0c3-6a02b63306ba",
		Job: &api.Job{
			ID:                 "abc123",
			ChunksMaxSizeBytes: 102400,
		},
		NextToken:            "nextToken",
		CodeBuildProjectName: "whatever",
		BuildStatus:          codebuild.StatusTypeSucceeded,
	}

	logsReader := cwlogs.NewCloudwatchLogsReader(cfg, cwlogsSvc)

	ch := &CheckJobHandler{
		cfg:          cfg,
		paramStore:   paramStore,
		buildkiteAPI: buildkiteAPI,
		codebuildSvc: codebuildSvc,
		logsReader:   logsReader,
	}
	got, err := ch.HandlerCheckJob(context.TODO(), evt)
	require.Nil(t, err)
	require.Equal(t, codebuild.StatusTypeStopped, got.BuildStatus)
}
