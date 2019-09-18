# Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

PACKAGES=./internal...
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

.PHONY: e2e-test
e2e-test:
	# -p: The number of test binaries that can be run in parallel
	# -parallel: Within a single test binary, how many test functions can run in parallel
	go test -v -p 1 -parallel 1 -tags=e2e ./e2e...

.PHONY: e2e-test-update-golden-files
e2e-test-update-golden-files:
	# use this target to update all the golden files (i.e expected responses)
	# then run `make e2e-test` afterward
	go test -v -p 1 -parallel 1 -tags=e2e ./e2e... --update

.PHONY: tools
tools:
	GOBIN=${GOBIN} go get github.com/golang/mock/mockgen

.PHONY: gen-mocks
gen-mocks: tools
	# TODO: make this more extensible?
	${GOBIN}/mockgen -source=./internal/pkg/archer/env.go -package=mocks -destination=./mocks/mock_env.go
	${GOBIN}/mockgen -source=./internal/pkg/archer/project.go -package=mocks -destination=./mocks/mock_project.go
	${GOBIN}/mockgen -source=./internal/pkg/spinner/spinner.go -package=mocks -destination=./internal/pkg/spinner/mocks/mock_spinner.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/spinner.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_spinner.go
