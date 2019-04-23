package agentpool

import (
	"fmt"

	"github.com/buildkite/agent/api"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/store"
)

// AgentInstance instance of the serverless agent
type AgentInstance struct {
	cfg   *config.Config
	agent *store.AgentRecord
}

// NewAgentInstance create a new agent instance
func NewAgentInstance(cfg *config.Config, agent *store.AgentRecord) *AgentInstance {
	return &AgentInstance{
		cfg:   cfg,
		agent: agent,
	}
}

// Name return the name of the agent instance
func (ai AgentInstance) Name() string {
	return ai.agent.Name
}

// EnvironmentName return the Environment Name of the agent instance
func (ai AgentInstance) EnvironmentName() string {
	return ai.cfg.EnvironmentName
}

// EnvironmentNumber return the Environment Number of the agent instance
func (ai AgentInstance) EnvironmentNumber() string {
	return ai.cfg.EnvironmentNumber
}

// ConfigKey return the key used to store the agent instances configuration
func (ai AgentInstance) ConfigKey() string {
	return fmt.Sprintf("/%s/%s/%s", ai.EnvironmentName(), ai.EnvironmentNumber(), ai.Name())
}

// Tags return the tags for the agent instance
func (ai AgentInstance) CodebuildProject() string {
	return ai.agent.CodebuildProject
}

// Tags return the tags for the agent instance
func (ai AgentInstance) Tags() []string {
	return ai.agent.Tags
}

// AgentConfig return the config for the agent instance
func (ai AgentInstance) AgentConfig() *api.Agent {
	return ai.agent.AgentConfig
}

// Agent return the agent instance
func (ai AgentInstance) Agent() *store.AgentRecord {
	return ai.agent
}
