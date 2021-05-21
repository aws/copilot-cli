// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ecs

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/require"
)

func TestService_TargetGroups(t *testing.T) {
	t.Run("should return correct ARNs", func(t *testing.T) {
		s := Service{
			LoadBalancers: []*ecs.LoadBalancer{
				{
					TargetGroupArn: aws.String("group-1"),
				},
				{
					TargetGroupArn: aws.String("group-2"),
				},
			},
		}
		got := s.TargetGroups()
		expected := []string{"group-1", "group-2"}
		require.Equal(t, expected, got)
	})
}
