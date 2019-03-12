package handlers

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/buildkite/agent/api"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/wolfeidau/buildkite-serverless-agent/mocks"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/cwlogs"
)

func TestGetBKClient(t *testing.T) {
	paramStore := &mocks.Store{}

	paramStore.On("GetAgentConfig", "/dev/1/ted").Return(&api.Agent{
		Name: "ted",
	}, nil)

	cfg := &config.Config{
		EnvironmentName:   "dev",
		EnvironmentNumber: "1",
	}

	client, agent, err := getBKClient("ted", cfg, paramStore)
	require.Nil(t, err)
	require.NotNil(t, client)
	require.Equal(t, &api.Agent{Name: "ted"}, agent)
}

func Test_uploadLogChunks(t *testing.T) {

	buildkiteAPI := &mocks.API{}
	buildkiteAPI.On("ChunksUpload", "token123", "abc123", mock.AnythingOfType("*api.Chunk")).Return(nil)

	cwlGetOutput := &cloudwatchlogs.GetLogEventsOutput{
		NextForwardToken: aws.String("f/34139340658027874184690460781927772298499668124394061824"),
		Events: []*cloudwatchlogs.OutputLogEvent{
			&cloudwatchlogs.OutputLogEvent{
				Message: aws.String("3413934065802787418469046078192777229849966812439406182434139340658027874184690460781927772298499668124394061824"),
			},
			&cloudwatchlogs.OutputLogEvent{
				Message: aws.String("test"),
			},
		},
	}

	cwlogsSvc := &mocks.CloudWatchLogsAPI{}
	cwlogsSvc.On("GetLogEvents", &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String("/aws/codebuild/buildkite-dev-1"),
		LogStreamName: aws.String("58df10ab-9dc5-4c7f-b0c3-6a02b63306ba"),
		NextToken:     aws.String("nextToken"),
	}).Return(cwlGetOutput, nil)

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
		NextToken: "nextToken",
	}

	logsReader := cwlogs.NewCloudwatchLogsReader(cfg, cwlogsSvc)

	err := uploadLogChunks("token123", buildkiteAPI, logsReader, evt)
	require.Nil(t, err)
	require.Equal(t, "f/34139340658027874184690460781927772298499668124394061824", evt.NextToken)
}
