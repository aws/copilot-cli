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
			"copilot-application": "phonetool",
		}),
		WithRoleARN("arn"))

	// THEN
	require.Equal(t, "hello", s.Name)
	require.Equal(t, "world", s.TemplateBody)
	require.Equal(t, []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String("Port"),
			ParameterValue: aws.String("80"),
		},
	}, s.Parameters)
	require.Equal(t, []*cloudformation.Tag{
		{
			Key:   aws.String("copilot-application"),
			Value: aws.String("phonetool"),
		},
	}, s.Tags)
	require.Equal(t, aws.String("arn"), s.RoleARN)
}

func TestNewStackWithURL(t *testing.T) {
	// WHEN
	s := NewStackWithURL("hello", "worldlyURL",
		WithParameters(map[string]string{
			"Port": "80",
		}),
		WithTags(map[string]string{
			"copilot-application": "phonetool",
		}),
		WithRoleARN("arn"))

	// THEN
	require.Equal(t, "hello", s.Name)
	require.Equal(t, "worldlyURL", s.TemplateURL)
	require.Equal(t, []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String("Port"),
			ParameterValue: aws.String("80"),
		},
	}, s.Parameters)
	require.Equal(t, []*cloudformation.Tag{
		{
			Key:   aws.String("copilot-application"),
			Value: aws.String("phonetool"),
		},
	}, s.Tags)
	require.Equal(t, aws.String("arn"), s.RoleARN)
}
