package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/onrik/logrus/filename"
	"github.com/sirupsen/logrus"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/store"
)

var (
	app        = kingpin.New("agent-cli", "A command-line agent provisioning application.")
	verbose    = app.Flag("verbose", "Verbose mode.").Short('v').Bool()
	agentTable = app.Flag("agent-table", "Dynamodb table which stores your agents.").Short('a').Envar("AGENT_TABLE_NAME").Required().String()

	createAgent        = app.Command("create-agent", "Create a new agent.")
	createAgentTags    = createAgent.Flag("tag", "Assign a tag to the agent.").Short('t').Strings()
	createAgentProject = createAgent.Arg("project", "The name of the codebuild project.").Required().String()
	buildSpec          = app.Command("build-spec", "Create a buildspec json.")
)

func main() {
	logrus.AddHook(filename.NewHook())

	command := kingpin.MustParse(app.Parse(os.Args[1:]))

	if *verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}

	cfg := &config.Config{AgentTableName: *agentTable}
	agentStore := store.NewAgents(cfg)

	switch command {
	case createAgent.FullCommand():

		rand.Seed(time.Now().UnixNano())

		agentName := namesgenerator.GetRandomName(10)

		logrus.WithField("agentName", agentName).Info("Agent name assigned")

		agentRecord := &store.AgentRecord{
			Name:             agentName,
			Tags:             updateTags(*createAgentTags),
			CodebuildProject: *createAgentProject,
		}

		agentRecord, err := agentStore.CreateOrUpdate(agentRecord)
		if err != nil {
			logrus.WithError(err).Fatal("failed to create agent")
		}

		logrus.WithField("agent", agentRecord).Info("created")
	case buildSpec.FullCommand():

		jsonSpec := map[string]string{
			"BuildSpec": config.DefaultBuildSpec,
		}

		data, err := json.Marshal(&jsonSpec)
		if err != nil {
			logrus.WithError(err).Fatal("failed to create json")
		}

		fmt.Println(string(data))
	}
}

func updateTags(tags []string) []string {

	resTags := []string{}

	for _, tag := range tags {
		if !strings.HasPrefix(tag, "queue=") {
			resTags = append(resTags, tag)
		}
	}

	return resTags
}
