package ssmcache

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/mock"
	"github.com/wolfeidau/buildkite-serverless-agent/mocks"
)

func TestGetKey(t *testing.T) {

	ssmMock := &mocks.SSMAPI{}

	gpo := &ssm.GetParameterOutput{
		Parameter: &ssm.Parameter{
			Name:  aws.String("testtest"),
			Value: aws.String("sup"),
		},
	}

	ssmMock.On("GetParameter", mock.AnythingOfType("*ssm.GetParameterInput")).Return(gpo, nil)

	cache := &cache{
		ssmSvc:    ssmMock,
		ssmValues: make(map[string]*Entry),
	}

	val, err := cache.GetKey("testtest", true)
	require.Nil(t, err)
	require.Equal(t, "sup", val)
}

func TestPutKey(t *testing.T) {
	ssmMock := &mocks.SSMAPI{}

	ppo := &ssm.PutParameterOutput{
		Version: aws.Int64(1),
	}
	ssmMock.On("PutParameter", mock.AnythingOfType("*ssm.PutParameterInput")).Return(ppo, nil)
	cache := &cache{
		ssmSvc:    ssmMock,
		ssmValues: make(map[string]*Entry),
	}
	gpo := &ssm.GetParameterOutput{
		Parameter: &ssm.Parameter{
			Name:  aws.String("testtest"),
			Value: aws.String("sup"),
		},
	}

	ssmMock.On("GetParameter", mock.AnythingOfType("*ssm.GetParameterInput")).Return(gpo, nil)

	err := cache.PutKey("testtest", "sup", true)
	require.Nil(t, err)
}
