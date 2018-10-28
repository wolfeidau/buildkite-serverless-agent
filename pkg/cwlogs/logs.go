package cwlogs

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/pkg/errors"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
)

// CloudwatchLogsReader cloudwatch log reader which uploads chunch of log data to buildkite
type CloudwatchLogsReader struct {
	cfg       *config.Config
	cwlogsSvc cloudwatchlogsiface.CloudWatchLogsAPI
}

// NewCloudwatchLogsReader read all the things
func NewCloudwatchLogsReader(cfg *config.Config, cwlogsSvc cloudwatchlogsiface.CloudWatchLogsAPI) *CloudwatchLogsReader {
	return &CloudwatchLogsReader{
		cfg:       cfg,
		cwlogsSvc: cwlogsSvc,
	}
}

// ReadLogs this reads a page of logs from cloudwatch and returns a token which will access the next page
func (cwlr *CloudwatchLogsReader) ReadLogs(buildID string, nextToken string) (string, []byte, error) {

	tokens := strings.Split(buildID, ":")
	if len(tokens) != 2 {
		return "", nil, errors.Errorf("unable to parse build id: %s", buildID)
	}

	groupName := fmt.Sprintf("/aws/codebuild/%s", tokens[0])
	streamName := tokens[1]

	getlogsInput := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(groupName),
		LogStreamName: aws.String(streamName),
	}

	if nextToken != "" {
		getlogsInput.NextToken = aws.String(nextToken)
	}

	logrus.WithFields(logrus.Fields{
		"LogGroupName":  groupName,
		"LogStreamName": streamName,
		"NextToken":     nextToken,
	}).Info("GetLogEvents")

	getlogsResult, err := cwlr.cwlogsSvc.GetLogEvents(getlogsInput)
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to read logs from codebuild cloudwatch log group")
	}

	buf := new(bytes.Buffer)

	for _, event := range getlogsResult.Events {
		_, err := buf.WriteString(aws.StringValue(event.Message))
		if err != nil {
			return "", nil, errors.Wrap(err, "failed to append to buffer")
		}
	}

	nextTokenResult := nextToken

	// only update the token some events came through
	if len(getlogsResult.Events) != 0 {
		nextTokenResult = aws.StringValue(getlogsResult.NextForwardToken)
	}

	return nextTokenResult, buf.Bytes(), nil
}