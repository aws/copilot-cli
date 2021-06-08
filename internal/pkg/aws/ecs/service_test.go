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

func TestService_ServiceStatus(t *testing.T) {
	t.Run("should include active dpeloyments in status", func(t *testing.T) {
		inServce := Service{
			Deployments: []*ecs.Deployment{
				{
					Status:       aws.String("ACTIVE"),
					Id:           aws.String("id-1"),
					DesiredCount: aws.Int64(3),
					RunningCount: aws.Int64(3),
				},
				{
					Status:       aws.String("ACTIVE"),
					Id:           aws.String("id-3"),
					DesiredCount: aws.Int64(4),
					RunningCount: aws.Int64(2),
				},
				{
					Status:       aws.String("PRIMARY"),
					Id:           aws.String("id-4"),
					DesiredCount: aws.Int64(10),
					RunningCount: aws.Int64(1),
				},
				{
					Status: aws.String("INACTIVE"),
					Id:     aws.String("id-5"),
				},
			},
		}
		wanted := ServiceStatus{
			Status:         "",
			DesiredCount:   0,
			RunningCount:   0,
			TaskDefinition: "",
			ActiveDeployments: []Deployment{
				{
					Id:           "id-1",
					DesiredCount: 3,
					RunningCount: 3,
				},
				{
					Id:           "id-3",
					DesiredCount: 4,
					RunningCount: 2,
				},
			},
		}
		got := inServce.ServiceStatus()
		require.Equal(t, got, wanted)
	})
}
