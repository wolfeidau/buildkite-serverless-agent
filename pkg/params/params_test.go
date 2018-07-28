package params

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
)

func TestSSMStore_GetAgentKey(t *testing.T) {
	type fields struct {
		cfg    *config.Config
		ssmSvc ssmiface.SSMAPI
	}
	type args struct {
		agentSSMKey string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &SSMStore{
				cfg:    tt.fields.cfg,
				ssmSvc: tt.fields.ssmSvc,
			}
			got, err := st.GetAgentKey(tt.args.agentSSMKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("SSMStore.GetAgentKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SSMStore.GetAgentKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
