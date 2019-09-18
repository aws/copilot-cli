# Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

PACKAGES=./pkg... ./internal...
GOBIN=${PWD}/bin/tools

all: build

.PHONY: build
build:
	CGO_ENABLED=0 go build -o ./bin/local/archer ./cmd/archer

.PHONY: test
test:
	go test -v -race -cover -count=1 ${PACKAGES}

.PHONY: integ-test
integ-test:
	go test -v -run Integration -tags integration ${PACKAGES}

.PHONY: tools
tools:
	GOBIN=${GOBIN} go get github.com/golang/mock/mockgen

.PHONY: gen-mocks
gen-mocks: tools
	# TODO: make this more extensible?
	${GOBIN}/mockgen -source=./pkg/archer/env.go -package=mocks -destination=./mocks/mock_env.go
	${GOBIN}/mockgen -source=./pkg/archer/project.go -package=mocks -destination=./mocks/mock_project.go
	${GOBIN}/mockgen -source=./pkg/spinner/spinner.go -package=mocks -destination=./pkg/spinner/mocks/mock_spinner.go
	${GOBIN}/mockgen -source=./pkg/cli/spinner.go -package=mocks -destination=./pkg/cli/mocks/mock_spinner.go
