
APPNAME ?= lambda-bk-sfn-check-job
ENV ?= dev
ENV_NO ?= 1

default: clean build package deploy

# build the lambda binary
build:
	GOOS=linux GOARCH=amd64 go build -o main ./cmd/lambda
.PHONY: build

# clean all the things
clean:
	rm -f ./main
	rm -f ./handler.zip
	rm -f ./deploy.out.yml
.PHONY: clean

# package up the lambda and upload it to S3
package:
	echo "package main into handler.zip"
	@zip -9 -r ./handler.zip main
	echo "Running as: $(shell aws sts get-caller-identity --query Arn --output text)"
	aws cloudformation package \
		--template-file deploy.sam.yml \
		--output-template-file deploy.out.yml \
		--s3-bucket $(S3_BUCKET) \
		--s3-prefix sam
.PHONY: package

# deploy the lambda
deploy:
	aws cloudformation deploy \
		--template-file deploy.out.yml \
		--capabilities CAPABILITY_IAM \
		--stack-name $(APPNAME)-$(ENV)-$(ENV_NO) \
		--parameter-overrides EnvironmentName=$(ENV) EnvironmentNumber=$(ENV_NO)
.PHONY: deploy