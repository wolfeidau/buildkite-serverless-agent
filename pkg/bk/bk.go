package bk

import (
	"runtime"

	"github.com/buildkite/agent/agent"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
)

const (
	// DefaultAPIEndpoint default api endpoint
	DefaultAPIEndpoint = "https://agent.buildkite.com/v3"

	// Version the serverless agent version
	Version = "1.0.0"

	// BuildVersion the serverless agent version
	BuildVersion = "1.0.0"

	// MaxAgentConcurrentJobs jobs which can be run per agent instance
	MaxAgentConcurrentJobs = 1

	// DefaultAgentNamePrefix serverless agent name prefix used when registering the agent
	DefaultAgentNamePrefix = "serverless-agent"
)

// WorkflowData this is information passed along in the step function workflow
type WorkflowData struct {
	Job                  *api.Job `json:"job,omitempty"`
	BuildID              string   `json:"build_id,omitempty"`
	BuildStatus          string   `json:"build_status,omitempty"`
	WaitTime             int      `json:"wait_time,omitempty"`
	NextToken            string   `json:"next_token,omitempty"`
	LogBytes             int      `json:"log_bytes,omitempty"`
	LogSequence          int      `json:"log_sequence,omitempty"`
	AgentName            string   `json:"agent_name,omitempty"`
	CodeBuildProjectName string   `json:"code_build_project_name,omitempty"`
}

// API wrap up all the buildkite api operations
type API interface {
	Register(string, string, []string) (*api.Agent, error)
	Beat(string) (*api.Heartbeat, error)
	Ping(string) (*api.Ping, error)
	AcceptJob(string, *api.Job) (*api.Job, error)
}

// AgentAPI wrapper around all the buildkite api operations
type AgentAPI struct {
}

// NewAgentAPI create a new agent api
func NewAgentAPI() *AgentAPI {
	return &AgentAPI{}
}

// Register register an agent
func (ab *AgentAPI) Register(agentName string, agentKey string, tags []string) (*api.Agent, error) {

	client := agent.APIClient{Endpoint: DefaultAPIEndpoint, Token: agentKey}.Create()

	agentConfig, _, err := client.Agents.Register(&api.Agent{
		Name: agentName,
		// Priority:          r.Priority,
		Tags:    tags,
		Version: Version,
		Build:   BuildVersion,
		Arch:    runtime.GOARCH,
		OS:      runtime.GOOS,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to register agent")
	}

	return agentConfig, nil
}

// Beat send a heartbeat to the agent api
func (ab *AgentAPI) Beat(agentKey string) (*api.Heartbeat, error) {

	client := agent.APIClient{Endpoint: DefaultAPIEndpoint, Token: agentKey}.Create()

	heartbeat, _, err := client.Heartbeats.Beat()
	if err != nil {
		return nil, errors.Wrap(err, "failed to send agent heartbeat")
	}

	return heartbeat, nil
}

// Ping ping the agent api for a job
func (ab *AgentAPI) Ping(agentKey string) (*api.Ping, error) {

	client := agent.APIClient{Endpoint: DefaultAPIEndpoint, Token: agentKey}.Create()

	ping, _, err := client.Pings.Get()
	if err != nil {
		return nil, errors.Wrap(err, "failed to send agent ping")
	}

	return ping, nil
}

// AcceptJob accept the job provided by buildkite
func (ab *AgentAPI) AcceptJob(agentKey string, job *api.Job) (*api.Job, error) {

	client := agent.APIClient{Endpoint: DefaultAPIEndpoint, Token: agentKey}.Create()

	job, _, err := client.Jobs.Accept(job)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send agent ping")
	}

	return job, nil
}
