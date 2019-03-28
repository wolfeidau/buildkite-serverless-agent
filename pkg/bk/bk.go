package bk

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/codebuild"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
)

const (
	// DefaultAPIEndpoint default api endpoint
	DefaultAPIEndpoint = "https://agent.buildkite.com/v3"

	// MaxAgentConcurrentJobs jobs which can be run per agent instance
	MaxAgentConcurrentJobs = 1

	// DefaultAgentNamePrefix serverless agent name prefix used when registering the agent
	DefaultAgentNamePrefix = "serverless-agent"

	// DefaultWaitTime used in the SFN loop to poll job status
	DefaultWaitTime = 10
)

var (
	// Version the serverless agent version
	Version = "dev"

	// BuildVersion the serverless agent version
	BuildVersion = "dev"
)

// WorkflowData this is information passed along in the step function workflow
type WorkflowData struct {
	Job         *api.Job               `json:"job,omitempty"` // buildkite job
	WaitTime    int                    `json:"wait_time,omitempty"`
	NextToken   string                 `json:"next_token,omitempty"`
	LogBytes    int                    `json:"log_bytes,omitempty"`    // used for cloudwatch log streaming
	LogSequence int                    `json:"log_sequence,omitempty"` // used for cloudwatch log streaming
	AgentName   string                 `json:"agent_name,omitempty"`
	Codebuild   *CodebuildWorkflowData `json:"codebuild,omitempty"`
	TaskStatus  string                 `json:"task_status,omitempty"`
}

// CodebuildWorkflowData codebuild workflow info
type CodebuildWorkflowData struct {
	BuildID         string `json:"build_id,omitempty"`
	BuildStatus     string `json:"build_status,omitempty"`
	ProjectName     string `json:"project_name,omitempty"`
	LogGroupName    string `json:"log_group_name,omitempty"`
	LogStreamName   string `json:"log_stream_name,omitempty"`
	LogStreamPrefix string `json:"log_stream_prefix,omitempty"`
}

// UpdateJobExitCode update the exit code of the buildkite job using info from codebuild
func (evt *WorkflowData) UpdateJobExitCode() error {

	if evt.Job == nil {
		return errors.New("job is missing in workflow event")
	}

	// this is currently defaulted as error cases may result in this being empty
	if evt.Codebuild == nil {
		evt.Job.ExitStatus = "-5"
		return nil
	}

	switch evt.Codebuild.BuildStatus {
	case codebuild.StatusTypeStopped:
		evt.Job.ExitStatus = "-3"
	case codebuild.StatusTypeFailed:
		evt.Job.ExitStatus = "-2"
	case codebuild.StatusTypeSucceeded:
		evt.Job.ExitStatus = "0"
	default:
		evt.Job.ExitStatus = "-4"
	}

	return nil
}

// UpdateCodebuildProject create the codebuild section and assign a project name
func (evt *WorkflowData) UpdateCodebuildProject(buildProjectName string) {

	if codebuildProj := evt.GetJobEnvString("CB_PROJECT_NAME"); codebuildProj != nil {
		buildProjectName = aws.StringValue(codebuildProj)
	}

	evt.Codebuild = &CodebuildWorkflowData{
		ProjectName: buildProjectName,
	}

}

// UpdateCloudwatchLogs assign the log group/stream names
func (evt *WorkflowData) UpdateCloudwatchLogs(buildID string) error {

	tokens := strings.Split(buildID, ":")
	if len(tokens) != 2 {
		return errors.Errorf("unable to parse build id: %s", buildID)
	}

	groupName := fmt.Sprintf("/aws/codebuild/%s", tokens[0])
	streamName := tokens[1]

	evt.Codebuild.BuildID = buildID

	// override the stream name if prefix is present caters for 2.x aws_launch based projects
	if evt.Codebuild.LogStreamPrefix == "" {
		// original cloudwatch settings
		evt.Codebuild.LogGroupName = groupName
		evt.Codebuild.LogStreamName = streamName
	} else {
		evt.Codebuild.LogStreamName = fmt.Sprintf("%s/%s", evt.Codebuild.LogStreamPrefix, streamName)
	}

	return nil
}

// UpdateCodebuildStatus assign the codebuild status
func (evt *WorkflowData) UpdateCodebuildStatus(buildID, buildStatus, taskStatus string) {
	evt.Codebuild.BuildID = buildID
	evt.Codebuild.BuildStatus = buildStatus

	// moving to the new normalised task statuses
	evt.TaskStatus = taskStatus

	evt.WaitTime = DefaultWaitTime
}

// UpdateBuildJobCreds update the buildkite credentials in the build job environment
func (evt *WorkflowData) UpdateBuildJobCreds(token string) {
	// merge in the agent endpoint so it can send back git information to buildkite
	evt.Job.Env["BUILDKITE_AGENT_ENDPOINT"] = evt.Job.Endpoint

	// merge in the agent access token so it can send back git information to buildkite
	evt.Job.Env["BUILDKITE_AGENT_ACCESS_TOKEN"] = token
}

// GetJobEnvString retrieve values from job environment for use in codebuild
func (evt *WorkflowData) GetJobEnvString(key string) *string {
	if val, ok := evt.Job.Env[key]; ok {
		return aws.String(val)
	}

	return nil
}

// GetJobEnvBool retrieve values from job environment for use in codebuild
func (evt *WorkflowData) GetJobEnvBool(key string) *bool {
	if val, ok := evt.Job.Env[key]; !ok {

		b, err := strconv.ParseBool(val)
		if err != nil {
			return nil
		}

		return aws.Bool(b)
	}

	return nil
}

// GetBuildkitePipelineEnvString get the buildkite variable for the pipeline name which is used as the codebuild project name
func (evt *WorkflowData) GetBuildkitePipelineEnvString() string {
	return evt.Job.Env["BUILDKITE_PIPELINE_SLUG"]
}

// API wrap up all the buildkite api operations
type API interface {
	Register(string, string, []string) (*api.Agent, error)
	Beat(string) (*api.Heartbeat, error)
	Ping(string) (*api.Ping, error)
	AcceptJob(string, *api.Job) (*api.Job, error)
	StartJob(string, *api.Job) error
	FinishJob(string, *api.Job) error
	GetStateJob(string, string) (*api.JobState, error)
	ChunksUpload(string, string, *api.Chunk) error
}
