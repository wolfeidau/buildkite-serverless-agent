
APPNAME ?= bk-sfn-lambdas
ENV ?= dev
ENV_NO ?= 1

default: clean build package deploy

# build the lambda binary
build:
	GOOS=linux GOARCH=amd64 go build -o submit-job ./cmd/submit-job
	GOOS=linux GOARCH=amd64 go build -o check-job ./cmd/check-job
	GOOS=linux GOARCH=amd64 go build -o complete-job ./cmd/complete-job
.PHONY: build

# clean all the things
clean:
	@rm -f ./submit-job
	@rm -f ./check-job
	@rm -f ./complete-job
	@rm -f ./handler.zip
	@rm -f ./deploy.out.yml
.PHONY: clean

# package up the lambda and upload it to S3
package:
	@echo "package lambdas into handler.zip"
	@zip -9 -r ./handler.zip submit-job check-job complete-job
	@echo "Running as: $(shell aws sts get-caller-identity --query Arn --output text)"
	@aws cloudformation package \
		--template-file deploy.sam.yml \
		--output-template-file deploy.out.yml \
		--s3-bucket $(S3_BUCKET) \
		--s3-prefix sam
.PHONY: package

# deploy the lambda
deploy:
	@aws cloudformation deploy \
		--template-file deploy.out.yml \
		--capabilities CAPABILITY_IAM \
		--stack-name $(APPNAME)-$(ENV)-$(ENV_NO) \
		--parameter-overrides EnvironmentName=$(ENV) EnvironmentNumber=$(ENV_NO)
.PHONY: deploy