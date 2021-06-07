package describe

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	awsECS "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/elbv2"
)

// Indicator methods that determine whether some information should be shown in humanized output.
func (s *ecsServiceStatus) shouldShowHealthSummary() bool {
	return s.shouldShowContainerHealth() || s.shouldShowHTTPHealth()
}

func (s *ecsServiceStatus) shouldShowContainerHealth() bool {
	for _, t := range s.Tasks {
		if t.Health != awsECS.TaskContainerHealthStatusUnknown && t.Health != "" {
			return true
		}
	}
	// If all tasks' main container health are UNKNOWN or empty, we don't need to show container health.
	return false
}

func (s *ecsServiceStatus) shouldShowCapacityProvider() bool {
	for _, task := range s.Tasks {
		if task.CapacityProvider != "" {
			return true
		}
	}
	// If all tasks' capacity provider is empty, we don't need to show capacity provider.
	return false
}

func (s *ecsServiceStatus) shouldShowHTTPHealth() bool {
	return len(s.TasksTargetHealth) != 0
}

// Data methods that return reorganized information inside ecsServiceStatus
func (s *ecsServiceStatus) containerHealthData() (healthy int, unhealthy int, unknown int) {
	for _, t := range s.Tasks {
		switch strings.ToUpper(t.Health) {
		case "HEALTHY":
			healthy += 1
		case "UNHEALTHY":
			unhealthy += 1
		case "UNKNOWN":
			unknown += 1
		}
	}
	return
}

func (s *ecsServiceStatus) taskDefinitionRevisionData() map[int]int {
	out := make(map[int]int)
	for _, t := range s.Tasks {
		version, err := awsECS.TaskDefinitionVersion(t.TaskDefinition)
		if err != nil {
			out[-1] += 1
		} else {
			out[version] += 1
		}
	}
	return out
}

func (s *ecsServiceStatus) healthyHTTPTasksCount() int {
	var count int
	tasksHealthStates := s.summarizeTasksTargetHealth()
	for _, states := range tasksHealthStates {
		healthy := true
		// A task is HTTP-healthy if it's deemed healthy by all of its HTTP health check.
		for _, state := range states {
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

func (s *ecsServiceStatus) capacityProviderData() (fargate int, spot int, unset int) {
	for _, t := range s.Tasks {
		switch strings.ToUpper(t.CapacityProvider) {
		case "FARGATE":
			fargate += 1
		case "FARGATE_SPOT":
			spot += 1
		default:
			unset += 1
		}
	}
	return
}

func (s *ecsServiceStatus) summarizeTasksTargetHealth() map[string][]string {
	out := make(map[string][]string)
	for _, th := range s.TasksTargetHealth {
		out[th.TaskID] = append(out[th.TaskID], aws.StringValue(th.TargetHealthDescription.TargetHealth.State))
	}
	return out
}
