#!/bin/bash
# Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

# Normalize to working directory being build root (up one level from ./scripts)
ROOT=$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )
cd "${ROOT}"

go run ./internal/pkg/version/gen/generate-version.go

GIT_DIRTY=`git diff --quiet || echo '*'`
GIT_SHORT_HASH="$GIT_DIRTY"`git rev-parse --short=7 HEAD`

echo "Building archer to ${DESTINATION}"

# TODO: Inject version and git short hash into build
GOOS=$TARGET_GOOS GOARCH=$GOARCH CGO_ENABLED=0 go build \
	-ldflags "-X github.com/aws/amazon-ecs-cli-v2/internal/pkg/version.GitHash=$GIT_SHORT_HASH" \
       	-o ${DESTINATION} ./cmd/archer
