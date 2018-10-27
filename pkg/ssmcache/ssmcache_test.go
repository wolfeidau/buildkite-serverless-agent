package ssmcache

import (
	"time"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/mock"
	"github.com/wolfeidau/buildkite-serverless-agent/mocks"
)

func TestGetKey(t *testing.T) {

	ssmMock := &mocks.SSMAPI{}

	modDate := aws.Time(time.Now())
	
	gpo := &ssm.GetParameterOutput{
		Parameter: &ssm.Parameter{
			Name:  aws.String("testtest"),
			Value: aws.String("sup"),
			LastModifiedDate: modDate,
		},
	}

	ssmMock.On("GetParameter", mock.AnythingOfType("*ssm.GetParameterInput")).Return(gpo, nil)

	dpo := &ssm.DescribeParametersOutput{
		Parameters: []*ssm.ParameterMetadata {
			&ssm.ParameterMetadata{
				LastModifiedDate: modDate,
			},
		},
	}

	ssmMock.On("DescribeParameters", mock.AnythingOfType("*ssm.DescribeParametersInput")).Return(dpo, nil)

	cache := &cache{
		ssmSvc:    ssmMock,
		ssmValues: make(map[string]*Entry),
	}

	SetDefaultExpiry(1 * time.Second)

	val, err := cache.GetKey("testtest", true)
	require.Nil(t, err)
	require.Equal(t, "sup", val)
	require.Len(t, ssmMock.Calls, 1)

	ssmMock.Calls = []mock.Call{}
	time.Sleep(1 * time.Second)

	val, err = cache.GetKey("testtest", true)
	require.Nil(t, err)
	require.Equal(t, "sup", val)
	require.Len(t, ssmMock.Calls, 1)

	ssmMock.Calls = []mock.Call{}
	time.Sleep(1 * time.Second)

	// simulate an update of key where a subsequent change ot the parameter will 
	// trigger retrieval from SSM
	gpo.Parameter.LastModifiedDate = aws.Time(time.Now())
	dpo.Parameters[0].LastModifiedDate = aws.Time(time.Now())
	val, err = cache.GetKey("testtest", true)
	require.Nil(t, err)
	require.Equal(t, "sup", val)
	require.Len(t, ssmMock.Calls, 2)
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
