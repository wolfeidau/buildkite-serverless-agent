package agentpool

import (
	"fmt"

	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
)

// AgentInstance instance of the serverless agent
type AgentInstance struct {
	cfg   *config.Config
	index int
	tags  []string
}

// NewAgentInstance create a new agent instance
func NewAgentInstance(cfg *config.Config, index int, tags []string) *AgentInstance {
	return &AgentInstance{
		cfg:   cfg,
		index: index,
		tags:  tags,
	}
}

// Name return the name of the agent instance
func (ai AgentInstance) Name() string {
	return fmt.Sprintf("serverless-agent-%s-%s_%d", ai.cfg.EnvironmentName, ai.cfg.EnvironmentNumber, ai.index)
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
func (ai AgentInstance) Tags() []string {
	return ai.tags
}
