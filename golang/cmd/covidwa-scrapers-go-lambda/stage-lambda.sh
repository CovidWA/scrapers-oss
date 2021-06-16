#!/bin/bash
STAGE_DIR="../../lambda_stage"
cd "$(dirname "$0")"
go build
rm -rf $STAGE_DIR
mkdir -p $STAGE_DIR
cp ../../*.yaml $STAGE_DIR
cp ./covidwa-scrapers-go-lambda $STAGE_DIR

