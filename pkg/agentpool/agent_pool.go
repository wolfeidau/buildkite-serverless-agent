package agentpool

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/params"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/statemachine"
)

// DefaultAgentNamePrefix serverless agent name prefix used when registering the agent
const DefaultAgentNamePrefix = "serverless-agent"

// DefaultAgentPoolSize default agent size if one is not specified
const DefaultAgentPoolSize = 5

// ActionFunc async agent action function
type ActionFunc func(agentInstance *AgentInstance, resultsChan chan *AgentResult)

// AgentResult agent result from operation
type AgentResult struct {
	Name  string // name of the agent which returned the result
	Error error
}

// AgentPool used to store a pool of agents which are created on launch
type AgentPool struct {
	Agents       []*AgentInstance
	cfg          *config.Config
	buildkiteAPI bk.API
	paramStore   params.Store
	executor     statemachine.Executor
}

// New create a new agent pool and populate it based on the poolsize
func New(cfg *config.Config, sess *session.Session, buildkiteAPI bk.API) *AgentPool {

	poolsize := DefaultAgentPoolSize

	// how many do we want..
	if cfg.AgentPoolSize > 0 {
		poolsize = cfg.AgentPoolSize
	}

	agents := make([]*AgentInstance, poolsize)

	for i := range agents {
		agents[i] = &AgentInstance{
			cfg:   cfg,
			index: i,
			tags:  []string{"aws", "serverless", "codebuild", cfg.AwsRegion, cfg.EnvironmentName},
		}
	}

	executor := statemachine.NewSFNExecutor(cfg, sess)

	ssmSvc := ssm.New(sess)
	paramStore := params.New(cfg, ssmSvc)

	return &AgentPool{
		Agents:       agents,
		buildkiteAPI: buildkiteAPI,
		cfg:          cfg,
		paramStore:   paramStore,
		executor:     executor,
	}
}

// RegisterAgents register all the agents in the pool
func (ap *AgentPool) RegisterAgents() error {
	resultsChan := dispatchAgentTasks(ap.Agents, ap.asyncRegister)
	return processResults(ap.Agents, resultsChan)
}

// PollAgents send a heartbeat to all the agents in the pool then check for jobs using ping
func (ap *AgentPool) PollAgents() error {
	resultsChan := dispatchAgentTasks(ap.Agents, ap.asyncPoll)
	return processResults(ap.Agents, resultsChan)
}

func dispatchAgentTasks(agents []*AgentInstance, action ActionFunc) chan *AgentResult {
	resultsChan := make(chan *AgentResult)

	for _, agent := range agents {
		go action(agent, resultsChan)
	}

	return resultsChan
}

func processResults(agents []*AgentInstance, resultsChan chan *AgentResult) error {

	for _ = range agents {
		result := <-resultsChan
		if result.Error != nil {
			return result.Error
		}
	}

	return nil

}

func (ap *AgentPool) getAgentKey() (string, error) {
	agentSSMKey := fmt.Sprintf("/%s/%s/buildkite-agent-key", ap.cfg.EnvironmentName, ap.cfg.EnvironmentNumber)

	log.WithField("agentSSMKey", agentSSMKey).Info("Loading buildkite key from SSM")

	agentKey, err := ap.paramStore.GetAgentKey(agentSSMKey)
	if err != nil {
		return "", errors.Wrap(err, "failed to retrieve key from ssm")
	}

	return agentKey, nil
}

func (ap *AgentPool) asyncRegister(agentInstance *AgentInstance, resultsChan chan *AgentResult) {
	resultsChan <- &AgentResult{Name: agentInstance.Name(), Error: ap.register(agentInstance)}
}

func (ap *AgentPool) register(agentInstance *AgentInstance) error {

	agentKey, err := ap.getAgentKey()
	if err != nil {
		return errors.Wrap(err, "failed to get agent key from param store")
	}

	agentConfig, err := ap.buildkiteAPI.Register(agentInstance.Name(), agentKey, agentInstance.Tags())
	if err != nil {
		return errors.Wrap(err, "failed to register agent")
	}

	err = ap.paramStore.SaveAgentConfig(agentInstance.ConfigKey(), agentConfig)
	if err != nil {
		return errors.Wrap(err, "failed to save agent config")
	}

	return nil
}

func (ap *AgentPool) asyncPoll(agentInstance *AgentInstance, resultsChan chan *AgentResult) {
	resultsChan <- &AgentResult{Name: agentInstance.Name(), Error: ap.poll(agentInstance)}
}

func (ap *AgentPool) poll(agentInstance *AgentInstance) error {

	log.WithField("agentConfigSSMKey", agentInstance.ConfigKey()).Info("Loading buildkite key from SSM")

	agentConfig, err := ap.paramStore.GetAgentConfig(agentInstance.ConfigKey())
	if err != nil {
		return errors.Wrap(err, "failed to load agent config")
	}

	// use the token from the agent config
	beat, err := ap.buildkiteAPI.Beat(agentConfig.AccessToken)
	if err != nil {
		return errors.Wrap(err, "failed to send heartbeat to buildkite")
	}

	log.Infof("Heartbeat sent at %s and received at %s", beat.SentAt, beat.ReceivedAt)

	ping, err := ap.buildkiteAPI.Ping(agentConfig.AccessToken)
	if err != nil {
		return errors.Wrap(err, "failed to ping buildkite")
	}

	log.WithField("Action", ping.Action).WithField("Message", ping.Message).Info("Received ping from buildkite api")

	if ping.Job == nil {
		log.Info("Ping to endpoint returned no job")

		return nil // we are done
	}

	count, err := ap.executor.RunningForAgent(agentInstance.Name())
	if err != nil {
		return errors.Wrap(err, "failed to list executions")
	}

	if count >= 1 {
		log.Infof("Running %d executions so not retrieving a job", count)
		return nil // we are done as there is already a job running
	}

	job, err := ap.buildkiteAPI.AcceptJob(agentConfig.AccessToken, ping.Job)
	if err != nil {
		return errors.Wrap(err, "failed to accept job from endpoint")
	}

	wd := &bk.WorkflowData{
		Job:       job,
		AgentName: agentInstance.Name(),
	}

	data, err := json.Marshal(wd)
	if err != nil {
		return errors.Wrap(err, "failed to marshal job")
	}

	err = ap.executor.StartExecution(agentInstance.Name(), job, data)
	if err != nil {
		return errors.Wrap(err, "failed to start execution")
	}

	return nil
}
