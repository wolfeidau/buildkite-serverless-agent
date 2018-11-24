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
)

func TestNew(t *testing.T) {

	buildkiteAPI := &mocks.API{}

	type args struct {
		cfg  *config.Config
		sess *session.Session
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "New() with valid pool size config",
			args: args{
				cfg: &config.Config{
					EnvironmentName:   "dev",
					EnvironmentNumber: "1",
					AgentPoolSize:     1,
				},
				sess: session.Must(session.NewSession()),
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.args.cfg, tt.args.sess, buildkiteAPI)
			require.Equal(t, tt.want, len(got.Agents))
		})
	}
}

func TestAgentPool_RegisterAgents(t *testing.T) {

	paramStore := &mocks.Store{}

	paramStore.On("GetAgentKey", "/dev/1/buildkite-agent-key").Return("abc123", nil)
	paramStore.On("SaveAgentConfig", "/dev/1/serverless-agent-dev-1_0", mock.AnythingOfType("*api.Agent")).Return(nil)

	cfg := &config.Config{
		EnvironmentName:   "dev",
		EnvironmentNumber: "1",
		AgentPoolSize:     1,
	}

	type fields struct {
		Agents   []*AgentInstance
		poolsize int
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
					&AgentInstance{cfg: cfg, index: 0, tags: []string{"dev"}},
				},
			},
			apiMock: apiMock{
				method:          "Register",
				arguments:       []interface{}{"serverless-agent-dev-1_0", "abc123", []string{"dev"}},
				returnArguments: []interface{}{&api.Agent{}, nil},
			},
			wantErr: false,
		},
		{
			name: "RegisterAgents() with failed api call",
			fields: fields{
				Agents: []*AgentInstance{
					&AgentInstance{cfg: cfg, index: 0, tags: []string{"dev"}},
				},
			},
			apiMock: apiMock{
				method:          "Register",
				arguments:       []interface{}{"serverless-agent-dev-1_0", "abc123", []string{"dev"}},
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
	paramStore.On("GetAgentConfig", "/dev/1/serverless-agent-dev-1_0").Return(&api.Agent{AccessToken: "abc123"}, nil)

	cfg := &config.Config{
		EnvironmentName:   "dev",
		EnvironmentNumber: "1",
		AgentPoolSize:     1,
	}

	type fields struct {
		Agents   []*AgentInstance
		poolsize int
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
					&AgentInstance{cfg: cfg, index: 0},
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

			executor.On("StartExecution", "serverless-agent-dev-1_0", mock.Anything, mock.Anything).Return(nil)

			ap := &AgentPool{
				Agents:       tt.fields.Agents,
				executor:     executor,
				paramStore:   paramStore,
				buildkiteAPI: buildkiteAPI,
				cfg:          cfg,
			}

			for _, mock := range tt.apiMock {
				buildkiteAPI.On(mock.method, mock.arguments...).Return(mock.returnArguments...)
			}

			executor.On("RunningForAgent", "serverless-agent-dev-1_0").Return(0, nil)

			err := ap.PollAgents(time.Now().Add(30 * time.Second))
			if (err != nil) != tt.wantErr {
				require.Error(t, err)
			}
		})
	}
}
