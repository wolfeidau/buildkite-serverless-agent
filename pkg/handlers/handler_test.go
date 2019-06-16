package handlers

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/buildkite/agent/api"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	launchmocks "github.com/wolfeidau/aws-launch/mocks"
	"github.com/wolfeidau/aws-launch/pkg/cwlogs"
	"github.com/wolfeidau/buildkite-serverless-agent/mocks"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
)

func Test_uploadLogChunks(t *testing.T) {

	buildkiteAPI := &mocks.API{}
	buildkiteAPI.On("ChunksUpload", "token123", "abc123", mock.AnythingOfType("*api.Chunk")).Return(nil)

	logsReader := &launchmocks.LogsReader{}

	readLogsRes := &cwlogs.ReadLogsResult{
		NextToken: aws.String("f/34139340658027874184690460781927772298499668124394061824"),
		LogLines: []*cwlogs.LogLine{
			&cwlogs.LogLine{
				Message: "3413934065802787418469046078192777229849966812439406182434139340658027874184690460781927772298499668124394061824",
			},
			&cwlogs.LogLine{
				Message: "test",
			},
		},
	}

	logsReader.On("ReadLogs", &cwlogs.ReadLogsParams{
		GroupName:  "/aws/codebuild/buildkite-dev-1",
		StreamName: "58df10ab-9dc5-4c7f-b0c3-6a02b63306ba",
		NextToken:  aws.String("nextToken"),
	}).Return(readLogsRes, nil)

	evt := &bk.WorkflowData{
		AgentName: "buildkite",
		Codebuild: &bk.CodebuildWorkflowData{
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

	err := uploadLogChunks("token123", buildkiteAPI, logsReader, evt)
	require.Nil(t, err)
	require.Equal(t, "f/34139340658027874184690460781927772298499668124394061824", evt.NextToken)
}
