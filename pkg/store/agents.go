package store

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/buildkite/agent/api"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/dynalock"
)

const (
	storePartition = "Agents"
	agentPrefix    = "/agent/"
	lockPrefix     = "/locks/"
)

// AgentRecord stores the details of the agent
type AgentRecord struct {
	Name             string     `json:"name,omitempty"`
	Tags             []string   `json:"tags,omitempty"`
	CodebuildProject string     `json:"codebuild_project,omitempty"`
	Modified         time.Time  `json:"modified,omitempty"`
	AgentConfig      *api.Agent `json:"agent_config,omitempty"`
}

// AgentsAPI agents store API
type AgentsAPI interface {
	List() ([]*AgentRecord, error)
	Get(name string) (*AgentRecord, error)
	CreateOrUpdate(agent *AgentRecord) (*AgentRecord, error)
	NewLock(name string, ttl time.Duration) (dynalock.Locker, error)
}

// Agents store all configured agents
type Agents struct {
	config *config.Config
	kv     dynalock.Store
}

func NewAgents(config *config.Config, cfg ...*aws.Config) AgentsAPI {
	sess := session.Must(session.NewSession(cfg...))
	kv := dynalock.New(dynamodb.New(sess), config.AgentTableName, storePartition)
	return &Agents{
		config: config,
		kv:     kv,
	}
}

func (ag *Agents) List() ([]*AgentRecord, error) {

	pairs, err := ag.kv.List(agentPrefix)
	if err != nil {
		if err == dynalock.ErrKeyNotFound {
			return []*AgentRecord{}, nil // no agents is OK
		}
		return nil, err
	}

	agents := make([]*AgentRecord, len(pairs))

	for n, pair := range pairs {

		agent := new(AgentRecord)

		err := dynalock.UnmarshalStruct(pair.AttributeValue(), agent)
		if err != nil {
			return nil, err
		}

		agent.Name = strings.TrimPrefix(agent.Name, agentPrefix)

		agents[n] = agent
	}

	return agents, nil
}

func (ag *Agents) Get(name string) (*AgentRecord, error) {

	pair, err := ag.kv.Get(agentPrefix + name)
	if err != nil {
		return nil, err
	}
	agent := new(AgentRecord)

	err = dynalock.UnmarshalStruct(pair.AttributeValue(), agent)
	if err != nil {
		return nil, err
	}

	agent.Name = strings.TrimPrefix(agent.Name, agentPrefix)

	return agent, nil

}

func (ag *Agents) CreateOrUpdate(agent *AgentRecord) (*AgentRecord, error) {

	// set a date modified to track updates
	agent.Modified = time.Now()

	item, err := dynalock.MarshalStruct(agent)
	if err != nil {
		return nil, err
	}

	err = ag.kv.Put(
		agentPrefix+agent.Name,
		dynalock.WriteWithAttributeValue(item),
		dynalock.WriteWithNoExpires(),
	)
	if err != nil {
		return nil, err
	}

	agent.Name = strings.TrimPrefix(agent.Name, agentPrefix)

	return agent, nil
}

func (ag *Agents) NewLock(name string, ttl time.Duration) (dynalock.Locker, error) {

	renewCh := make(chan struct{})

	return ag.kv.NewLock(
		lockPrefix+name,
		dynalock.LockWithBytes([]byte(`lock`)),
		dynalock.LockWithTTL(ttl),
		dynalock.LockWithRenewLock(renewCh),
	)
}
