package params

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
)

// Store store for buildkite related params which is backed by SSM
type Store struct {
	cfg    *config.Config
	ssmSvc ssmiface.SSMAPI
}

// New create a new params store which is backed by SSM
func New(cfg *config.Config, ssmSvc ssmiface.SSMAPI) *Store {
	return &Store{
		ssmSvc: ssmSvc,
		cfg:    cfg,
	}
}

// GetAgentKey retrieve the buildkite agent key from the params store
func (st *Store) GetAgentKey() (string, error) {
	agentKey := fmt.Sprintf("/%s/%s/buildkite-agent-key", st.cfg.EnvironmentName, st.cfg.EnvironmentNumber)

	resp, err := st.ssmSvc.GetParameter(&ssm.GetParameterInput{
		Name:           aws.String(agentKey),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", errors.Wrapf(err, "failed to retrieve key %s from ssm", agentKey)
	}

	return aws.StringValue(resp.Parameter.Value), nil
}

// GetAgentConfig retrieve the buildkite agent config from the params store
func (st *Store) GetAgentConfig() (*api.Agent, error) {

	agentKey := fmt.Sprintf("/%s/%s/agent-config", st.cfg.EnvironmentName, st.cfg.EnvironmentNumber)

	resp, err := st.ssmSvc.GetParameter(&ssm.GetParameterInput{
		Name:           aws.String(agentKey),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve key %s from ssm", agentKey)
	}

	agentConfig := new(api.Agent)

	stringReader := strings.NewReader(aws.StringValue(resp.Parameter.Value))

	err = json.NewDecoder(stringReader).Decode(agentConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse JSON for key %s from ssm", agentKey)
	}

	return agentConfig, nil
}

// SaveAgentConfig save the agent configuration to SSM so it can be retrieved by other lambda functions
func (st *Store) SaveAgentConfig(agentConfig *api.Agent) error {

	agentKey := fmt.Sprintf("/%s/%s/agent-config", st.cfg.EnvironmentName, st.cfg.EnvironmentNumber)

	agentData, err := json.Marshal(agentConfig)
	if err != nil {
		return errors.Wrap(err, "failed to write agent config to ssm")
	}

	resp, err := st.ssmSvc.PutParameter(&ssm.PutParameterInput{
		Name:      aws.String(agentKey),
		Type:      aws.String(ssm.ParameterTypeSecureString),
		Value:     aws.String(string(agentData)),
		Overwrite: aws.Bool(true),
	})
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve key %s from ssm", agentKey)
	}

	logrus.WithField("version", aws.Int64Value(resp.Version)).Info("saved agent configuration")

	return nil
}
