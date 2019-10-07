# Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

PACKAGES=./internal...
GOBIN=${PWD}/bin/tools
COVERAGE=coverage.out

all: build

.PHONY: build
build: packr-build compile-local packr-clean

.PHONY: release
release: packr-build compile-darwin compile-linux compile-windows packr-clean

compile-local:
	@echo "Building archer to ./bin/local/archer" &&\
	CGO_ENABLED=0 go build -o ./bin/local/archer ./cmd/archer

compile-windows:
	@echo "Building windows archer to ./bin/local/archer.exe" &&\
	CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -o ./bin/local/archer.exe ./cmd/archer

compile-linux:
	@echo "Building linux archer to ./bin/local/archer-amd64" &&\
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./bin/local/archer-amd64 ./cmd/archer

compile-darwin:
	@echo "Building darwin archer to ./bin/local/archer" &&\
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o ./bin/local/archer ./cmd/archer

packr-build: tools
	@echo "Packaging static files" &&\
	env -i PATH=$$PATH:${GOBIN} GOCACHE=$$(go env GOCACHE) GOPATH=$$(go env GOPATH) \
	go generate ./...

packr-clean: tools
	@echo "Cleaning up static files generated code" &&\
	cd templates &&\
	${GOBIN}/packr2 clean &&\
	cd ..\

.PHONY: test
test: packr-build run-unit-test packr-clean

run-unit-test:
	go test -v -race -cover -count=1 -coverprofile ${COVERAGE} ${PACKAGES}

generate-coverage: ${COVERAGE}
	go tool cover -html=${COVERAGE}

${COVERAGE}: test

.PHONY: integ-test
integ-test: packr-build run-integ-test packr-clean

run-integ-test:
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
	go test -v -p 1 -parallel 1 -tags=e2e ./e2e... -update

.PHONY: tools
tools:
	GOBIN=${GOBIN} go get github.com/golang/mock/mockgen
	GOBIN=${GOBIN} go get github.com/gobuffalo/packr/v2/packr2

.PHONY: gen-mocks
gen-mocks: tools
	# TODO: make this more extensible?
	${GOBIN}/mockgen -source=./internal/pkg/archer/env.go -package=mocks -destination=./mocks/mock_env.go
	${GOBIN}/mockgen -source=./internal/pkg/archer/project.go -package=mocks -destination=./mocks/mock_project.go
	${GOBIN}/mockgen -source=./internal/pkg/archer/workspace.go -package=mocks -destination=./mocks/mock_workspace.go
	${GOBIN}/mockgen -source=./internal/pkg/term/spinner/spinner.go -package=mocks -destination=./internal/pkg/term/spinner/mocks/mock_spinner.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/progress.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_progress.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/prompter.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_prompter.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/completion.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_completion.go
