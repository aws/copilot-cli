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
	PLATFORM=local DESTINATION=./bin/local/archer ./scripts/build_binary.sh

compile-windows:
	PLATFORM=Windows CGO_ENABLED=0 GOOS=windows GOARCH=386 DESTINATION=./bin/local/archer.exe ./scripts/build_binary.sh

compile-linux:
	PLATFORM=Linux CGO_ENABLED=0 GOOS=linux GOARCH=amd64 DESTINATION=./bin/local/archer-amd64 ./scripts/build_binary.sh

compile-darwin:
	PLATFORM=Darwin CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 DESTINATION=./bin/local/archer ./scripts/build_binary.sh

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
	go test -race -cover -count=1 -coverprofile ${COVERAGE} ${PACKAGES}

generate-coverage: ${COVERAGE}
	go tool cover -html=${COVERAGE}

${COVERAGE}: test

.PHONY: integ-test
integ-test: packr-build run-integ-test packr-clean

run-integ-test:
	# These tests have a long timeout as they create and teardown CloudFormation stacks.
	# Also adding count=1 so the test results aren't cached.
	# This command also targets files with the build integration tag
	# and runs tests which end in Integration.
	go test -v -count=1 -timeout 15m -tags=integration ${PACKAGES}

.PHONY: e2e-test
e2e-test: build
	# the target assumes the AWS-* environment variables are exported
	# -p: The number of test binaries that can be run in parallel
	# -parallel: Within a single test binary, how many test functions can run in parallel
	env -i PATH=$$PATH GOCACHE=$$(go env GOCACHE) GOPATH=$$(go env GOPATH) GOPROXY=direct \
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
	${GOBIN}/mockgen -source=./internal/pkg/archer/app.go -package=mocks -destination=./mocks/mock_app.go
	${GOBIN}/mockgen -source=./internal/pkg/archer/env.go -package=mocks -destination=./mocks/mock_env.go
	${GOBIN}/mockgen -source=./internal/pkg/archer/project.go -package=mocks -destination=./mocks/mock_project.go
	${GOBIN}/mockgen -source=./internal/pkg/archer/workspace.go -package=mocks -destination=./mocks/mock_workspace.go
	${GOBIN}/mockgen -source=./internal/pkg/term/progress/spinner.go -package=mocks -destination=./internal/pkg/term/progress/mocks/mock_spinner.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/progress.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_progress.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/prompter.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_prompter.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/cli.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_cli.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/completion.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_completion.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/identity.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_identity.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/app_deploy.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_projectservice.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/deploy.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_deploy.go
