# Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

PACKAGES=./internal... ./cmd/archer/internal...
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
e2e-test: build test
	# the target assumes the AWS-* environment variables are exported
	# -p: The number of test binaries that can be run in parallel
	# -parallel: Within a single test binary, how many test functions can run in parallel
	env -i PATH=$$PATH GOCACHE=$$(go env GOCACHE) GOPATH=$$(go env GOPATH) \
	AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} \
	AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} \
	AWS_SESSION_TOKEN=${AWS_SESSION_TOKEN} \
	AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION} \
	go test -v -p 1 -parallel 1 -tags=e2e ./e2e...

.PHONY: e2e-test-update-golden-files
e2e-test-update-golden-files:
	# CAUTION: only use this target when the archer CLI output changes
	# (for example, a new command is added) and the golden files
	# (i.e. the expected responses from CLI) need to be updated.
	# The normal flow is the following:
	#
	# make e2e-test-update-golden-files // this is expected to fail but will update the golden files
	# make e2e-test // this should pass because the golden files were updated
	go test -v -p 1 -parallel 1 -tags=e2e ./e2e... --update

.PHONY: tools
tools:
	GOBIN=${GOBIN} go get github.com/golang/mock/mockgen

.PHONY: gen-mocks
gen-mocks: tools
	# TODO: make this more extensible?
	${GOBIN}/mockgen -source=./internal/pkg/archer/env.go -package=mocks -destination=./mocks/mock_env.go
	${GOBIN}/mockgen -source=./internal/pkg/archer/project.go -package=mocks -destination=./mocks/mock_project.go
	${GOBIN}/mockgen -source=./internal/pkg/archer/workspace.go -package=mocks -destination=./mocks/mock_workspace.go
	${GOBIN}/mockgen -source=./internal/pkg/spinner/spinner.go -package=mocks -destination=./internal/pkg/spinner/mocks/mock_spinner.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/spinner.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_spinner.go
