package registration

import (
	"fmt"
	"runtime"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/buildkite/agent/agent"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/params"
)

// DefaultAgentNamePrefix serverless agent name prefix used when registering the agent
const DefaultAgentNamePrefix = "serverless-agent"

// Service manager registration
type Service struct {
	cfg        *config.Config
	sess       *session.Session
	paramStore *params.Store
}

// New new registration service
func New(cfg *config.Config, sess *session.Session) *Service {

	paramStore := params.New(cfg, ssm.New(sess))

	return &Service{
		cfg:        cfg,
		sess:       sess,
		paramStore: paramStore,
	}
}

// RegisterAgent register the agent in buildkite using the agent key
func (rm *Service) RegisterAgent() (*api.Agent, error) {

	agentSSMKey := fmt.Sprintf("/%s/%s/buildkite-agent-key", rm.cfg.EnvironmentName, rm.cfg.EnvironmentNumber)

	agentKey, err := rm.paramStore.GetAgentKey(agentSSMKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve key from ssm")
	}

	client := agent.APIClient{Endpoint: bk.DefaultAPIEndpoint, Token: agentKey}.Create()

	agentName := fmt.Sprintf("%s-%s-%s", DefaultAgentNamePrefix, rm.cfg.EnvironmentName, rm.cfg.EnvironmentNumber)

	agentConfig, _, err := client.Agents.Register(&api.Agent{
		Name: agentName,
		// Priority:          r.Priority,
		Tags:    []string{"aws", "serverless", "codebuild"},
		Version: bk.Version,
		Build:   bk.BuildVersion,
		Arch:    runtime.GOARCH,
		OS:      runtime.GOOS,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to register agent")
	}

	agentSSMConfigKey := fmt.Sprintf("/%s/%s/agent-config", rm.cfg.EnvironmentName, rm.cfg.EnvironmentNumber)

	err = rm.paramStore.SaveAgentConfig(agentSSMConfigKey, agentConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to register agent")
	}

	return agentConfig, nil
}
