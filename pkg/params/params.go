package params

import (
	"encoding/json"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/ssmcache"
)

// Store store for buildkite agent params
type Store interface {
	GetAgentKey(string) (string, error)
	GetAgentConfig(string) (*api.Agent, error)
	SaveAgentConfig(string, *api.Agent) error
}

// SSMStore store for buildkite related params which is backed by SSM
type SSMStore struct {
	cfg      *config.Config
	ssmCache ssmcache.Cache
}

// New create a new params store which is backed by SSM
func New(cfg *config.Config, ssmSvc *session.Session) *SSMStore {
	return &SSMStore{
		ssmCache: ssmcache.New(session.Must(session.NewSession())),
		cfg:      cfg,
	}
}

// GetAgentKey retrieve the buildkite agent key from the params store
func (st *SSMStore) GetAgentKey(agentSSMKey string) (string, error) {

	value, err := st.ssmCache.GetKey(agentSSMKey, true)
	if err != nil {
		return "", errors.Wrapf(err, "failed to retrieve key %s from ssm", agentSSMKey)
	}

	return value, nil
}

// GetAgentConfig retrieve the buildkite agent config from the params store
func (st *SSMStore) GetAgentConfig(agentSSMConfigKey string) (*api.Agent, error) {

	value, err := st.ssmCache.GetKey(agentSSMConfigKey, true)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve key %s from ssm", agentSSMConfigKey)
	}

	agentConfig := new(api.Agent)

	stringReader := strings.NewReader(value)

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

	err = st.ssmCache.PutKey(agentSSMConfigKey, string(agentData), true)
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve key %s from ssm", agentSSMConfigKey)
	}

	logrus.Info("saved agent configuration")

	return nil
}
