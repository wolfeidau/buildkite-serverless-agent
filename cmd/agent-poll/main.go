package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/onrik/logrus/filename"
	log "github.com/sirupsen/logrus"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/agentpool"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
)

func main() {
	log.AddHook(filename.NewHook())
	log.SetFormatter(&log.JSONFormatter{})

	cfg, err := config.New()
	if err != nil {
		log.WithError(err).Fatal("failed to load configuration")
	}

	err = xray.Configure(xray.Config{LogLevel: "info"})
	if err != nil {
		log.WithError(err).Fatal("failed to xray configuration")
	}

	sess := session.Must(session.NewSession())

	agentPool := agentpool.New(cfg, sess, bk.NewAgentAPI())

	err = agentPool.RegisterAgents()
	if err != nil {
		log.WithError(err).Fatal("failed to register agents")
	}

	bkw := agentpool.NewBuildkiteWorker(agentPool)

	lambda.Start(bkw.Handler)
}
