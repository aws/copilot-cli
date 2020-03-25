// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// Tag keys used while creating stacks.
const (
	ProjectTagKey = "ecs-project"
	EnvTagKey     = "ecs-environment"
	AppTagKey     = "ecs-application"
)

func mergeAndFlattenTags(additionalTags map[string]string, cliTags map[string]string) []*cloudformation.Tag {
	tags := make(map[string]string)
	for k, v := range additionalTags {
		tags[k] = v
	}
	// Ignore user overrides for reserved tags so that we can still detect resources created with the CLI.
	for k, v := range cliTags {
		tags[k] = v
	}

	var flatTags []*cloudformation.Tag
	for k, v := range tags {
		flatTags = append(flatTags, &cloudformation.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}
	return flatTags
}
