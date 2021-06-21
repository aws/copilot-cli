// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/elbv2"
)

func isContainerHealthCheckEnabled(tasks []ecs.TaskStatus) bool {
	// If all of the tasks container health is empty or UNKNOWN, then the container health check
	// is *typically* NOT enabled.
	for _, t := range tasks {
		if t.Health != ecs.TaskContainerHealthStatusUnknown && t.Health != "" {
			return true
		}
	}
	return false
}

func isCapacityProvidersEnabled(tasks []ecs.TaskStatus) bool {
	for _, task := range tasks {
		if task.CapacityProvider != "" {
			return true
		}
	}
	return false
}

func anyTasksInAnyTargetGroup(tasks []ecs.TaskStatus, targetHealthDescriptions []taskTargetHealth) bool {
	taskToHealth := summarizeHTTPHealthForTasks(targetHealthDescriptions)
	for _, t := range tasks {
		if _, ok := taskToHealth[t.ID]; ok {
			return true
		}
	}
	return false
}

func containerHealthBreakDownByCount(tasks []ecs.TaskStatus) (healthy int, unhealthy int, unknown int) {
	for _, t := range tasks {
		switch strings.ToUpper(t.Health) {
		case ecs.TaskContainerHealthStatusHealthy:
			healthy += 1
		case ecs.TaskContainerHealthStatusUnhealthy:
			unhealthy += 1
		case ecs.TaskContainerHealthStatusUnknown:
			unknown += 1
		}
	}
	return
}

func countHealthyHTTPTasks(tasks []ecs.TaskStatus, targetHealthDescriptions []taskTargetHealth) int {
	var count int
	taskToHealth := summarizeHTTPHealthForTasks(targetHealthDescriptions)
	for _, t := range tasks {
		// A task is healthy if it has health states and all of its states are healthy
		if _, ok := taskToHealth[t.ID]; !ok {
			continue
		}
		healthy := true
		for _, state := range taskToHealth[t.ID] {
			if state != elbv2.TargetHealthStateHealthy {
				healthy = false
			}
		}
		if healthy {
			count += 1
		}
	}
	return count
}

func summarizeHTTPHealthForTasks(targetsHealth []taskTargetHealth) map[string][]string {
	out := make(map[string][]string)
	for _, th := range targetsHealth {
		if th.TaskID == "" {
			continue
		}
		out[th.TaskID] = append(out[th.TaskID], th.HealthStatus.HealthState)
	}
	return out
}

func runningCapacityProvidersBreakDownByCount(tasks []ecs.TaskStatus) (fargate, spot, empty int) {
	for _, t := range tasks {
		if t.LastStatus != ecs.TaskStatusRunning {
			continue
		}
		switch strings.ToUpper(t.CapacityProvider) {
		case ecs.TaskCapacityProviderFargate:
			fargate += 1
		case ecs.TaskCapacityProviderFargateSpot:
			spot += 1
		default:
			empty += 1
		}
	}
	return
}

func (s *ecsServiceStatus) tasksOfRevision(revision int) []ecs.TaskStatus {
	var ret []ecs.TaskStatus
	for _, t := range s.DesiredRunningTasks {
		taskRevision, err := ecs.TaskDefinitionVersion(t.TaskDefinition)
		if err != nil {
			continue
		}
		if taskRevision == revision {
			ret = append(ret, t)
		}
	}
	return ret
}
