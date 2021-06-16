# copied from https://github.com/richtong/lib/include.python.mk
name ?= covidwa
PYTHON ?= 3.9
SHELL ?= /bin/bash
MAIN ?= ScrapeAllAndSend.py
MAIN_PATH ?= $(PWD)

# this is not yet a module so no name
IS_MODULE ?= False
ifeq ($(IS_MODULE),True)
MODULE ?= -m $(MAIN)
else
MODULE ?= $(MAIN)
endif

FLAGS ?=

RUN := conda run -n $(name)
INIT := eval "$$(conda shell.bash hook)"
ACTIVATE := $(INIT) && conda activate $(name)
UPDATE := conda update --all -y
INSTALL := conda install -y -n $(name)
MAIN := ScrapeAllAndSend.py

## install: The default is into Anaconda
.PHONY: install
install:
	conda env list | grep ^$(name) || conda create -y --name $(name)
	conda config --env --add channels conda-forge
	conda config --env --set channel_priority strict
	conda install --name $(name) -y python=$(PYTHON)
	[[ -r environment.yml ]] && conda env update --name $(name) -f environment.yml || true
	[[ -r requirements.txt ]] && \
		grep -v "^#" requirements.txt| \
			(while read requirement; do \
				if ! conda install --name $(name) -y "$$requirement"; then \
					$(ACTIVATE) && pip install "$$requirement"; \
				fi; \
			done)
	# https://docs.conda.io/projects/conda/en/latest/user-guide/tasks/manage-environments.html#setting-environment-variables
	conda env config vars set PYTHONNOUSERSITE=true --name $(name)
	@echo WARNING -- we do not parse the PYthon User site in ~/.

## main: run the main program
.PHONY: main
main:
	$(RUN) python $(MODULE) $(FLAGS)

## pdb: run locally with python to test components from main
.PHONY: pdb
pdb:
	$(ACTIVATE) && python -m pdb $(MODULE) $(FLAGS)

## debug: run with debug model on for main
.PHONY: debug
debug:
	$(RUN) python -d $(MODULE) $(FLAGS)

# https://docs.github.com/en/actions/guides/building-and-testing-python
# https://pytest-cov.readthedocs.io/en/latest/config.html
# https://docs.pytest.org/en/stable/usage.html
# https://pypi.org/project/pytest-timeout/
# The lambda directory causes problems
## test: unit test
.PHONY: test
test:
	pytest --timeout=600 --ignore=lambda_stage --doctest-modules --cov=.

## test-ci: product junit for consumption by ci server
# --doctest-modules --cove measure for a particular path
# --junitxml is readable by Jenkins and CI servers
.PHONY: test-ci
test-ci:
	pytest --ignore=lambda_stage -cov-=. --doctest-modules \
		--junitxml=junit/test-results.xml --cov-report=xml --cov-report=html

# build: run the lambda build script
.PHONY: build
build:
	cd bin && ./build.sh

# lambda: run lambda locally
.PHONY: lambda
lambda:
	docker run --rm -v lambda_stage:/var/task:ro,delegated \
		lambci/lambda:python3.8 handler

.DEFAULT_GOAL := help
.PHONY: help
# https://swcarpentry.github.io/make-novice/08-self-doc/ is simpler just need
# and it dumpes them out relies on the variable MAKEFILE_LIST which is a list of
# all files note we do not just use $< because this is an include.mk file
## help: available commands
help: $(MAKEFILE_LIST)
	@sed -n 's/^##//p' $(MAKEFILE_LIST)
