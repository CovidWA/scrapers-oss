#!/bin/bash
# requires Python 3.8 to work
cd "$(dirname "$0")/.."

PYTHON=python3
if [[ ! $(python3 --version) =~ 3.8 ]]; then
   PYTHON=/usr/local/opt/python@3.8/libexec/bin/python
fi
"$PYTHON" -m venv .env

# shellcheck disable=SC1091
source .env/bin/activate
pip install -r requirements.txt
deactivate
rm -rf ./lambda_stage
mkdir -p ./lambda_stage
mkdir -p ./lambda_stage/util
cp -r .env/lib/python3.8/site-packages/* ./lambda_stage
cp -r ./util/* ./lambda_stage/util
cp -r ./*.py ./lambda_stage
cp -r ./*.csv ./lambda_stage

# to push locally uncommit the line below
# aws lambda update-function-code --function-name python-scrapers --zip-file fileb://lambda_stage/bundle.zip
