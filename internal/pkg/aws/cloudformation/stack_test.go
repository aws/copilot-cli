// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/stretchr/testify/require"
)

func TestNewStack(t *testing.T) {
	// WHEN
	s := NewStack("hello", "world",
		WithParameters(map[string]string{
			"Port": "80",
		}),
		WithTags(map[string]string{
			"ecs-project": "phonetool",
		}),
		WithRoleARN("arn"))

	// THEN
	require.Equal(t, "hello", s.name)
	require.Equal(t, "world", s.template)
	require.Equal(t, []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String("Port"),
			ParameterValue: aws.String("80"),
		},
	}, s.parameters)
	require.Equal(t, []*cloudformation.Tag{
		{
			Key:   aws.String("ecs-project"),
			Value: aws.String("phonetool"),
		},
	}, s.tags)
	require.Equal(t, aws.String("arn"), s.roleARN)
}
