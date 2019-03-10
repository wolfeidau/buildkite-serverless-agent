APPNAME ?= bk-serverless-codebuild-agent
ENV ?= dev
ENV_NO ?= 1
DEPLOYER_NAME ?= default

VERSION := 1.1.0
BUILD_VERSION := $(shell git rev-parse --short HEAD)
GOPKG := $(shell go list -m)

SOURCE_FILES?=$$(go list ./... | grep -v /vendor/ | grep -v mocks)

LDFLAGS := -ldflags="-s -w -X '$(GOPKG)/pkg/bk.Version=$(VERSION)' -X '$(GOPKG)/pkg/bk.BuildVersion=$(BUILD_VERSION)'"

default: clean lint test build package deploy upload-buildkite-project
.PHONY: default

ci: setup lint test build upload
.PHONY: ci

setup:
	@echo "--- setup install deps"
	@GO111MODULE=off go get -v -u github.com/golangci/golangci-lint/cmd/golangci-lint
.PHONY: setup

lint:
	@echo "--- lint all the things"
	@$(shell go env GOPATH)/bin/golangci-lint run
.PHONY: lint

test:
	@echo "--- test all the things"
	@go test -cover ./...
.PHONY: test

mocks:
	mockery -dir pkg/params --all
	mockery -dir pkg/bk --all
	mockery -dir pkg/statemachine --all
	mockery -dir pkg/ssmcache --all
.PHONY: mocks

build:
	@echo "--- build all the things"
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o agent-poll ./cmd/agent-poll
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o step-handler ./cmd/step-handler
.PHONY: build

upload:
	@echo "--- upload all the things"
	@echo "--- upload all the things"
	@zip -9 -r ./handler.zip agent-poll step-handler
	@buildkite-agent artifact upload handler.zip
	@buildkite-agent artifact upload deploy.sam.yml
.PHONY: upload

clean:
	@echo "--- clean all the things"
	@rm -f ./agent-poll
	@rm -f ./step-handler
	@rm -f ./handler.zip
	@rm -f ./buildkite.zip
	@rm -f ./deploy.out.yml
.PHONY: clean

package:
	@echo "--- package lambdas into handler.zip"
	@zip -9 -r ./handler.zip agent-poll step-handler
	@echo "Running as: $(shell aws sts get-caller-identity --query Arn --output text)"
	@aws cloudformation package \
		--template-file deploy.sam.yml \
		--output-template-file deploy.out.yml \
		--s3-bucket $(S3_BUCKET) \
		--s3-prefix sam
.PHONY: package

deploy:
	@echo "--- deploy lambdas into aws"
	@aws cloudformation deploy \
		--template-file deploy.out.yml \
		--capabilities CAPABILITY_IAM \
		--stack-name $(APPNAME)-$(ENV)-$(ENV_NO) \
		--parameter-overrides EnvironmentName=$(ENV) EnvironmentNumber=$(ENV_NO) ConcurrentBuilds=2
.PHONY: deploy

upload-buildkite-project:
	@echo "--- upload the buildkite codebuild sources to s3"
	@zip -j -9 -r ./buildkite.zip codebuild-template/buildspec.yml
	@aws cloudformation describe-stacks --stack-name $(APPNAME)-$(ENV)-$(ENV_NO) --query 'Stacks[0].Outputs[?OutputKey==`SourceBucket`].OutputValue' --output text | \
		xargs -I{} -n1 aws s3 cp ./buildkite.zip s3://{}
.PHONY: upload-buildkite-project

deploy-deployer-project:
	@echo "--- deploy deployer codebuild project into aws"
	@aws cloudformation deploy \
		--template-file examples/codebuild-project.yaml \
		--capabilities CAPABILITY_IAM \
		--stack-name $(APPNAME)-$(ENV)-$(ENV_NO)-deployer-$(DEPLOYER_NAME) \
		--parameter-overrides EnvironmentName=$(ENV) EnvironmentNumber=$(ENV_NO) Name=$(DEPLOYER_NAME) \
			BuildkiteAgentPeerStack=$(APPNAME)-$(ENV)-$(ENV_NO) SourceName=buildkite-deployer.zip
.PHONY: deploy-deployer-project

upload-deployer-project:
	@echo "--- upload the deployer codebuild sources to s3"
	@zip -j -9 -r ./buildkite-deployer.zip codebuild-template/buildspec.yml
	@aws cloudformation describe-stacks --stack-name $(APPNAME)-$(ENV)-$(ENV_NO) --query 'Stacks[0].Outputs[?OutputKey==`SourceBucket`].OutputValue' --output text | \
		xargs -I{} -n1 aws s3 cp ./buildkite-deployer.zip s3://{}
.PHONY: upload-deployer-project
