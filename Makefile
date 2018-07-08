ENV ?= dev
ENV_NO ?= 1

AGENTSUBDIR := lambdas/agent-worker
SFNSUBDIR := lambdas/sfn

GOPATH := $(shell go env GOPATH)

SOURCE_FILES?=$$(go list ./... | grep -v /vendor/ | grep -v mocks)

default: lint test
.PHONY: default

all: setup lint test codebuild upload-buildkite-project sfn-lambda stepfunctions agent-lambda
.PHONY: all

sfn-lambda: $(SFNSUBDIR)
.PHONY: sfn-lambda

agent-lambda: $(AGENTSUBDIR)
.PHONY: agent-lambda

$(AGENTSUBDIR):
	$(MAKE) -C $@
.PHONY: $(AGENTSUBDIR)

$(SFNSUBDIR):
	$(MAKE) -C $@
.PHONY: $(SFNSUBDIR)

setup:
	@echo "setup install deps"
	@go get -u github.com/mgechev/revive
	@go get -u github.com/golang/dep/cmd/dep
	@$(GOPATH)/bin/dep ensure
.PHONY: setup

lint:
	@echo "lint all the things"
	@$(GOPATH)/bin/revive -formatter friendly $(SOURCE_FILES)
.PHONY: lint

test:
	@echo "test all the things"
	@go test -cover ./...
.PHONY: test

codebuild:
	@echo "deploy the codebuild stack"
	@aws cloudformation deploy \
		--template-file $$(pwd)/infra/codebuild/template.yml \
		--capabilities CAPABILITY_IAM \
		--stack-name bk-codebuild-$(ENV)-$(ENV_NO) \
		--parameter-overrides EnvironmentName=$(ENV) EnvironmentNumber=$(ENV_NO)
.PHONY: codebuild

upload-buildkite-project:
	@echo "upload the buildkite codebuild project to s3"
	@zip -j -9 -r ./buildkite.zip infra/codebuild/project-template/buildspec.yml
	@aws cloudformation describe-stacks --stack-name bk-codebuild-$(ENV)-$(ENV_NO) --query 'Stacks[0].Outputs[?OutputKey==`SourceBucket`].OutputValue' --output text | \
		xargs -I{} -n1 aws s3 cp ./buildkite.zip s3://{}
.PHONY: upload-buildkite-project

stepfunctions:
	@echo "deploy the stepfunctions stack"
	@aws cloudformation deploy \
		--template-file $$(pwd)/infra/stepfunctions/template.yml \
		--capabilities CAPABILITY_IAM \
		--stack-name bk-stepfunctions-$(ENV)-$(ENV_NO) \
		--parameter-overrides EnvironmentName=$(ENV) EnvironmentNumber=$(ENV_NO) SfnLambdaStack=bk-sfn-lambdas-$(ENV)-$(ENV_NO)
.PHONY: stepfunctions
