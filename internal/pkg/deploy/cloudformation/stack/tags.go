// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/tags"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// Tag keys used while creating stacks.
const (
	AppTagKey     = "ecs-project"
	EnvTagKey     = "ecs-environment"
	ServiceTagKey = "ecs-application"
)

func mergeAndFlattenTags(additionalTags map[string]string, cliTags map[string]string) []*cloudformation.Tag {
	var flatTags []*cloudformation.Tag
	for k, v := range tags.Merge(additionalTags, cliTags) {
		flatTags = append(flatTags, &cloudformation.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}
	return flatTags
}
