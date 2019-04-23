package agentpool

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/params"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/statemachine"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/store"
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
	agentStore   store.AgentsAPI
	executor     statemachine.Executor
}

// New create a new agent pool and populate it based on the poolsize
func New(cfg *config.Config, sess *session.Session, buildkiteAPI bk.API) *AgentPool {

	executor := statemachine.NewSFNExecutor(cfg, sess)

	paramStore := params.New(cfg)

	return &AgentPool{
		Agents:       []*AgentInstance{},
		buildkiteAPI: buildkiteAPI,
		cfg:          cfg,
		agentStore:   store.NewAgents(cfg),
		paramStore:   paramStore,
		executor:     executor,
	}
}

// LoadAgents register all the agents in the pool
func (ap *AgentPool) LoadAgents() error {

	agents, err := ap.agentStore.List()
	if err != nil {
		return err
	}

	agentsIntances := []*AgentInstance{}

	for _, agent := range agents {
		agent.Tags = []string{
			"aws",
			"serverless",
			"codebuild",
			ap.cfg.AwsRegion,
			fmt.Sprintf("queue=%s", ap.cfg.EnvironmentName),
		}
		agentsIntances = append(agentsIntances, NewAgentInstance(ap.cfg, agent))
	}

	ap.Agents = agentsIntances

	return nil
}

// RegisterAgents register all the agents in the pool
func (ap *AgentPool) RegisterAgents(deadline time.Time) error {
	resultsChan := dispatchAgentTasks(ap.Agents, ap.asyncRegister)
	return processResults(ap.Agents, deadline, resultsChan)
}

// PollAgents send a heartbeat to all the agents in the pool then check for jobs using ping
func (ap *AgentPool) PollAgents(deadline time.Time) error {
	resultsChan := dispatchAgentTasks(ap.Agents, ap.asyncPoll)
	return processResults(ap.Agents, deadline, resultsChan)
}

// CleanupAgents unlock agent name
func (ap *AgentPool) CleanupAgents(deadline time.Time) error {
	resultsChan := dispatchAgentTasks(ap.Agents, ap.asyncCleanup)
	return processResults(ap.Agents, deadline, resultsChan)
}

func dispatchAgentTasks(agents []*AgentInstance, action ActionFunc) chan *AgentResult {
	resultsChan := make(chan *AgentResult)

	for _, agent := range agents {
		go action(agent, resultsChan)
	}

	return resultsChan
}

func processResults(agents []*AgentInstance, deadline time.Time, resultsChan chan *AgentResult) error {

	timeoutChannel := time.After(time.Until(deadline))

	for range agents {
		select {
		case <-timeoutChannel:
			return fmt.Errorf("timed out during API operation")
		case result := <-resultsChan:
			if result.Error != nil {
				return result.Error
			}
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

	// load the agents key
	agentKey, err := ap.getAgentKey()
	if err != nil {
		return errors.Wrap(err, "failed to get agent key from param store")
	}

	if agentInstance.AgentConfig() != nil {
		return nil
	}

	// register a new agent
	agentConfig, err := ap.buildkiteAPI.Register(agentInstance.Name(), agentKey, agentInstance.Tags())
	if err != nil {
		return errors.Wrap(err, "failed to register agent")
	}

	agent := agentInstance.Agent()
	agent.AgentConfig = agentConfig

	agent, err = ap.agentStore.CreateOrUpdate(agent)
	if err != nil {
		return errors.Wrap(err, "failed to save agent config")
	}

	log.WithField("agentName", agent.Name).Info("updated agent config")

	return nil
}

func (ap *AgentPool) asyncPoll(agentInstance *AgentInstance, resultsChan chan *AgentResult) {
	resultsChan <- &AgentResult{Name: agentInstance.Name(), Error: ap.poll(agentInstance)}
}

func (ap *AgentPool) poll(agentInstance *AgentInstance) error {

	// use the token from the agent config
	beat, err := ap.buildkiteAPI.Beat(agentInstance.AgentConfig().AccessToken)
	if err != nil {
		return errors.Wrap(err, "failed to send heartbeat to buildkite")
	}

	log.Infof("Heartbeat sent at %s and received at %s", beat.SentAt, beat.ReceivedAt)

	ping, err := ap.buildkiteAPI.Ping(agentInstance.AgentConfig().AccessToken)
	if err != nil {
		return errors.Wrap(err, "failed to ping buildkite")
	}

	log.WithField("Action", ping.Action).WithField("Message", ping.Message).Info("Received ping from buildkite api")

	if ping.Job == nil {
		log.Info("Ping to endpoint returned no job")

		return nil // we are done
	}

	log.WithField("state", ping.Job.State).Info("Job received")

	if ping.Job.State == "canceling" {

		ping.Job.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		ping.Job.ExitStatus = "-99"

		err := ap.buildkiteAPI.FinishJob(agentInstance.AgentConfig().AccessToken, ping.Job)
		if err != nil {
			return errors.Wrap(err, "failed to finish canceling job")
		}

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

	job, err := ap.buildkiteAPI.AcceptJob(agentInstance.AgentConfig().AccessToken, ping.Job)
	if err != nil {
		return errors.Wrap(err, "failed to accept job from endpoint")
	}

	wd := &bk.WorkflowData{
		Job:       job,
		AgentName: agentInstance.Name(),
		Codebuild: &bk.CodebuildWorkflowData{
			ProjectName: agentInstance.CodebuildProject(),
		},
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

func (ap *AgentPool) asyncCleanup(agentInstance *AgentInstance, resultsChan chan *AgentResult) {
	resultsChan <- &AgentResult{Name: agentInstance.Name(), Error: ap.cleanup(agentInstance)}
}

func (ap *AgentPool) cleanup(agentInstance *AgentInstance) error {

	log.WithField("name", agentInstance.Name()).Info("cleanup renew channel")
	// close(agentInstance.renewChan)

	return nil
}

// func sliceUniqMap(s []string) []string {
// 	seen := make(map[string]struct{}, len(s))
// 	j := 0
// 	for _, v := range s {
// 		if _, ok := seen[v]; ok {
// 			continue
// 		}
// 		seen[v] = struct{}{}
// 		s[j] = v
// 		j++
// 	}
// 	return s[:j]
// }
