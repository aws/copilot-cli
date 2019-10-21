#!/bin/bash
# Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

# Normalize to working directory being build root (up one level from ./scripts)
ROOT=$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )
cd "${ROOT}"

GIT_TAGGED_VERSION=`git describe --tags --always`

echo "Building archer to ${DESTINATION}"

# Injects last tagged version and/or git hash into the build to populate version info
GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=$CGO_ENABLED go build -ldflags \
	"-X github.com/aws/amazon-ecs-cli-v2/internal/pkg/version.Platform=$PLATFORM -X github.com/aws/amazon-ecs-cli-v2/internal/pkg/version.Version=$GIT_TAGGED_VERSION" \
       	-o ${DESTINATION} ./cmd/archer
