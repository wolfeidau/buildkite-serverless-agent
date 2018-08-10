APPNAME ?= bk-serverless-codebuild-agent
ENV ?= dev
ENV_NO ?= 1

VERSION := 2.0.0
BUILD_VERSION := $(shell git rev-parse --short HEAD)

GOPATH := $(shell go env GOPATH)
GOPKG := github.com/wolfeidau/buildkite-serverless-agent
SOURCE_FILES?=$$(go list ./... | grep -v /vendor/ | grep -v mocks)

LDFLAGS := -ldflags="-s -w -X $(GOPKG)/pkg/bk.Version=$(VERSION) -X $(GOPKG)/pkg/bk.BuildVersion=$(BUILD_VERSION)"

default: clean lint test build package deploy upload-buildkite-project
.PHONY: default

ci: setup lint test build
.PHONY: ci

setup:
	@echo "setup install deps"
	@go get -u github.com/mgechev/revive
	@go get -u github.com/golang/dep/cmd/dep
	@$(GOPATH)/bin/dep ensure -v
.PHONY: setup

lint:
	@echo "lint all the things"
	@$(GOPATH)/bin/revive -formatter friendly $(SOURCE_FILES)
.PHONY: lint

test:
	@echo "test all the things"
	@go test -cover ./...
.PHONY: test

mocks:
	mockery -dir pkg/params --all
	mockery -dir pkg/bk --all
	mockery -dir pkg/statemachine --all
.PHONY: mocks

# build the lambda binary
build:
	@echo "build all the things"
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o agent-poll ./cmd/agent-poll
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o submit-job ./cmd/submit-job
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o check-job ./cmd/check-job
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o complete-job ./cmd/complete-job
.PHONY: build

# clean all the things
clean:
	@echo "clean all the things"
	@rm -f ./agent-poll
	@rm -f ./submit-job
	@rm -f ./check-job
	@rm -f ./complete-job
	@rm -f ./handler.zip
	@rm -f ./buildkite.zip
	@rm -f ./deploy.out.yml
.PHONY: clean

# package up the lambda and upload it to S3
package:
	@echo "package lambdas into handler.zip"
	@zip -9 -r ./handler.zip agent-poll submit-job check-job complete-job
	@echo "Running as: $(shell aws sts get-caller-identity --query Arn --output text)"
	@aws cloudformation package \
		--template-file deploy.sam.yml \
		--output-template-file deploy.out.yml \
		--s3-bucket $(S3_BUCKET) \
		--s3-prefix sam
.PHONY: package

# deploy the lambda
deploy:
	@echo "deploy lambdas into aws"
	@aws cloudformation deploy \
		--template-file deploy.out.yml \
		--capabilities CAPABILITY_IAM \
		--stack-name $(APPNAME)-$(ENV)-$(ENV_NO) \
		--parameter-overrides EnvironmentName=$(ENV) EnvironmentNumber=$(ENV_NO) ConcurrentBuilds=2
.PHONY: deploy

upload-buildkite-project:
	@echo "upload the buildkite codebuild sources to s3"
	@zip -j -9 -r ./buildkite.zip codebuild-template/buildspec.yml
	@aws cloudformation describe-stacks --stack-name $(APPNAME)-$(ENV)-$(ENV_NO) --query 'Stacks[0].Outputs[?OutputKey==`SourceBucket`].OutputValue' --output text | \
		xargs -I{} -n1 aws s3 cp ./buildkite.zip s3://{}
.PHONY: upload-buildkite-project

deploy-deployer-project:
	@echo "deploy deployer codebuild project into aws"
	@aws cloudformation deploy \
		--template-file examples/codebuild-project.yaml \
		--capabilities CAPABILITY_IAM \
		--stack-name $(APPNAME)-$(ENV)-$(ENV_NO)-deployer-project \
		--parameter-overrides EnvironmentName=$(ENV) EnvironmentNumber=$(ENV_NO) Name=deployer \
			BuildkiteAgentPeerStack=$(APPNAME)-$(ENV)-$(ENV_NO) SourceName=buildkite-deployer.zip
.PHONY: deploy-deployer-project

upload-deployer-project:
	@echo "upload the deployer codebuild sources to s3"
	@zip -j -9 -r ./buildkite-deployer.zip codebuild-template/buildspec.yml
	@aws cloudformation describe-stacks --stack-name $(APPNAME)-$(ENV)-$(ENV_NO) --query 'Stacks[0].Outputs[?OutputKey==`SourceBucket`].OutputValue' --output text | \
		xargs -I{} -n1 aws s3 cp ./buildkite-deployer.zip s3://{}
.PHONY: upload-deployer-project
