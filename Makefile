# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

BINARY_NAME=copilot
PACKAGES=./internal...
ROOT_SRC_DIR=${PWD}
SOURCE_CUSTOM_RESOURCES=${ROOT_SRC_DIR}/cf-custom-resources
TEMPLATES_DIR=${ROOT_SRC_DIR}/internal/pkg/template/templates
BUILT_CUSTOM_RESOURCES=${TEMPLATES_DIR}/custom-resources
GOBIN=${ROOT_SRC_DIR}/bin/tools
COVERAGE=coverage.out

DESTINATION=./bin/local/${BINARY_NAME}
VERSION=$(shell git describe --always --tags | sed 's/-/+/')

BINARY_S3_BUCKET_PATH=https://ecs-cli-v2-release.s3.amazonaws.com

LINKER_FLAGS=-X github.com/aws/copilot-cli/internal/pkg/version.Version=${VERSION}\
-X github.com/aws/copilot-cli/internal/pkg/cli.binaryS3BucketPath=${BINARY_S3_BUCKET_PATH}
# RELEASE_BUILD_LINKER_FLAGS disables DWARF and symbol table generation to reduce binary size
RELEASE_BUILD_LINKER_FLAGS=-s -w

all: build

.PHONY: build
build: package-custom-resources compile-local package-custom-resources-clean

.PHONY: build-e2e
build-e2e: package-custom-resources compile-linux package-custom-resources-clean

.PHONY: build-regression
build-regression: package-custom-resources compile-linux package-custom-resources-clean

.PHONY: release
release: package-custom-resources compile-darwin compile-linux compile-windows package-custom-resources-clean

.PHONY: release-docker
release-docker:
	docker build -t aws/copilot . &&\
	docker create -ti --name amazon-ecs-copilot-builder aws/copilot &&\
	docker cp amazon-ecs-copilot-builder:/copilot/bin/local/ . &&\
	docker rm -f amazon-ecs-copilot-builder
	@echo "Built binaries under ./local/"

.PHONY: compile-local
compile-local:
	CGO_ENABLED=0 go build -ldflags "${LINKER_FLAGS}" -o ${DESTINATION} ./cmd/copilot

.PHONY: compile-windows
compile-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "${LINKER_FLAGS} ${RELEASE_BUILD_LINKER_FLAGS}" -o ${DESTINATION}.exe ./cmd/copilot

.PHONY: compile-linux
compile-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${LINKER_FLAGS} ${RELEASE_BUILD_LINKER_FLAGS}" -o ${DESTINATION}-linux-amd64 ./cmd/copilot
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "${LINKER_FLAGS} ${RELEASE_BUILD_LINKER_FLAGS}" -o ${DESTINATION}-linux-arm64 ./cmd/copilot

.PHONY: compile-darwin
compile-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "${LINKER_FLAGS} ${RELEASE_BUILD_LINKER_FLAGS}" -o ${DESTINATION}-darwin-amd64 ./cmd/copilot
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "${LINKER_FLAGS} ${RELEASE_BUILD_LINKER_FLAGS}" -o ${DESTINATION}-darwin-arm64 ./cmd/copilot

.PHONY: test
test: run-unit-test custom-resource-tests

.PHONY: custom-resource-tests
custom-resource-tests: tools
	@echo "Running custom resource unit tests" &&\
	cd ${SOURCE_CUSTOM_RESOURCES} &&\
	npm test -- --coverage &&\
	cd ${ROOT_SRC_DIR}

# Minifies the resources in cf-custom-resources/lib and copies
# those minified assets into templates/custom-resources so that
# they can be packed.
.PHONY: package-custom-resources
package-custom-resources: tools
	@echo "Packaging custom resources to templates/custom-resources" &&\
	cd ${SOURCE_CUSTOM_RESOURCES} &&\
	npm run package &&\
	cd ${ROOT_SRC_DIR}

