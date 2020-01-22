# Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

BINARY_NAME=ecs-preview
PACKAGES=./internal...
SOURCE_CUSTOM_RESOURCES=${PWD}/cf-custom-resources
BUILT_CUSTOM_RESOURCES=${PWD}/templates/custom-resources
GOBIN=${PWD}/bin/tools
COVERAGE=coverage.out

DESTINATION=./bin/local/${BINARY_NAME}
VERSION=$(shell git describe --always --tags)

BINARY_S3_BUCKET_PATH=https://ecs-cli-v2-release.s3.amazonaws.com

BUILD_DATE=$(shell date -u '+%Y%m%d')
SHORT_SHA1=$(shell git log -1 --pretty=%h)

LINKER_FLAGS=-X github.com/datadotworld/dev-tools/go-common/_version.VersionDate=${BUILD_DATE} -X github.com/datadotworld/dev-tools/go-common/_version.VersionCommit=${SHORT_SHA1}
# RELEASE_BUILD_LINKER_FLAGS disables DWARF and symbol table generation to reduce binary size
RELEASE_BUILD_LINKER_FLAGS=-s -w

all: build

.PHONY: build
build: packr-build compile-local packr-clean

.PHONY: release
release: packr-build compile-darwin compile-linux compile-windows packr-clean

.PHONY: release-docker
release-docker:
	docker build -t aws/amazon-ecs-cli-v2 . &&\
	docker create -ti --name amazon-ecs-cli-v2-builder aws/amazon-ecs-cli-v2 &&\
	docker cp amazon-ecs-cli-v2-builder:/aws-amazon-ecs-cli-v2/bin/local/ . &&\
	docker rm -f amazon-ecs-cli-v2-builder
	@echo "Built binaries under ./local/"

compile-local:
	go build -ldflags "${LINKER_FLAGS}" -o ${DESTINATION} ./cmd/ecs-preview

compile-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "${LINKER_FLAGS} ${RELEASE_BUILD_LINKER_FLAGS}" -o ${DESTINATION}.exe ./cmd/ecs-preview

compile-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${LINKER_FLAGS} ${RELEASE_BUILD_LINKER_FLAGS}" -o ${DESTINATION}-amd64 ./cmd/ecs-preview

compile-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "${LINKER_FLAGS} ${RELEASE_BUILD_LINKER_FLAGS}" -o ${DESTINATION} ./cmd/ecs-preview

packr-build: tools package-custom-resources
	@echo "Packaging static files" &&\
	env -i PATH="$$PATH":${GOBIN} GOCACHE=$$(go env GOCACHE) GOPATH=$$(go env GOPATH) \
	go generate ./...

packr-clean: tools package-custom-resources-clean
	@echo "Cleaning up static files generated code" &&\
	cd templates &&\
	${GOBIN}/packr2 clean &&\
	cd ..\

.PHONY: test
test: packr-build run-unit-test custom-resource-tests packr-clean

custom-resource-tests:
	@echo "Running custom resource unit tests" &&\
	cd ${SOURCE_CUSTOM_RESOURCES} &&\
	npm test &&\
	cd ..

# Minifies the resources in cf-custom-resources/lib and copies
# those minified assets into templates/custom-resources so that
# they can be packed.
package-custom-resources:
	@echo "Packaging custom resources to templates/custom-resources" &&\
	cd ${SOURCE_CUSTOM_RESOURCES} &&\
	npm run package &&\
	cd ..

# We only need the minified custom resources during building. After
# they're packed, we can remove them.
package-custom-resources-clean:
	@echo "Removing minified templates/custom-resources" &&\
	rm ${BUILT_CUSTOM_RESOURCES}/*.js

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
	go test -v -count=1 -timeout 60m -tags=integration ${PACKAGES}

.PHONY: e2e-test
e2e-test: build
	# the target assumes the AWS-* environment variables are exported
	# -p: The number of test binaries that can be run in parallel
	# -parallel: Within a single test binary, how many test functions can run in parallel
	env -i PATH="$$PATH" GOCACHE=$$(go env GOCACHE) GOPATH=$$(go env GOPATH) GOPROXY=direct \
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
	@echo "Installing custom resource dependencies" &&\
	cd ${SOURCE_CUSTOM_RESOURCES} && npm install

.PHONY: gen-mocks
gen-mocks: tools
	# TODO: make this more extensible?
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/cli/mocks/mock_rg.go -source=./internal/pkg/cli/env_delete.go resourceGetter
	${GOBIN}/mockgen -source=./internal/pkg/archer/app.go -package=mocks -destination=./mocks/mock_app.go
	${GOBIN}/mockgen -source=./internal/pkg/archer/env.go -package=mocks -destination=./mocks/mock_env.go
	${GOBIN}/mockgen -source=./internal/pkg/archer/project.go -package=mocks -destination=./mocks/mock_project.go
	${GOBIN}/mockgen -source=./internal/pkg/archer/secret.go -package=mocks -destination=./mocks/mock_secret.go
	${GOBIN}/mockgen -source=./internal/pkg/archer/workspace.go -package=mocks -destination=./mocks/mock_workspace.go
	${GOBIN}/mockgen -source=./internal/pkg/term/progress/spinner.go -package=mocks -destination=./internal/pkg/term/progress/mocks/mock_spinner.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/progress.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_progress.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/prompter.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_prompter.go
	${GOBIN}/mockgen -source=./internal/pkg/aws/route53/route53.go -package=mocks -destination=./internal/pkg/aws/mocks/mock_route53.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/cli.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_cli.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/completion.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_completion.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/identity.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_identity.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/deploy.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_deploy.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/cli/mocks/mock_iam.go github.com/aws/aws-sdk-go/service/iam/iamiface IAMAPI
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_describe.go -source=./internal/pkg/describe/webapp.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/ecr/mocks/mock_ecr.go -source=./internal/pkg/aws/ecr/ecr.go
	${GOBIN}/mockgen -source=./internal/pkg/build/docker/docker.go -package=mocks -destination=./internal/pkg/build/docker/mocks/mock_docker.go