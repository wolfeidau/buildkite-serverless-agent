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
	"github.com/wolfeidau/aws-launch/pkg/cwlogs"
	"github.com/wolfeidau/buildkite-serverless-agent/mocks"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
)

func TestCompletedJobHandler_HandlerCompletedJob(t *testing.T) {

	paramStore := &mocks.Store{}
	paramStore.On("GetAgentConfig", "/dev/1/buildkite").Return(&api.Agent{
		Name:        "buildkite",
		AccessToken: "token123",
	}, nil)

	cwlogsSvc := &mocks.CloudWatchLogsAPI{}
	cwlogsSvc.On("GetLogEvents", mock.AnythingOfType("*cloudwatchlogs.GetLogEventsInput")).Return(&cloudwatchlogs.GetLogEventsOutput{}, nil)

	buildkiteAPI := &mocks.API{}
	buildkiteAPI.On("FinishJob", "token123", mock.AnythingOfType("*api.Job")).Return(nil)

	cfg := &config.Config{
		EnvironmentName:   "dev",
		EnvironmentNumber: "1",
	}

	logsReader := &launchmocks.LogsReader{}
	logsReader.On("ReadLogs", &cwlogs.ReadLogsParams{
		GroupName:  "/aws/codebuild/buildkite-dev-1",
		StreamName: "58df10ab-9dc5-4c7f-b0c3-6a02b63306ba",
		NextToken:  aws.String("nextToken"),
	}).Return(&cwlogs.ReadLogsResult{}, nil)

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
				evt:    newEvent(),
				status: codebuild.StatusTypeSucceeded,
			},
			want:    "0",
			wantErr: false,
		},
		{
			name: "completed build with StatusTypeFailed",
			args: args{
				ctx:    context.TODO(),
				evt:    newEvent(),
				status: codebuild.StatusTypeFailed,
			},
			want:    "-2",
			wantErr: false,
		},
		{
			name: "completed build with StatusTypeStopped",
			args: args{
				ctx:    context.TODO(),
				evt:    newEvent(),
				status: codebuild.StatusTypeStopped,
			},
			want:    "-3",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			bkw := &CompletedJobHandler{
				cfg:          cfg,
				paramStore:   paramStore,
				buildkiteAPI: buildkiteAPI,
				logsReader:   logsReader,
			}

			tt.args.evt.Codebuild.BuildStatus = tt.args.status
			got, err := bkw.HandlerCompletedJob(tt.args.ctx, tt.args.evt)
			if (err != nil) != tt.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Equal(t, tt.want, got.Job.ExitStatus)
		})
	}
}

func newEvent() *bk.WorkflowData {
	return &bk.WorkflowData{
		AgentName: "buildkite",
		Codebuild: &bk.CodebuildWorkflowData{
			ProjectName:   "whatever",
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
}
