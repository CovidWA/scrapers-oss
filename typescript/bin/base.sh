#!/bin/bash
set -e

# NOTE: Github Secrets should contain a GitHub Token with permission to write
# packages
#
# Example file
# export DOCKER_LOGIN_PAT=TBD
source "./githubsecrets.sh"

# REF: https://hub.docker.com/r/pulumi/pulumi-go/tags
TAG="quezocp/typescript-scrapers-base:v1"

docker build \
  --file Dockerfile.base \
  --progress=plain \
  --tag "${TAG}" \
  .
docker push "${TAG}"
