// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package resourcegroups provides a client to make API requests to AWS Resource Groups

package resourcegroups

import (
	"github.com/aws/aws-sdk-go/service/resourcegroups"
)

type ResourceGroupClient interface {
	SearchResources(input *resourcegroups.SearchResourcesInput) (*resourcegroups.SearchResourcesOutput, error)
}
