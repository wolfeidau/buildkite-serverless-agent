package handlers

import (
	"context"
	"testing"

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

func TestCompletedJobHandler_HandlerCompletedJob(t *testing.T) {

	paramStore := &mocks.Store{}
	paramStore.On("GetAgentConfig", "/dev/1/buildkite").Return(&api.Agent{
		Name:        "buildkite",
		AccessToken: "token123",
	}, nil)

	cwlogsSvc := &mocks.CloudWatchLogsAPI{}
	cwlogsSvc.On("GetLogEvents", mock.AnythingOfType("*cloudwatchlogs.GetLogEventsInput")).Return(&cloudwatchlogs.GetLogEventsOutput{}, nil)
	codebuildSvc := &mocks.CodeBuildAPI{}
	buildkiteAPI := &mocks.API{}
	buildkiteAPI.On("FinishJob", "token123", mock.AnythingOfType("*api.Job")).Return(nil)

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
	}

	logsReader := cwlogs.NewCloudwatchLogsReader(cfg, cwlogsSvc)

	bkw := &CompletedJobHandler{
		cfg:          cfg,
		paramStore:   paramStore,
		buildkiteAPI: buildkiteAPI,
		codebuildSvc: codebuildSvc,
		logsReader:   logsReader,
	}

	type args struct {
		ctx    context.Context
		evt    *bk.WorkflowData
		status string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "completed build with StatusTypeSucceeded",
			args: args{
				ctx:    context.TODO(),
				evt:    evt,
				status: codebuild.StatusTypeSucceeded,
			},
			want:    "0",
			wantErr: false,
		},
		{
			name: "completed build with StatusTypeFailed",
			args: args{
				ctx:    context.TODO(),
				evt:    evt,
				status: codebuild.StatusTypeFailed,
			},
			want:    "-2",
			wantErr: false,
		},
		{
			name: "completed build with StatusTypeStopped",
			args: args{
				ctx:    context.TODO(),
				evt:    evt,
				status: codebuild.StatusTypeStopped,
			},
			want:    "-3",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.evt.BuildStatus = tt.args.status
			got, err := bkw.HandlerCompletedJob(tt.args.ctx, tt.args.evt)
			if (err != nil) != tt.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Equal(t, tt.want, got.Job.ExitStatus)
		})
	}
}
