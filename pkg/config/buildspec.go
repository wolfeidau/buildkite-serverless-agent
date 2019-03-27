package config

// DefaultBuildSpec default buildspec used in buildkite codebuild jobs
const DefaultBuildSpec = `
version: 0.2

phases:
  install:
    commands:
      - nohup /usr/local/bin/dockerd --host=unix:///var/run/docker.sock --host=tcp://127.0.0.1:2375 --storage-driver=overlay&
      - timeout 15 sh -c "until docker info; do echo .; sleep 1; done"
  pre_build:
    commands:
      - aws ssm get-parameters --names "/${ENVIRONMENT_NAME}/${ENVIRONMENT_NUMBER}/buildkite-ssh-key" --with-decryption --output text --query 'Parameters[0].Value' > ~/.ssh/id_rsa
      - chmod 600 ~/.ssh/id_rsa
  build:
    commands:
      - echo Build started on $(date)
      - /opt/buildkite/buildkite-agent bootstrap --build-path ${CODEBUILD_SRC_DIR} --bin-path /opt/buildkite
  post_build:
    commands:
      - echo Build completed on $(date)
`
