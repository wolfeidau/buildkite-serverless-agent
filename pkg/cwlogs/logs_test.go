package cwlogs

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/wolfeidau/buildkite-serverless-agent/mocks"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
)

func TestReadLogs(t *testing.T) {

	cwlGetOutput := &cloudwatchlogs.GetLogEventsOutput{
		NextForwardToken: aws.String("f/34139340658027874184690460781927772298499668124394061824"),
		Events: []*cloudwatchlogs.OutputLogEvent{
			&cloudwatchlogs.OutputLogEvent{
				Message: aws.String("test"),
			},
			&cloudwatchlogs.OutputLogEvent{
				Message: aws.String("test"),
			},
		},
	}

	cwlogsSvc := &mocks.CloudWatchLogsAPI{}

	cwlogsSvc.On("GetLogEvents", mock.Anything).Return(cwlGetOutput, nil)

	config := &config.Config{
		EnvironmentName:   "dev",
		EnvironmentNumber: "1",
	}

	logReader := NewCloudwatchLogsReader(config, cwlogsSvc)

	next, data, err := logReader.ReadLogs("buildkite-dev-1:58df10ab-9dc5-4c7f-b0c3-6a02b63306ba", "")
	require.Nil(t, err)
	require.Len(t, data, 8)
	require.Equal(t, "f/34139340658027874184690460781927772298499668124394061824", next)
}
