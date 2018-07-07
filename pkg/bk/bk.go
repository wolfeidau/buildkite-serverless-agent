package bk

import "github.com/buildkite/agent/api"

const (
	// DefaultAPIEndpoint default api endpoint
	DefaultAPIEndpoint = "https://agent.buildkite.com/v3"

	// Version the serverless agent version
	Version = "1.0.0"

	// BuildVersion the serverless agent version
	BuildVersion = "1.0.0"
)

// WorkflowData this is information passed along in the step function workflow
type WorkflowData struct {
	Job         *api.Job `json:"job,omitempty"`
	BuildID     string   `json:"build_id,omitempty"`
	BuildStatus string   `json:"build_status,omitempty"`
	WaitTime    int      `json:"wait_time,omitempty"`
	NextToken   string   `json:"next_token,omitempty"`
	LogBytes    int      `json:"log_bytes,omitempty"`
	LogSequence int      `json:"log_sequence,omitempty"`
}
