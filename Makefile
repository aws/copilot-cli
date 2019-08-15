# Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

all: build

.PHONY: build
build:
	CGO_ENABLED=0 go build -o ./bin/local/archer ./cmd/archer