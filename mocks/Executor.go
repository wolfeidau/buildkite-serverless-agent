// Code generated by mockery v1.0.0
package mocks

import api "github.com/buildkite/agent/api"
import mock "github.com/stretchr/testify/mock"

// Executor is an autogenerated mock type for the Executor type
type Executor struct {
	mock.Mock
}

// RunningForAgent provides a mock function with given fields: agentName
func (_m *Executor) RunningForAgent(agentName string) (int, error) {
	ret := _m.Called(agentName)

	var r0 int
	if rf, ok := ret.Get(0).(func(string) int); ok {
		r0 = rf(agentName)
	} else {
		r0 = ret.Get(0).(int)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(agentName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// StartExecution provides a mock function with given fields: agentName, job, jsonData
func (_m *Executor) StartExecution(agentName string, job *api.Job, jsonData []byte) error {
	ret := _m.Called(agentName, job, jsonData)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, *api.Job, []byte) error); ok {
		r0 = rf(agentName, job, jsonData)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}