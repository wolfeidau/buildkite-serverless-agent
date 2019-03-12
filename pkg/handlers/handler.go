package handlers

import (
	"bytes"
	"fmt"
	"io"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/codebuild"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/cwlogs"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/params"
)

const codebuildProjectPrefix = "BuildkiteProject"

func getBKClient(agentName string, cfg *config.Config, paramStore params.Store) (string, *api.Agent, error) {
	agentSSMConfigKey := fmt.Sprintf("/%s/%s/%s", cfg.EnvironmentName, cfg.EnvironmentNumber, agentName)

	agentConfig, err := paramStore.GetAgentConfig(agentSSMConfigKey)
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to load agent configuration")
	}
	return agentConfig.AccessToken, agentConfig, nil
}

func uploadLogChunks(token string, buildkiteAPI bk.API, logsReader *cwlogs.CloudwatchLogsReader, evt *bk.WorkflowData) error {

	logrus.WithFields(logrus.Fields{
		"BuildID":   evt.BuildID,
		"NextToken": evt.NextToken,
	}).Info("ReadLogs")

	nextToken, pageData, err := logsReader.ReadLogs(evt.BuildID, evt.NextToken)
	if err != nil {
		return errors.Wrap(err, "failed to finish job")
	}

	logrus.WithFields(logrus.Fields{
		"BuildID":   evt.BuildID,
		"ChunkLen":  len(pageData),
		"NextToken": nextToken,
	}).Info("ReadLogs Complete")

	if len(pageData) > 0 {

		buf := bytes.NewBuffer(pageData)

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
				"BuildID": evt.BuildID,
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
				"BuildID":     evt.BuildID,
				"LogSequence": evt.LogSequence,
				"LogBytes":    evt.LogBytes,
			}).Info("Wrote Chunk")

			// increment everything
			evt.LogSequence++
			evt.LogBytes += br
		}

	}

	evt.NextToken = nextToken

	return nil
}

func convertEnvVars(env map[string]string) []*codebuild.EnvironmentVariable {

	codebuildEnv := make([]*codebuild.EnvironmentVariable, 0)

	for k, v := range env {
		codebuildEnv = append(codebuildEnv, &codebuild.EnvironmentVariable{
			Name:  aws.String(k),
			Value: aws.String(v),
		})
	}

	return codebuildEnv
}

type overrides struct {
	logger logrus.FieldLogger
	env    map[string]string
}

func (ov *overrides) String(key string) *string {
	if val, ok := ov.env[key]; ok {
		ov.logger.Infof("updating %s to %s", key, val)
		return aws.String(val)
	}

	return nil
}

func (ov *overrides) Bool(key string) *bool {
	if val, ok := ov.env[key]; !ok {

		b, err := strconv.ParseBool(val)
		if err != nil {
			ov.logger.Warnf("failed to update %s to %s", key, val)
			return nil
		}

		ov.logger.Infof("updating %s to %t", key, b)
		return aws.Bool(b)
	}

	return nil
}
