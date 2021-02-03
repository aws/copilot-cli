// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
//// SPDX-License-Identifier: Apache-2.0

package codestar

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/codestarconnections"
)

type client interface {
	WaitUntilStatusAvailableWithContext(aws.Context, *codestarconnections.GetConnectionOutput, ...request.WaiterOption) error
}
