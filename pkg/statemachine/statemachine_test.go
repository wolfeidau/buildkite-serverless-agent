package statemachine

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/buildkite/agent/api"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/wolfeidau/buildkite-serverless-agent/mocks"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
)

var (
	cfg = &config.Config{
		EnvironmentName:           "dev",
		EnvironmentNumber:         "1",
		SfnCodebuildJobMonitorArn: "test",
	}

	agentName = "test-agent-dev-1"
)

func TestSFNExecutor_RunningForAgent(t *testing.T) {
	type fields struct {
		cfg *config.Config
	}
	type args struct {
		agentName string
	}
	type sfnMock struct {
		method          string
		arguments       []interface{}
		returnArguments []interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		sfnMock sfnMock
		args    args
		want    int
		wantErr bool
	}{
		{
			name:   "RunningForAgent() with valid agent",
			fields: fields{cfg: cfg},
			args:   args{agentName},
			sfnMock: sfnMock{
				method:    "ListExecutions",
				arguments: []interface{}{mock.Anything},
				returnArguments: []interface{}{
					&sfn.ListExecutionsOutput{
						Executions: []*sfn.ExecutionListItem{
							&sfn.ExecutionListItem{
								ExecutionArn: aws.String("test"),
								Name:         aws.String("test_test-agent-dev-1_test"),
							},
						},
					},
					nil,
				},
			},
			want:    1,
			wantErr: false,
		},
		{
			name:   "RunningForAgent() with aws api failure",
			fields: fields{cfg: cfg},
			args:   args{agentName},
			sfnMock: sfnMock{
				method:    "ListExecutions",
				arguments: []interface{}{mock.Anything},
				returnArguments: []interface{}{
					nil, errors.New("woops"),
				},
			},
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			sfnSvc := &mocks.SFNAPI{}

			sfne := &SFNExecutor{
				cfg:    tt.fields.cfg,
				sfnSvc: sfnSvc,
			}

			sfnSvc.On(tt.sfnMock.method, tt.sfnMock.arguments...).Return(tt.sfnMock.returnArguments...)

			got, err := sfne.RunningForAgent(tt.args.agentName)
			if (err != nil) != tt.wantErr {
				t.Errorf("SFNExecutor.RunningForAgent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSFNExecutor_StartExecution(t *testing.T) {
	type fields struct {
		cfg *config.Config
	}
	type args struct {
		agentName string
		job       *api.Job
		jsonData  []byte
	}
	type sfnMock struct {
		method          string
		arguments       []interface{}
		returnArguments []interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		sfnMock sfnMock
		wantErr bool
	}{
		{
			name:   "RunningForAgent() with valid job",
			fields: fields{cfg: cfg},
			args: args{
				agentName: agentName,
				job: &api.Job{
					Env: map[string]string{
						"BUILDKITE_PIPELINE_SLUG": "testingtestingtestingtestingtestingtestingtestingtestingtestingtesting",
					},
				},
				jsonData: []byte{},
			},
			sfnMock: sfnMock{
				method:    "StartExecution",
				arguments: []interface{}{mock.Anything},
				returnArguments: []interface{}{
					&sfn.StartExecutionOutput{
						ExecutionArn: aws.String("test"),
					},
					nil,
				},
			},
		},
		{
			name:   "RunningForAgent() with aws api failure",
			fields: fields{cfg: cfg},
			args: args{
				agentName: agentName,
				job: &api.Job{
					Env: map[string]string{
						"BUILDKITE_PIPELINE_SLUG": "testingtestingtestingtestingtestingtestingtestingtestingtestingtesting",
					},
				},
				jsonData: []byte{},
			},
			sfnMock: sfnMock{
				method:    "StartExecution",
				arguments: []interface{}{mock.Anything},
				returnArguments: []interface{}{
					nil,
					errors.New("woops"),
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			sfnSvc := &mocks.SFNAPI{}

			sfne := &SFNExecutor{
				cfg:    tt.fields.cfg,
				sfnSvc: sfnSvc,
			}

			sfnSvc.On(tt.sfnMock.method, tt.sfnMock.arguments...).Return(tt.sfnMock.returnArguments...)

			if err := sfne.StartExecution(tt.args.agentName, tt.args.job, tt.args.jsonData); (err != nil) != tt.wantErr {
				t.Errorf("SFNExecutor.StartExecution() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
