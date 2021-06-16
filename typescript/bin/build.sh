#!/bin/bash
set -e

TAG="typescript-scrapers-latest"

export AWS_PROFILE=covidwa

aws ecr get-login-password --region us-west-2 | docker login --username AWS --password-stdin 508192331067.dkr.ecr.us-west-2.amazonaws.com

docker build \
  --file Dockerfile \
  --progress=plain \
  --tag "${TAG}" \
  .

docker tag "${TAG}" "508192331067.dkr.ecr.us-west-2.amazonaws.com/covidwa:${TAG}"
docker push "508192331067.dkr.ecr.us-west-2.amazonaws.com/covidwa:${TAG}"

aws lambda update-function-code \
  --function-name typescript-scrapers \
  --image-uri "508192331067.dkr.ecr.us-west-2.amazonaws.com/covidwa:${TAG}"
