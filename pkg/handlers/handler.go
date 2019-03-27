package handlers

import (
	"bytes"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wolfeidau/aws-launch/pkg/cwlogs"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/params"
)

const (
	codebuildProjectPrefix = "BuildkiteProject"
)

func getBKClient(agentName string, cfg *config.Config, paramStore params.Store) (string, *api.Agent, error) {
	agentSSMConfigKey := fmt.Sprintf("/%s/%s/%s", cfg.EnvironmentName, cfg.EnvironmentNumber, agentName)

	agentConfig, err := paramStore.GetAgentConfig(agentSSMConfigKey)
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to load agent configuration")
	}
	return agentConfig.AccessToken, agentConfig, nil
}

func uploadLogChunks(token string, buildkiteAPI bk.API, logsReader cwlogs.LogsReader, evt *bk.WorkflowData) error {

	req := &cwlogs.ReadLogsParams{
		GroupName:  evt.Codebuild.LogGroupName,
		StreamName: evt.Codebuild.LogStreamName,
	}

	if evt.NextToken != "" {
		req.NextToken = aws.String(evt.NextToken)
	}

	// evt.Codebuild.BuildID, evt.NextToken
	res, err := logsReader.ReadLogs(req)
	if err != nil {
		return errors.Wrap(err, "failed to finish job")
	}

	logrus.WithFields(logrus.Fields{
		"BuildID":          evt.Codebuild.BuildID,
		"LogGroupName":     evt.Codebuild.LogGroupName,
		"LogStreamName":    evt.Codebuild.LogStreamName,
		"LogLinesLen":      len(res.LogLines),
		"CurrentNextToken": evt.NextToken,
		"NewNextToken":     res.NextToken,
	}).Info("ReadLogs")

	// The token for the next set of items in the forward direction. If you have
	// reached the end of the stream, it will return the same token you passed in.
	if evt.NextToken == aws.StringValue(res.NextToken) {
		return nil
	}

	if len(res.LogLines) > 0 {

		buf := new(bytes.Buffer)

		for _, logLine := range res.LogLines {
			_, err := buf.WriteString(logLine.Message)
			if err != nil {
				return errors.Wrap(err, "failed to append to buffer")
			}
		}

		p := make([]byte, evt.Job.ChunksMaxSizeBytes)
		for {
			br, err := buf.Read(p)
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}

			logrus.WithFields(logrus.Fields{
				"BuildID": evt.Codebuild.BuildID,
				"n":       br,
			}).Info("Read Chunk")

			err = buildkiteAPI.ChunksUpload(token, evt.Job.ID, &api.Chunk{
				Data:     string(p[:br]),
				Sequence: evt.LogSequence,
				Offset:   evt.LogBytes,
				Size:     br,
			})
			if err != nil {
				return errors.Wrap(err, "failed to write chunk to buildkite API")
			}

			logrus.WithFields(logrus.Fields{
				"BuildID":     evt.Codebuild.BuildID,
				"LogSequence": evt.LogSequence,
				"LogBytes":    evt.LogBytes,
			}).Info("Wrote Chunk")

			// increment everything
			evt.LogSequence++
			evt.LogBytes += br
		}

		// only update the token if events came through
		evt.NextToken = aws.StringValue(res.NextToken)
	}

	return nil
}
