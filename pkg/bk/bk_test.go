package bk

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/codebuild"
	"github.com/buildkite/agent/api"
	"github.com/stretchr/testify/require"
	"github.com/wolfeidau/aws-launch/pkg/launcher"
)

func TestWorkflowData_UpdateJobExitCode(t *testing.T) {

	type fields struct {
		Codebuild *CodebuildWorkflowData
		Job       *api.Job
	}
	type results struct {
		exitCode          string
		chunksFailedCount int
	}
	tests := []struct {
		name    string
		fields  fields
		want    results
		wantErr bool
	}{
		{
			name: "check succeeded results in exitcode 0",
			fields: fields{
				Codebuild: &CodebuildWorkflowData{BuildStatus: codebuild.StatusTypeSucceeded},
				Job:       &api.Job{},
			},
			want:    results{exitCode: "0", chunksFailedCount: 0},
			wantErr: false,
		},
		{
			name: "check failed results in exitcode -1",
			fields: fields{
				Codebuild: &CodebuildWorkflowData{BuildStatus: codebuild.StatusTypeFailed},
				Job:       &api.Job{},
			},
			want:    results{exitCode: "-1", chunksFailedCount: 0},
			wantErr: false,
		},
		{
			name: "check stopped results in exitcode -2",
			fields: fields{
				Codebuild: &CodebuildWorkflowData{BuildStatus: codebuild.StatusTypeStopped},
				Job:       &api.Job{},
			},
			want:    results{exitCode: "-2", chunksFailedCount: 0},
			wantErr: false,
		},
		{
			name: "check missing codebuild results in exitcode -5",
			fields: fields{
				Job: &api.Job{},
			},
			want:    results{exitCode: "-5", chunksFailedCount: 0},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &WorkflowData{
				Codebuild: tt.fields.Codebuild,
				Job:       tt.fields.Job,
			}
			if err := evt.UpdateJobExitCode(); (err != nil) != tt.wantErr {
				require.Equal(t, tt.want.exitCode, evt.Job.ExitStatus)
				require.Equal(t, tt.want.chunksFailedCount, evt.Job.ChunksFailedCount)
				t.Errorf("WorkflowData.UpdateJobExitCode() error = %v, wantErr %v", err, tt.wantErr)
			}

		})
	}
}

func TestWorkflowData_UpdateCodebuildStatus(t *testing.T) {

	type args struct {
		buildID     string
		buildStatus string
		taskStatus  string
	}
	type fields struct {
		evt *WorkflowData
	}
	tests := []struct {
		name   string
		args   args
		fields fields
		want   *WorkflowData
	}{
		{
			name: "check update status is correct",
			fields: fields{
				evt: &WorkflowData{Codebuild: &CodebuildWorkflowData{BuildStatus: codebuild.StatusTypeSucceeded}, Job: &api.Job{}},
			},
			args: args{
				buildID:     "buildkite-dev-1:58df10ab-9dc5-4c7f-b0c3-6a02b63306ba",
				buildStatus: codebuild.StatusTypeSucceeded,
				taskStatus:  launcher.TaskSucceeded,
			},
			want: &WorkflowData{
				Codebuild: &CodebuildWorkflowData{
					BuildID:     "buildkite-dev-1:58df10ab-9dc5-4c7f-b0c3-6a02b63306ba",
					BuildStatus: codebuild.StatusTypeSucceeded,
				},
				WaitTime:   10,
				TaskStatus: launcher.TaskSucceeded,
				Job:        &api.Job{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fields.evt.UpdateCodebuildStatus(tt.args.buildID, tt.args.buildStatus, tt.args.taskStatus)
			require.Equal(t, tt.want, tt.fields.evt)
		})
	}
}
