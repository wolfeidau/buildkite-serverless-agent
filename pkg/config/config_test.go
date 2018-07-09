package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_GetConcurrentBuilds(t *testing.T) {
	type fields struct {
		AwsRegion                 string
		EnvironmentName           string
		EnvironmentNumber         string
		SfnCodebuildJobMonitorArn string
		ConcurrentBuilds          string
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		{
			name:   "should return 1 for empty value",
			fields: fields{ConcurrentBuilds: ""},
			want:   1,
		},
		{
			name:   "should return 1 for values less than 1",
			fields: fields{ConcurrentBuilds: "0"},
			want:   1,
		},
		{
			name:   "should return same as input for valid numbers",
			fields: fields{ConcurrentBuilds: "100"},
			want:   100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				AwsRegion:                 tt.fields.AwsRegion,
				EnvironmentName:           tt.fields.EnvironmentName,
				EnvironmentNumber:         tt.fields.EnvironmentNumber,
				SfnCodebuildJobMonitorArn: tt.fields.SfnCodebuildJobMonitorArn,
				ConcurrentBuilds:          tt.fields.ConcurrentBuilds,
			}
			if got := cfg.GetConcurrentBuilds(); got != tt.want {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

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
				ConcurrentBuilds:  "0",
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
				ConcurrentBuilds:          tt.fields.ConcurrentBuilds,
			}
			err := cfg.Validate()
			assert.Equal(t, tt.wantErr, err)
		})
	}
}
