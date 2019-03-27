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
	launchmocks "github.com/wolfeidau/aws-launch/mocks"
	"github.com/wolfeidau/aws-launch/mocks/codebuildmock"
	"github.com/wolfeidau/aws-launch/pkg/cwlogs"
	"github.com/wolfeidau/aws-launch/pkg/launcher"
	cblauncher "github.com/wolfeidau/aws-launch/pkg/launcher/codebuild"
	"github.com/wolfeidau/buildkite-serverless-agent/mocks"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
)

func TestCheckJobHandler_HandlerCheckJob_Running(t *testing.T) {

	paramStore := &mocks.Store{}
	paramStore.On("GetAgentConfig", "/dev/1/buildkite").Return(&api.Agent{
		Name:        "buildkite",
		AccessToken: "token123",
	}, nil)

	cwlogsSvc := &mocks.CloudWatchLogsAPI{}
	cwlogsSvc.On("GetLogEvents", mock.AnythingOfType("*cloudwatchlogs.GetLogEventsInput")).Return(&cloudwatchlogs.GetLogEventsOutput{}, nil)

	lch := new(codebuildmock.LauncherAPI)
	lch.On("GetTaskStatus", mock.AnythingOfType("*codebuild.GetTaskStatusParams")).Return(
		&cblauncher.GetTaskStatusResult{
			ID:          "buildkite-dev-1:58df10ab-9dc5-4c7f-b0c3-6a02b63306ba",
			TaskStatus:  launcher.TaskSucceeded,
			BuildStatus: codebuild.StatusTypeSucceeded,
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
		Codebuild: &bk.CodebuildWorkflowData{
			BuildID:       "buildkite-dev-1:58df10ab-9dc5-4c7f-b0c3-6a02b63306ba",
			ProjectName:   "whatever",
			BuildStatus:   codebuild.StatusTypeSucceeded,
			LogGroupName:  "/aws/codebuild/buildkite-dev-1",
			LogStreamName: "58df10ab-9dc5-4c7f-b0c3-6a02b63306ba",
		},
		Job: &api.Job{
			ID:                 "abc123",
			ChunksMaxSizeBytes: 102400,
		},
		NextToken:  "nextToken",
		TaskStatus: launcher.TaskSucceeded,
	}

	logsReader := &launchmocks.LogsReader{}
	logsReader.On("ReadLogs", &cwlogs.ReadLogsParams{
		GroupName:  "/aws/codebuild/buildkite-dev-1",
		StreamName: "58df10ab-9dc5-4c7f-b0c3-6a02b63306ba",
		NextToken:  aws.String("nextToken"),
	}).Return(&cwlogs.ReadLogsResult{}, nil)

	ch := &CheckJobHandler{
		cfg:          cfg,
		paramStore:   paramStore,
		buildkiteAPI: buildkiteAPI,
		lch:          lch,
		logsReader:   logsReader,
	}
	got, err := ch.HandlerCheckJob(context.TODO(), evt)
	require.Nil(t, err)
	require.Equal(t, codebuild.StatusTypeSucceeded, got.Codebuild.BuildStatus)
	require.Equal(t, launcher.TaskSucceeded, got.TaskStatus)
}

func TestCheckJobHandler_HandlerCheckJob_Cancelled(t *testing.T) {

	paramStore := &mocks.Store{}
	paramStore.On("GetAgentConfig", "/dev/1/buildkite").Return(&api.Agent{
		Name:        "buildkite",
		AccessToken: "token123",
	}, nil)

	cwlogsSvc := &mocks.CloudWatchLogsAPI{}
	cwlogsSvc.On("GetLogEvents", mock.AnythingOfType("*cloudwatchlogs.GetLogEventsInput")).Return(&cloudwatchlogs.GetLogEventsOutput{}, nil)

	lch := new(codebuildmock.LauncherAPI)
	lch.On("GetTaskStatus", mock.AnythingOfType("*codebuild.GetTaskStatusParams")).Return(
		&cblauncher.GetTaskStatusResult{
			ID:          "buildkite-dev-1:58df10ab-9dc5-4c7f-b0c3-6a02b63306ba",
			TaskStatus:  launcher.TaskStopped,
			BuildStatus: codebuild.StatusTypeStopped,
		}, nil,
	)
	lch.On("StopTask", mock.AnythingOfType("*codebuild.StopTaskParams")).Return(
		&cblauncher.StopTaskResult{
			BuildStatus: codebuild.StatusTypeStopped,
			TaskStatus:  launcher.TaskStopped,
		}, nil,
	)

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
		Codebuild: &bk.CodebuildWorkflowData{
			ProjectName:   "whatever",
			BuildStatus:   codebuild.StatusTypeSucceeded,
			BuildID:       "buildkite-dev-1:58df10ab-9dc5-4c7f-b0c3-6a02b63306ba",
			LogGroupName:  "/aws/codebuild/buildkite-dev-1",
			LogStreamName: "58df10ab-9dc5-4c7f-b0c3-6a02b63306ba",
		},
		Job: &api.Job{
			ID:                 "abc123",
			ChunksMaxSizeBytes: 102400,
		},
		NextToken: "nextToken",
	}

	logsReader := &launchmocks.LogsReader{}
	logsReader.On("ReadLogs", &cwlogs.ReadLogsParams{
		GroupName:  "/aws/codebuild/buildkite-dev-1",
		StreamName: "58df10ab-9dc5-4c7f-b0c3-6a02b63306ba",
		NextToken:  aws.String("nextToken"),
	}).Return(&cwlogs.ReadLogsResult{}, nil)

	ch := &CheckJobHandler{
		cfg:          cfg,
		paramStore:   paramStore,
		buildkiteAPI: buildkiteAPI,
		lch:          lch,
		logsReader:   logsReader,
	}
	got, err := ch.HandlerCheckJob(context.TODO(), evt)
	require.Nil(t, err)
	require.Equal(t, codebuild.StatusTypeStopped, got.Codebuild.BuildStatus)
}
