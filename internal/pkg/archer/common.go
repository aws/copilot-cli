// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

import "github.com/aws/aws-sdk-go/service/cloudformation"

// StackConfiguration represents an entity that can be serialized
// into a Cloudformation template
type StackConfiguration interface {
	StackName() string
	Template() (string, error)
	Parameters() []*cloudformation.Parameter
	Tags() []*cloudformation.Tag
}
