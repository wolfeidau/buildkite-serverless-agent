package bk

import (
	"fmt"
	"runtime"
	"time"

	"github.com/buildkite/agent/agent"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/telemetry"
)

// AgentAPI wrapper around all the buildkite api operations
type AgentAPI struct {
}

// NewAgentAPI create a new agent api
func NewAgentAPI() *AgentAPI {
	return &AgentAPI{}
}

// Register register an agent
func (ab *AgentAPI) Register(agentName string, agentKey string, tags []string) (*api.Agent, error) {
	defer telemetry.MeasureSince("register", time.Now())

	client := newAgent(agentKey)

	agentConfig, res, err := client.Agents.Register(&api.Agent{
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
	defer telemetry.ReportAPIResponse(res)

	return agentConfig, nil
}

// Beat send a heartbeat to the agent api
func (ab *AgentAPI) Beat(agentKey string) (*api.Heartbeat, error) {
	defer telemetry.MeasureSince("beat", time.Now())

	client := newAgent(agentKey)

	heartbeat, res, err := client.Heartbeats.Beat()
	if err != nil {
		return nil, errors.Wrap(err, "failed to send agent heartbeat")
	}
	defer telemetry.ReportAPIResponse(res)

	return heartbeat, nil
}

// Ping ping the agent api for a job
func (ab *AgentAPI) Ping(agentKey string) (*api.Ping, error) {
	defer telemetry.MeasureSince("ping", time.Now())

	client := newAgent(agentKey)

	ping, res, err := client.Pings.Get()
	if err != nil {
		return nil, errors.Wrap(err, "failed to send agent ping")
	}
	defer telemetry.ReportAPIResponse(res)

	return ping, nil
}

// AcceptJob accept the job provided by buildkite
func (ab *AgentAPI) AcceptJob(agentKey string, job *api.Job) (*api.Job, error) {
	defer telemetry.MeasureSince("acceptjob", time.Now())

	client := newAgent(agentKey)

	job, res, err := client.Jobs.Accept(job)
	if err != nil {
		return nil, errors.Wrap(err, "failed to accept job")
	}
	defer telemetry.ReportAPIResponse(res)

	return job, nil
}

// StartJob start the job provided by buildkite
func (ab *AgentAPI) StartJob(agentKey string, job *api.Job) error {
	defer telemetry.MeasureSince("startjob", time.Now())

	client := newAgent(agentKey)

	// update the started date
	job.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)

	res, err := client.Jobs.Start(job)
	if err != nil {
		return errors.Wrap(err, "failed to start job")
	}
	defer telemetry.ReportAPIResponse(res)

	// we failed to start the job
	if res.StatusCode > 299 {
		return errors.Errorf("failed to start job, returned status: %s", res.Status)
	}

	return nil
}

// GetStateJob get the state of the job
func (ab *AgentAPI) GetStateJob(agentKey string, jobID string) (*api.JobState, error) {
	defer telemetry.MeasureSince("getstatejob", time.Now())

	client := newAgent(agentKey)

	jobState, res, err := client.Jobs.GetState(jobID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get job state")
	}
	defer telemetry.ReportAPIResponse(res)

	// we failed to start the job
	if res.StatusCode > 299 {
		return nil, errors.Errorf("failed to get job state, returned status: %s", res.Status)
	}

	return jobState, nil
}

// ChunksUpload upload chunk of log data
func (ab *AgentAPI) ChunksUpload(agentKey string, jobID string, chunk *api.Chunk) error {
	defer telemetry.MeasureSince("chunkUpload", time.Now())

	client := newAgent(agentKey)

	res, err := client.Chunks.Upload(jobID, chunk)
	if err != nil {
		return errors.Wrap(err, "failed to upload chunk")
	}
	defer telemetry.ReportAPIResponse(res)

	return nil
}

// FinishJob finish the job provided by buildkite
func (ab *AgentAPI) FinishJob(agentKey string, job *api.Job) error {
	defer telemetry.MeasureSince("getstatejob", time.Now())

	client := newAgent(agentKey)

	job.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	job.ChunksFailedCount = 0

	res, err := client.Jobs.Finish(job)
	if err != nil {
		return errors.Wrap(err, "failed to finish job")
	}

	defer telemetry.ReportAPIResponse(res)

	if res.StatusCode == 422 {
		return errors.Errorf("Buildkite rejected the call to finish the job (%s)", res.Status)
	}

	return nil
}

// enables overriding of the user agent to ensure this agent is recongised as a
// seperate project.
func newAgent(agentKey string) *api.Client {
	client := agent.APIClient{Endpoint: DefaultAPIEndpoint, Token: agentKey}.Create()
	client.UserAgent = fmt.Sprintf("buildkite-serverless-agent/%s_%s (%s; %s)", Version, BuildVersion, runtime.GOOS, runtime.GOARCH)
	return client
}