# We only need the minified custom resources during building. After
# they're packed, we can remove them.
.PHONY: package-custom-resources-clean
package-custom-resources-clean:
	@echo "Removing minified templates/custom-resources" &&\
	rm ${BUILT_CUSTOM_RESOURCES}/*.js

.PHONY: run-unit-test
run-unit-test:
	go test -coverprofile=${COVERAGE} ${PACKAGES}

.PHONY: test-race
test-race:
	go test -race -count=1 ${PACKAGES}

.PHONY: generate-coverage
generate-coverage: test
	go tool cover -html=${COVERAGE}

.PHONY: integ-test
integ-test: package-custom-resources run-integ-test package-custom-resources-clean 

.PHONY: run-integ-test
run-integ-test:
	# These tests have a long timeout as they create and teardown CloudFormation stacks.
	# Also adding count=1 so the test results aren't cached.
	# This command also targets files with the build integration tag
	# and runs tests which end in Integration.
	go test -race -count=1 -timeout 120m -tags=integration ${PACKAGES}

.PHONY: local-test
local-test: package-custom-resources custom-resource-tests run-local-test package-custom-resources-clean

.PHONY: run-local-test
run-local-test:
	go test -race -count=1 -timeout=60m -tags=localintegration -coverprofile=${COVERAGE} ${PACKAGES}

.PHONY: e2e
e2e: build-e2e
	@echo "Building E2E Docker Image" &&\
	docker build -t copilot/e2e . -f e2e/Dockerfile
	@echo "Running E2E Tests" &&\
	docker run --privileged -v ${HOME}/.aws:/home/.aws -e "HOME=/home" copilot/e2e:latest

.PHONY: e2e-dryrun
e2e-dryrun: build # Sample command "make e2e-dryrun test=multi-env-app" to run the test suit under "e2e/multi-env-app"
	@echo "Install ginkgo"
	go install github.com/onsi/ginkgo/v2/ginkgo@latest
	@echo "Setup credentials"
	./scripts/dryrun-creds.sh e2e
	@echo "Run the $(test) test"
	cd e2e/$(test) && DRYRUN=true ginkgo -v -r
	cd -

# Examples:
# REGRESSION_TEST_FROM_PATH=/usr/local/bin/copilot make regression-dryrun test=multi-svc-app
# REGRESSION_TEST_FROM_PATH=/usr/local/bin/copilot-v1.18.0 REGRESSION_TO_FROM_PATH=/usr/local/bin/copilot-v1.19.0 make regression-dryrun test=multi-svc-app
.PHONY: regression-dryrun
regression-dryrun: build
	@echo "Install ginkgo"
	go install github.com/onsi/ginkgo/v2/ginkgo@latest
	@echo "Setup credentials"
	./scripts/dryrun-creds.sh regression
	@echo "Run the $(test) test"
	cd regression/$(test) && DRYRUN=true ginkgo -v -r
	cd -

.PHONY: tools
tools:
	@echo "Installing custom resource dependencies" &&\
	cd ${SOURCE_CUSTOM_RESOURCES} && npm ci

.PHONY: site-local
site-local:
	docker build . -f Dockerfile.site -t site:latest
	docker run -p 8000:8000 -v `pwd`/site:/website/site -it site:latest

.PHONY: gen-mocks
gen-mocks: tools
	GOBIN=${GOBIN} go install github.com/golang/mock/mockgen@latest
	# TODO: make this more extensible?
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/sessions/mocks/mock_sessions.go -source=./internal/pkg/aws/sessions/sessions.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/cli/mocks/mock_rg.go -source=./internal/pkg/cli/env_delete.go resourceGetter
	${GOBIN}/mockgen -source=./internal/pkg/term/progress/spinner.go -package=mocks -destination=./internal/pkg/term/progress/mocks/mock_spinner.go
	${GOBIN}/mockgen -source=./internal/pkg/term/progress/render.go -package=mocks -destination=./internal/pkg/term/progress/mocks/mock_render.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/progress.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_progress.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/prompter.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_prompter.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/interfaces.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_interfaces.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/term/selector/mocks/mock_selector.go -source=./internal/pkg/term/selector/selector.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/term/selector/mocks/mock_ec2.go -source=./internal/pkg/term/selector/ec2.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/term/selector/mocks/mock_creds.go -source=./internal/pkg/term/selector/creds.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/completion.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_completion.go
	${GOBIN}/mockgen -source=./internal/pkg/cli/identity.go -package=mocks -destination=./internal/pkg/cli/mocks/mock_identity.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_lb_web_service.go -source=./internal/pkg/describe/lb_web_service.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_rd_web_service.go -source=./internal/pkg/describe/rd_web_service.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_backend_service.go -source=./internal/pkg/describe/backend_service.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_service.go -source=./internal/pkg/describe/service.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_describe.go -source=./internal/pkg/describe/describe.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_env.go -source=./internal/pkg/describe/env.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/stack/mocks/mock_stack.go -source=./internal/pkg/describe/stack/stack.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_status.go -source=./internal/pkg/describe/status.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_pipeline_show.go -source=./internal/pkg/describe/pipeline_show.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_pipeline_status.go -source=./internal/pkg/describe/pipeline_status.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/describe/mocks/mock_status_describe.go -source=./internal/pkg/describe/status_describe.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/ecr/mocks/mock_ecr.go -source=./internal/pkg/aws/ecr/ecr.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/ecs/mocks/mock_ecs.go -source=./internal/pkg/aws/ecs/ecs.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/ec2/mocks/mock_ec2.go -source=./internal/pkg/aws/ec2/ec2.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/identity/mocks/mock_identity.go -source=./internal/pkg/aws/identity/identity.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/route53/mocks/mock_route53.go -source=./internal/pkg/aws/route53/route53.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/iam/mocks/mock_iam.go -source=./internal/pkg/aws/iam/iam.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/secretsmanager/mocks/mock_secretsmanager.go -source=./internal/pkg/aws/secretsmanager/secretsmanager.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/codepipeline/mocks/mock_codepipeline.go -source=./internal/pkg/aws/codepipeline/codepipeline.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/codestar/mocks/mock_codestar.go -source=./internal/pkg/aws/codestar/codestar.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/cloudwatch/mocks/mock_cloudwatch.go -source=./internal/pkg/aws/cloudwatch/cloudwatch.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/aas/mocks/mock_aas.go -source=./internal/pkg/aws/aas/aas.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/resourcegroups/mocks/mock_resourcegroups.go -source=./internal/pkg/aws/resourcegroups/resourcegroups.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/cloudwatchlogs/mocks/mock_cloudwatchlogs.go -source=./internal/pkg/aws/cloudwatchlogs/cloudwatchlogs.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/s3/mocks/mock_s3.go -source=./internal/pkg/aws/s3/s3.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/acm/mocks/mock_acm.go -source=./internal/pkg/aws/acm/acm.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/cloudformation/mocks/mock_cloudformation.go -source=./internal/pkg/aws/cloudformation/interfaces.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/cloudformation/stackset/mocks/mock_stackset.go -source=./internal/pkg/aws/cloudformation/stackset/stackset.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/ssm/mocks/mock_ssm.go -source=./internal/pkg/aws/ssm/ssm.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/stepfunctions/mocks/mock_stepfunctions.go -source=./internal/pkg/aws/stepfunctions/stepfunctions.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/apprunner/mocks/mock_apprunner.go -source=./internal/pkg/aws/apprunner/apprunner.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/aws/elbv2/mocks/mock_elbv2.go -source=./internal/pkg/aws/elbv2/elbv2.go
	${GOBIN}/mockgen -package=exec -source=./internal/pkg/exec/exec.go -destination=./internal/pkg/exec/mock_exec.go
	${GOBIN}/mockgen -package=dockerengine -source=./internal/pkg/docker/dockerengine/dockerengine.go -destination=./internal/pkg/docker/dockerengine/mock_dockerengine.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/deploy/mocks/mock_deploy.go -source=./internal/pkg/deploy/deploy.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/deploy/cloudformation/mocks/mock_cloudformation.go -source=./internal/pkg/deploy/cloudformation/cloudformation.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/deploy/cloudformation/stack/mocks/mock_workload.go -source=./internal/pkg/deploy/cloudformation/stack/workload.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/deploy/cloudformation/stack/mocks/mock_embed.go -source=./internal/pkg/deploy/cloudformation/stack/embed.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/template/mocks/mock_template.go -source=./internal/pkg/template/template.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/task/mocks/mock_task.go -source=./internal/pkg/task/task.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/repository/mocks/mock_repository.go -source=./internal/pkg/repository/repository.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/logging/mocks/mock_workload.go -source=./internal/pkg/logging/workload.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/logging/mocks/mock_task.go -source=./internal/pkg/logging/task.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/cli/list/mocks/mock_list.go -source=./internal/pkg/cli/list/list.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/cli/deploy/mocks/mock_backend.go -source=./internal/pkg/cli/deploy/backend.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/cli/deploy/mocks/mock_env.go -source=./internal/pkg/cli/deploy/env.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/cli/deploy/mocks/mock_job.go -source=./internal/pkg/cli/deploy/job.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/cli/deploy/mocks/mock_lbws.go -source=./internal/pkg/cli/deploy/lbws.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/cli/deploy/mocks/mock_rdws.go -source=./internal/pkg/cli/deploy/rdws.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/cli/deploy/mocks/mock_svc.go -source=./internal/pkg/cli/deploy/svc.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/cli/deploy/mocks/mock_worker.go -source=./internal/pkg/cli/deploy/worker.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/cli/deploy/mocks/mock_workload.go -source=./internal/pkg/cli/deploy/workload.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/cli/deploy/mocks/mock_static_site.go -source=./internal/pkg/cli/deploy/static_site.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/cli/deploy/patch/mocks/mock_env.go -source=./internal/pkg/cli/deploy/patch/env.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/initialize/mocks/mock_workload.go -source=./internal/pkg/initialize/workload.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/ecs/mocks/mock_ecs.go -source=./internal/pkg/ecs/ecs.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/s3/mocks/mock_s3.go -source=./internal/pkg/s3/s3.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/apprunner/mocks/mock_apprunner.go -source=./internal/pkg/apprunner/apprunner.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/ecs/mocks/mock_run_task_request.go -source=./internal/pkg/ecs/run_task_request.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/runner/jobrunner/mocks/mock.go -source=./internal/pkg/runner/jobrunner/jobrunner.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/manifest/mocks/mock.go -source=./internal/pkg/manifest/loader.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/addon/mocks/mock_package.go -source=./internal/pkg/addon/package.go
	${GOBIN}/mockgen -package=mocks -destination=./internal/pkg/addon/mocks/mock_addons.go -source=./internal/pkg/addon/addons.go
