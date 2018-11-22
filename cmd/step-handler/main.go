package main

import (
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/onrik/logrus/filename"
	log "github.com/sirupsen/logrus"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/bk"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/config"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/handlers"
	"github.com/wolfeidau/buildkite-serverless-agent/pkg/ssmcache"
)

func main() {
	log.AddHook(filename.NewHook())
	log.SetFormatter(&log.JSONFormatter{
		DisableTimestamp: true,
	})

	log.WithField("version", bk.Version).Info("step-handler starting")

	ssmcache.SetDefaultExpiry(5 * time.Minute)

	cfg, err := config.New()
	if err != nil {
		log.WithError(err).Fatal("failed to load configuration")
	}

	sess := session.Must(session.NewSession())

	bkw := handlers.New(cfg, sess, bk.NewAgentAPI())

	switch cfg.StepHandler {
	case "check-job":
		lambda.Start(bkw.HandlerCheckJob)
	case "submit-job":
		lambda.Start(bkw.HandlerSubmitJob)
	case "complete-job":
		lambda.Start(bkw.HandlerCompletedJob)
	default:
		log.WithField("StepHandler", cfg.StepHandler).Fatal("failed to locate job handler")
	}
}
