package params

import (
	"encoding/json"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
)

// Store store for buildkite agent params
type Store interface {
	GetAgentKey(string) (string, error)
	GetAgentConfig(string) (*api.Agent, error)
	SaveAgentConfig(string, *api.Agent) error
}

// SSMStore store for buildkite related params which is backed by SSM
type SSMStore struct {
	cfg    *config.Config
	ssmSvc ssmiface.SSMAPI
}

// New create a new params store which is backed by SSM
func New(cfg *config.Config, ssmSvc ssmiface.SSMAPI) *SSMStore {
	return &SSMStore{
		ssmSvc: ssmSvc,
		cfg:    cfg,
	}
}

// GetAgentKey retrieve the buildkite agent key from the params store
func (st *SSMStore) GetAgentKey(agentSSMKey string) (string, error) {

	resp, err := st.ssmSvc.GetParameter(&ssm.GetParameterInput{
		Name:           aws.String(agentSSMKey),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", errors.Wrapf(err, "failed to retrieve key %s from ssm", agentSSMKey)
	}

	return aws.StringValue(resp.Parameter.Value), nil
}

// GetAgentConfig retrieve the buildkite agent config from the params store
func (st *SSMStore) GetAgentConfig(agentSSMConfigKey string) (*api.Agent, error) {

	resp, err := st.ssmSvc.GetParameter(&ssm.GetParameterInput{
		Name:           aws.String(agentSSMConfigKey),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve key %s from ssm", agentSSMConfigKey)
	}

	agentConfig := new(api.Agent)

	stringReader := strings.NewReader(aws.StringValue(resp.Parameter.Value))

	err = json.NewDecoder(stringReader).Decode(agentConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse JSON for key %s from ssm", agentSSMConfigKey)
	}

	return agentConfig, nil
}

// SaveAgentConfig save the agent configuration to SSM so it can be retrieved by other lambda functions
func (st *SSMStore) SaveAgentConfig(agentSSMConfigKey string, agentConfig *api.Agent) error {

	agentData, err := json.Marshal(agentConfig)
	if err != nil {
		return errors.Wrap(err, "failed to write agent config to ssm")
	}

	resp, err := st.ssmSvc.PutParameter(&ssm.PutParameterInput{
		Name:      aws.String(agentSSMConfigKey),
		Type:      aws.String(ssm.ParameterTypeSecureString),
		Value:     aws.String(string(agentData)),
		Overwrite: aws.Bool(true),
	})
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve key %s from ssm", agentSSMConfigKey)
	}

	logrus.WithField("version", aws.Int64Value(resp.Version)).Info("saved agent configuration")

	return nil
}
