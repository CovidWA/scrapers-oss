SHELL=/usr/bin/env bash
## install: install and compile
# to compile TypeScript code
.PHONY: install
install:
	npm install
	@echo edit config.json to put a secret and server URI - ask on Slack
	[[ ! -e config.json  ]] && cp config.json.sample config.json || true

## run: compile and start 
.PHONY: run
run:
	npm run compile
	npm start         # to run the scraper locally

.PHONY: test
test:
	npm run compile
	AWS_LAMBDA_FUNCTION_NAME="something" npm start

.DEFAULT_GOAL := help
.PHONY: help
# https://swcarpentry.github.io/make-novice/08-self-doc/ is simpler just need
# and it dumpes them out relies on the variable MAKEFILE_LIST which is a list of
# all files note we do not just use $< because this is an include.mk file
## help: available commands
help: $(MAKEFILE_LIST)
	@sed -n 's/^##//p' $(MAKEFILE_LIST)
