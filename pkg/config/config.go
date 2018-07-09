package config

import (
	"strconv"

	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
)

// DefaultConcurrentBuilds default number of concurrent builds supported by this agent
const DefaultConcurrentBuilds = 1

var (
	// ErrMissingEnvironmentName missing Environment name configuration
	ErrMissingEnvironmentName = errors.New("Missing Environment Name ENV Variable")

	// ErrMissingEnvironmentNumber missing Environment number configuration
	ErrMissingEnvironmentNumber = errors.New("Missing Environment Number ENV Variable")
)

// Config for the environment
type Config struct {
	AwsRegion                 string `envconfig:"AWS_REGION"`
	EnvironmentName           string `envconfig:"ENVIRONMENT_NAME"`
	EnvironmentNumber         string `envconfig:"ENVIRONMENT_NUMBER"`
	SfnCodebuildJobMonitorArn string `envconfig:"SFN_CODEBUILD_JOB_MONITOR_ARN"`
	ConcurrentBuilds          string `envconfig:"CONCURRENT_BUILDS"`
}

// GetConcurrentBuilds get the current value of cGetConcurrentBuilds or the default if it is not provided
func (cfg *Config) GetConcurrentBuilds() int {
	c, err := strconv.Atoi(cfg.ConcurrentBuilds)
	if err != nil {
		return DefaultConcurrentBuilds
	}

	if c < 1 {
		return DefaultConcurrentBuilds
	}

	return c
}

// Validate checks the presence of the loaded template path on the filesystem
func (cfg *Config) Validate() error {
	if cfg.EnvironmentName == "" {
		return ErrMissingEnvironmentName
	}
	if cfg.EnvironmentNumber == "" {
		return ErrMissingEnvironmentNumber
	}

	return nil
}

// New instantiates a Config object with the set environmental variables
func New() (*Config, error) {
	cfg := new(Config)
	err := envconfig.Process("", cfg)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse environment config")
	}
	return cfg, nil
}
