// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ecr contains utility functions for dealing with ECR repos
package ecr

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"
)

const (
	urlFmtString      = "%s.dkr.ecr.%s.amazonaws.com/%s"
	arnResourcePrefix = "repository/"
)

// URIFromARN converts an ECR Repo ARN to a Repository URI
func URIFromARN(repositoryARN string) (string, error) {
	repoARN, err := arn.Parse(repositoryARN)
	if err != nil {
		return "", fmt.Errorf("parsing repository ARN %s: %w", repositoryARN, err)
	}
	// Repo ARNs look like arn:aws:ecr:region:012345678910:repository/test
	// so we have to strip the repository out.
	repoName := strings.TrimPrefix(repoARN.Resource, arnResourcePrefix)
	return fmt.Sprintf(urlFmtString,
		repoARN.AccountID,
		repoARN.Region,
		repoName), nil
}
