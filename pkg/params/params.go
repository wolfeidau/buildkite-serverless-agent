package params

import (
	"github.com/pkg/errors"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/ssmcache"
)

// Store store for buildkite agent params
type Store interface {
	GetAgentKey(string) (string, error)
}

// SSMStore store for buildkite related params which is backed by SSM
type SSMStore struct {
	cfg      *config.Config
	ssmCache ssmcache.Cache
}

// New create a new params store which is backed by SSM
func New(cfg *config.Config) *SSMStore {
	return &SSMStore{
		ssmCache: ssmcache.New(),
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
