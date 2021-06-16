#!/bin/bash

cd "$(dirname "$0")/.."
go clean ./...
./cmd/covidwa-scrapers-go-lambda/stage-lambda.sh
go build ./cmd/covidwa-scrapers-go
go install ./...
cd ./cmd/covidwa-scrapers-go
go build