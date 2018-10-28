APPNAME ?= bk-serverless-codebuild-agent
ENV ?= dev
ENV_NO ?= 1

VERSION := 2.0.0
BUILD_VERSION := $(shell git rev-parse --short HEAD)

SOURCE_FILES?=$$(go list ./... | grep -v /vendor/ | grep -v mocks)

LDFLAGS := -ldflags="-s -w -X $(GOPKG)/pkg/bk.Version=$(VERSION) -X $(GOPKG)/pkg/bk.BuildVersion=$(BUILD_VERSION)"

default: clean lint test build package deploy upload-buildkite-project
.PHONY: default

ci: setup lint test build
.PHONY: ci

setup:
	@echo "--- setup install deps"
	@go get -u github.com/mgechev/revive
.PHONY: setup

lint:
	@echo "--- lint all the things"
	@$(shell go env GOPATH)/bin/revive -formatter friendly $(SOURCE_FILES)
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

# build the lambda binary
build:
	@echo "--- build all the things"
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o agent-poll ./cmd/agent-poll
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o step-handler ./cmd/step-handler
.PHONY: build

# clean all the things
clean:
	@echo "--- clean all the things"
	@rm -f ./agent-poll
	@rm -f ./step-handler
	@rm -f ./handler.zip
	@rm -f ./buildkite.zip
	@rm -f ./deploy.out.yml
.PHONY: clean

# package up the lambda and upload it to S3
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

# deploy the lambda
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
		--stack-name $(APPNAME)-$(ENV)-$(ENV_NO)-deployer-project \
		--parameter-overrides EnvironmentName=$(ENV) EnvironmentNumber=$(ENV_NO) Name=deployer \
			BuildkiteAgentPeerStack=$(APPNAME)-$(ENV)-$(ENV_NO) SourceName=buildkite-deployer.zip
.PHONY: deploy-deployer-project

upload-deployer-project:
	@echo "--- upload the deployer codebuild sources to s3"
	@zip -j -9 -r ./buildkite-deployer.zip codebuild-template/buildspec.yml
	@aws cloudformation describe-stacks --stack-name $(APPNAME)-$(ENV)-$(ENV_NO) --query 'Stacks[0].Outputs[?OutputKey==`SourceBucket`].OutputValue' --output text | \
		xargs -I{} -n1 aws s3 cp ./buildkite-deployer.zip s3://{}
.PHONY: upload-deployer-project
