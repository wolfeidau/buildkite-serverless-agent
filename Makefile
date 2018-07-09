APPNAME ?= bk-serverless-codebuild-agent
ENV ?= dev
ENV_NO ?= 1

GOPATH := $(shell go env GOPATH)
SOURCE_FILES?=$$(go list ./... | grep -v /vendor/ | grep -v mocks)

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

# build the lambda binary
build:
	@echo "build all the things"
	GOOS=linux GOARCH=amd64 go build -o agent ./cmd/agent
	GOOS=linux GOARCH=amd64 go build -o submit-job ./cmd/submit-job
	GOOS=linux GOARCH=amd64 go build -o check-job ./cmd/check-job
	GOOS=linux GOARCH=amd64 go build -o complete-job ./cmd/complete-job
.PHONY: build

# clean all the things
clean:
	@echo "clean all the things"
	@rm -f ./agent
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
	@zip -9 -r ./handler.zip agent submit-job check-job complete-job
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
		--parameter-overrides EnvironmentName=$(ENV) EnvironmentNumber=$(ENV_NO)
.PHONY: deploy

upload-buildkite-project:
	@echo "upload the buildkite codebuild project to s3"
	@zip -j -9 -r ./buildkite.zip codebuild-template/buildspec.yml
	@aws cloudformation describe-stacks --stack-name $(APPNAME)-$(ENV)-$(ENV_NO) --query 'Stacks[0].Outputs[?OutputKey==`SourceBucket`].OutputValue' --output text | \
		xargs -I{} -n1 aws s3 cp ./buildkite.zip s3://{}
.PHONY: upload-buildkite-project
