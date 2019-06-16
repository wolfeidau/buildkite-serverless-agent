package agentpool

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/wolfeidau/buildkite-serverless-agent/mocks"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/store"
)

func TestNew(t *testing.T) {

	assert := require.New(t)

	buildkiteAPI := &mocks.API{}

	type args struct {
		cfg  *config.Config
		sess *session.Session
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "New() with valid pool size config",
			args: args{
				cfg: &config.Config{
					EnvironmentName:   "dev",
					EnvironmentNumber: "1",
				},
				sess: session.Must(session.NewSession()),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.args.cfg, tt.args.sess, buildkiteAPI)
			assert.NotNil(got)
		})
	}
}

func TestAgentPool_RegisterAgents(t *testing.T) {

	paramStore := &mocks.Store{}

	paramStore.On("GetAgentKey", "/dev/1/buildkite-agent-key").Return("abc123", nil)

	agentStore := &mocks.AgentsAPI{}
	agentRecord := &store.AgentRecord{Name: "deployer-dev-1", Tags: []string{"aws", "serverless", "codebuild", "", "queue=dev"}, AgentConfig: &api.Agent{}}
	agentStore.On("CreateOrUpdate", agentRecord).Return(agentRecord, nil)

	cfg := &config.Config{
		EnvironmentName:   "dev",
		EnvironmentNumber: "1",
	}

	type fields struct {
		Agents []*AgentInstance
	}
	type apiMock struct {
		method          string
		arguments       []interface{}
		returnArguments []interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		apiMock apiMock
		wantErr bool
	}{
		{
			name: "RegisterAgents() with valid pool",
			fields: fields{
				Agents: []*AgentInstance{
					&AgentInstance{cfg: cfg, agent: &store.AgentRecord{Name: "deployer-dev-1", Tags: []string{"aws", "serverless", "codebuild", "", "queue=dev"}}},
				},
			},
			apiMock: apiMock{
				method:          "Register",
				arguments:       []interface{}{"deployer-dev-1", "abc123", []string{"aws", "serverless", "codebuild", "", "queue=dev"}},
				returnArguments: []interface{}{&api.Agent{}, nil},
			},
			wantErr: false,
		},
		{
			name: "RegisterAgents() with failed api call",
			fields: fields{
				Agents: []*AgentInstance{
					&AgentInstance{cfg: cfg, agent: &store.AgentRecord{Name: "deployer-dev-1", Tags: []string{"aws", "serverless", "codebuild", "", "queue=dev"}}},
				},
			},
			apiMock: apiMock{
				method:          "Register",
				arguments:       []interface{}{"deployer-dev-1", "abc123", []string{"aws", "serverless", "codebuild", "", "queue=dev"}},
				returnArguments: []interface{}{nil, errors.New("whoops")},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			buildkiteAPI := &mocks.API{}

			ap := &AgentPool{
				Agents:       tt.fields.Agents,
				paramStore:   paramStore,
				agentStore:   agentStore,
				buildkiteAPI: buildkiteAPI,
				cfg:          cfg,
			}

			buildkiteAPI.On(tt.apiMock.method, tt.apiMock.arguments...).Return(tt.apiMock.returnArguments...)

			err := ap.RegisterAgents(time.Now().Add(30 * time.Second))
			if (err != nil) != tt.wantErr {
				require.Error(t, err)
			}
		})
	}
}

func TestAgentPool_PollAgents(t *testing.T) {

	paramStore := &mocks.Store{}

	paramStore.On("GetAgentKey", "/dev/1/buildkite-agent-key").Return("abc123", nil)

	agentStore := &mocks.AgentsAPI{}

	cfg := &config.Config{
		EnvironmentName:   "dev",
		EnvironmentNumber: "1",
	}

	type fields struct {
		Agents []*AgentInstance
	}
	type apiMock struct {
		method          string
		arguments       []interface{}
		returnArguments []interface{}
	}
	tests := []struct {
		name         string
		fields       fields
		apiMock      []apiMock
		executorMock apiMock
		wantErr      bool
	}{
		{
			name: "PollAgents() with valid pool",
			fields: fields{
				Agents: []*AgentInstance{
					&AgentInstance{cfg: cfg, agent: &store.AgentRecord{Name: "deployer-dev-1", Tags: []string{"dev"}, AgentConfig: &api.Agent{AccessToken: "abc123"}}},
				},
			},
			apiMock: []apiMock{
				apiMock{
					method:          "Beat",
					arguments:       []interface{}{"abc123"},
					returnArguments: []interface{}{&api.Heartbeat{}, nil},
				},
				apiMock{
					method:          "Ping",
					arguments:       []interface{}{"abc123"},
					returnArguments: []interface{}{&api.Ping{Job: &api.Job{}}, nil},
				},
				apiMock{
					method:          "AcceptJob",
					arguments:       []interface{}{"abc123", mock.AnythingOfType("*api.Job")},
					returnArguments: []interface{}{&api.Job{}, nil},
				},
			},
			executorMock: apiMock{
				method:          "Ping",
				arguments:       []interface{}{"abc123"},
				returnArguments: []interface{}{&api.Ping{Job: &api.Job{}}, nil},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			buildkiteAPI := &mocks.API{}
			executor := &mocks.Executor{}

			executor.On("StartExecution", "deployer-dev-1", mock.Anything, mock.Anything).Return(nil)

			ap := &AgentPool{
				Agents:       tt.fields.Agents,
				executor:     executor,
				paramStore:   paramStore,
				agentStore:   agentStore,
				buildkiteAPI: buildkiteAPI,
				cfg:          cfg,
			}

			for _, mock := range tt.apiMock {
				buildkiteAPI.On(mock.method, mock.arguments...).Return(mock.returnArguments...)
			}

			executor.On("RunningForAgent", "deployer-dev-1").Return(0, nil)

			err := ap.PollAgents(time.Now().Add(30 * time.Second))
			if (err != nil) != tt.wantErr {
				require.Error(t, err)
			}
		})
	}
}
