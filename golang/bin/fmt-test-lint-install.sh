#!/bin/bash

cd "$(dirname "$0")"
cd ./..
go fmt
go test
golangci-lint run
go build ./cmd/covidwa-scrapers-go
go install ./...