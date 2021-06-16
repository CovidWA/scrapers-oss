#!/bin/bash
set -e

./bin/build.sh

echo "Running Docker Image"
docker run -p 9000:8080 covidwa/typescript-scrapers:latest

