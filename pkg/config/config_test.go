package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	type fields struct {
		AwsRegion                 string
		EnvironmentName           string
		EnvironmentNumber         string
		SfnCodebuildJobMonitorArn string
		ConcurrentBuilds          string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr error
	}{
		{
			name: "should validate environment fields",
			fields: fields{
				EnvironmentName:   "dev",
				EnvironmentNumber: "1",
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				AwsRegion:                 tt.fields.AwsRegion,
				EnvironmentName:           tt.fields.EnvironmentName,
				EnvironmentNumber:         tt.fields.EnvironmentNumber,
				SfnCodebuildJobMonitorArn: tt.fields.SfnCodebuildJobMonitorArn,
			}
			err := cfg.Validate()
			assert.Equal(t, tt.wantErr, err)
		})
	}
}
