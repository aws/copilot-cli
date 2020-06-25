# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

BINARY_NAME=copilot
PACKAGES=./internal...
SOURCE_CUSTOM_RESOURCES=${PWD}/cf-custom-resources
BUILT_CUSTOM_RESOURCES=${PWD}/templates/custom-resources
GOBIN=${PWD}/bin/tools
COVERAGE=coverage.out

DESTINATION=./bin/local/${BINARY_NAME}
VERSION=$(shell git describe --always --tags)

BINARY_S3_BUCKET_PATH=https://ecs-cli-v2-release.s3.amazonaws.com

LINKER_FLAGS=-X github.com/aws/amazon-ecs-cli-v2/internal/pkg/version.Version=${VERSION}\
-X github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli.binaryS3BucketPath=${BINARY_S3_BUCKET_PATH}
# RELEASE_BUILD_LINKER_FLAGS disables DWARF and symbol table generation to reduce binary size
RELEASE_BUILD_LINKER_FLAGS=-s -w

all: build

.PHONY: build
build: packr-build compile-local packr-clean

.PHONY: build-e2e
build-e2e: packr-build compile-linux packr-clean

.PHONY: release
release: packr-build compile-darwin compile-linux compile-windows packr-clean

.PHONY: release-docker
release-docker:
	docker build -t aws/copilot . &&\
	docker create -ti --name amazon-ecs-copilot-builder aws/copilot &&\
	docker cp amazon-ecs-copilot-builder:/copilot/bin/local/ . &&\
	docker rm -f amazon-ecs-copilot-builder
	@echo "Built binaries under ./local/"

compile-local:
	go build -ldflags "${LINKER_FLAGS}" -o ${DESTINATION} ./cmd/copilot

compile-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "${LINKER_FLAGS} ${RELEASE_BUILD_LINKER_FLAGS}" -o ${DESTINATION}.exe ./cmd/copilot

compile-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${LINKER_FLAGS} ${RELEASE_BUILD_LINKER_FLAGS}" -o ${DESTINATION}-amd64 ./cmd/copilot

compile-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "${LINKER_FLAGS} ${RELEASE_BUILD_LINKER_FLAGS}" -o ${DESTINATION} ./cmd/copilot

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
	go test -count=1 -timeout 60m -tags=integration ${PACKAGES}

.PHONY: e2e
e2e: build-e2e
	@echo "Building E2E Docker Image" &&\
	docker build -t copilot/e2e . -f e2e/Dockerfile
	@echo "Running E2E Tests" &&\
	docker run --privileged -v ${HOME}/.aws:/home/.aws -e "HOME=/home" copilot/e2e:latest

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
	${GOBIN}/mockgen -source=./internal/pkg/term/progress/spinner.go -package=mocks -destination=./internal/pkg/term/progress/mocks/mock_spinner.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/progress.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_progress.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/prompter.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_prompter.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/interfaces.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_interfaces.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/completion.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_completion.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/identity.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_identity.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_lb_web_service.go -source=./internal/pkg/describe/lb_web_service.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_service.go -source=./internal/pkg/describe/service.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_describe.go -source=./internal/pkg/describe/describe.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_env.go -source=./internal/pkg/describe/env.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_stack.go -source=./internal/pkg/describe/stack.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_status.go -source=./internal/pkg/describe/status.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_pipeline.go -source=./internal/pkg/describe/pipeline.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_pipeline_status.go -source=./internal/pkg/describe/pipeline_status.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/ecr/mocks/mock_ecr.go -source=./internal/pkg/aws/ecr/ecr.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/ecs/mocks/mock_ecs.go -source=./internal/pkg/aws/ecs/ecs.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/identity/mocks/mock_identity.go -source=./internal/pkg/aws/identity/identity.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/route53/mocks/mock_route53.go -source=./internal/pkg/aws/route53/route53.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/secretsmanager/mocks/mock_secretsmanager.go -source=./internal/pkg/aws/secretsmanager/secretsmanager.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/codepipeline/mocks/mock_codepipeline.go -source=./internal/pkg/aws/codepipeline/codepipeline.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/cloudwatch/mocks/mock_cloudwatch.go -source=./internal/pkg/aws/cloudwatch/cloudwatch.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/resourcegroups/mocks/mock_resourcegroups.go -source=./internal/pkg/aws/resourcegroups/resourcegroups.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/cloudwatchlogs/mocks/mock_cloudwatchlogs.go -source=./internal/pkg/aws/cloudwatchlogs/cloudwatchlogs.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/s3/mocks/mock_s3.go -source=./internal/pkg/aws/s3/s3.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/cloudformation/mocks/mock_cloudformation.go -source=./internal/pkg/aws/cloudformation/interfaces.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/cloudformation/stackset/mocks/mock_stackset.go -source=./internal/pkg/aws/cloudformation/stackset/stackset.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/addon/mocks/mock_addons.go -source=./internal/pkg/addon/addons.go
	${GOBIN}/mockgen -package=mocks -source=./internal/pkg/docker/docker.go -destination=./internal/pkg/docker/mocks/mock_docker.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/deploy/cloudformation/mocks/mock_cloudformation.go -source=./internal/pkg/deploy/cloudformation/cloudformation.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/deploy/cloudformation/stack/mocks/mock_lb_web_svc.go -source=./internal/pkg/deploy/cloudformation/stack/lb_web_svc.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/deploy/cloudformation/stack/mocks/mock_backend_svc.go -source=./internal/pkg/deploy/cloudformation/stack/backend_svc.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/template/mocks/mock_template.go -source=./internal/pkg/template/template.go
