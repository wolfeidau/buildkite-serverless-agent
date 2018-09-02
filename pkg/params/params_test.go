package params

import (
	"testing"

	"github.com/buildkite/agent/api"
	"github.com/stretchr/testify/mock"
	"github.com/tj/assert"
	"github.com/wolfeidau/buildkite-serverless-agent/mocks"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
)

func TestSSMStore_GetAgentKey(t *testing.T) {
	ssmCacheMock := &mocks.Cache{}

	ssmCacheMock.On("GetKey", "abc123", true).Return("qer567", nil)

	ssmStore := &SSMStore{
		cfg:      &config.Config{},
		ssmCache: ssmCacheMock,
	}

	res, err := ssmStore.GetAgentKey("abc123")
	assert.Nil(t, err)
	assert.Equal(t, "qer567", res)
}

func TestSSMStore_GetAgentConfig(t *testing.T) {

	ssmCacheMock := &mocks.Cache{}

	ssmCacheMock.On("GetKey", "abc123", true).Return("{}", nil)

	ssmStore := &SSMStore{
		cfg:      &config.Config{},
		ssmCache: ssmCacheMock,
	}

	res, err := ssmStore.GetAgentConfig("abc123")
	assert.Nil(t, err)
	assert.Equal(t, &api.Agent{}, res)
}

func TestSSMStore_SaveAgentConfig(t *testing.T) {
	ssmCacheMock := &mocks.Cache{}

	ssmCacheMock.On("PutKey", "abc123", mock.AnythingOfType("string"), true).Return(nil)
	ssmCacheMock.On("GetKey", "abc123", true).Return("{}", nil)

	ssmStore := &SSMStore{
		cfg:      &config.Config{},
		ssmCache: ssmCacheMock,
	}

	err := ssmStore.SaveAgentConfig("abc123", &api.Agent{})
	assert.Nil(t, err)
}
