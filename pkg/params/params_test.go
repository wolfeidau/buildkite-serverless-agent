package params

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
