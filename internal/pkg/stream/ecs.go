// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stream

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatch"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
)

const (
	// ECS service deployment constants.
	ecsPrimaryDeploymentStatus = "PRIMARY"
	rollOutCompleted           = "COMPLETED"
	rollOutFailed              = "FAILED"
	rollOutEmpty               = ""
)

const (
	ecsScalingActivity = "Scaling activity initiated by"
)

var ecsEventFailureKeywords = []string{"fail", "unhealthy", "error", "throttle", "unable", "missing", "alarm detected", "rolling back"}

// ECSServiceDescriber is the interface to describe an ECS service.
type ECSServiceDescriber interface {
	Service(clusterName, serviceName string) (*ecs.Service, error)
	StoppedServiceTasks(cluster, service string) ([]*ecs.Task, error)
}

// CloudWatchDescriber is the interface to describe CW alarms.
type CloudWatchDescriber interface {
	AlarmStatuses(opts ...cloudwatch.DescribeAlarmOpts) ([]cloudwatch.AlarmStatus, error)
}

// ECSDeployment represent an ECS rolling update deployment.
type ECSDeployment struct {
	Status          string
	TaskDefRevision string
	DesiredCount    int
	RunningCount    int
	FailedCount     int
	PendingCount    int
	RolloutState    string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Id              string
}

func (d ECSDeployment) isPrimary() bool {
	return d.Status == ecsPrimaryDeploymentStatus
}

func (d ECSDeployment) done() bool {
	switch d.RolloutState {
	case rollOutFailed:
		return true
	case rollOutCompleted, rollOutEmpty:
		return d.DesiredCount == d.RunningCount
	default:
		return false
	}
}

// ECSService is a description of an ECS service.
type ECSService struct {
	Deployments         []ECSDeployment
	LatestFailureEvents []string
	Alarms              []cloudwatch.AlarmStatus
	StoppedTasks        []ecs.Task
}

// ECSDeploymentStreamer is a Streamer for ECSService descriptions until the deployment is completed.
type ECSDeploymentStreamer struct {
	client                 ECSServiceDescriber
	cw                     CloudWatchDescriber
	clock                  clock
	cluster                string
	rand                   func(n int) int
	service                string
	deploymentCreationTime time.Time

	subscribers   []chan ECSService
	isDone        bool
	pastEventIDs  map[string]bool
	eventsToFlush []ECSService
	mu            sync.Mutex

	ecsRetries int
	cwRetries  int
}

// NewECSDeploymentStreamer creates a new ECSDeploymentStreamer that streams service descriptions
// since the deployment creation time and until the primary deployment is completed.
func NewECSDeploymentStreamer(ecs ECSServiceDescriber, cw CloudWatchDescriber, cluster, service string, deploymentCreationTime time.Time) *ECSDeploymentStreamer {
	return &ECSDeploymentStreamer{
		client:                 ecs,
		cw:                     cw,
		clock:                  realClock{},
		rand:                   rand.Intn,
		cluster:                cluster,
		service:                service,
		deploymentCreationTime: deploymentCreationTime,
		pastEventIDs:           make(map[string]bool),
	}
}

// Subscribe returns a read-only channel that will receive service descriptions from the ECSDeploymentStreamer.
func (s *ECSDeploymentStreamer) Subscribe() <-chan ECSService {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := make(chan ECSService)
	s.subscribers = append(s.subscribers, c)
	if s.isDone {
		// If the streamer is already done streaming, any new subscription requests should just return a closed channel.
		close(c)
	}
	return c
}

// Fetch retrieves and stores ECSService descriptions since the deployment's creation time
// until the primary deployment's running count is equal to its desired count.
// If an error occurs from describe service, returns a wrapped err.
// Otherwise, returns the time the next Fetch should be attempted.
func (s *ECSDeploymentStreamer) Fetch() (next time.Time, done bool, err error) {
	out, err := s.client.Service(s.cluster, s.service)
	if err != nil {
		if request.IsErrorThrottle(err) {
			s.ecsRetries += 1
			return nextFetchDate(s.clock, s.rand, s.ecsRetries), false, nil
		}
		return next, false, fmt.Errorf("fetch service description: %w", err)
	}
	s.ecsRetries = 0

	var deployments []ECSDeployment
	var primaryDeploymentId string
	for _, deployment := range out.Deployments {
		status := aws.StringValue(deployment.Status)
		desiredCount, runningCount := aws.Int64Value(deployment.DesiredCount), aws.Int64Value(deployment.RunningCount)
		rollingDeploy := ECSDeployment{
			Status:          status,
			TaskDefRevision: parseRevisionFromTaskDefARN(aws.StringValue(deployment.TaskDefinition)),
			DesiredCount:    int(desiredCount),
			RunningCount:    int(runningCount),
			FailedCount:     int(aws.Int64Value(deployment.FailedTasks)),
			PendingCount:    int(aws.Int64Value(deployment.PendingCount)),
			RolloutState:    aws.StringValue(deployment.RolloutState),
			CreatedAt:       aws.TimeValue(deployment.CreatedAt),
			UpdatedAt:       aws.TimeValue(deployment.UpdatedAt),
			Id:              aws.StringValue(deployment.Id),
		}
		deployments = append(deployments, rollingDeploy)
		if isDeploymentDone(rollingDeploy, s.deploymentCreationTime) {
			done = true
		}
		if rollingDeploy.isPrimary() {
			primaryDeploymentId = rollingDeploy.Id
		}
	}
	stoppedSvcTasks, err := s.client.StoppedServiceTasks(s.cluster, s.service)
	if err != nil {
		if request.IsErrorThrottle(err) {
			s.ecsRetries += 1
			return nextFetchDate(s.clock, s.rand, s.ecsRetries), false, nil
		}
		return next, false, fmt.Errorf("fetch stopped tasks: %w", err)
	}
	s.ecsRetries = 0

	var stoppedTasks []ecs.Task
	for _, st := range stoppedSvcTasks {
		if stoppingAt := aws.TimeValue(st.StoppingAt); aws.StringValue(st.StartedBy) != primaryDeploymentId || stoppingAt.Before(s.deploymentCreationTime) ||
			(strings.Contains(aws.StringValue(st.StoppedReason), ecsScalingActivity)) {
			continue
		}
		stoppedTasks = append(stoppedTasks, ecs.Task{
			TaskArn:       st.TaskArn,
			DesiredStatus: st.DesiredStatus,
			LastStatus:    st.LastStatus,
			StoppedReason: st.StoppedReason,
			StoppingAt:    st.StoppingAt,
		})
	}
	sort.SliceStable(stoppedTasks, func(i, j int) bool {
		return aws.TimeValue(stoppedTasks[i].StoppingAt).After(aws.TimeValue(stoppedTasks[j].StoppingAt))
	})

	var failureMsgs []string
	for _, event := range out.Events {
		if createdAt := aws.TimeValue(event.CreatedAt); createdAt.Before(s.deploymentCreationTime) {
			break
		}
		id := aws.StringValue(event.Id)
		if _, ok := s.pastEventIDs[id]; ok {
			break
		}
		if msg := aws.StringValue(event.Message); isFailureServiceEvent(msg) {
			failureMsgs = append(failureMsgs, msg)
		}
		s.pastEventIDs[id] = true
	}

	var alarms []cloudwatch.AlarmStatus
	if out.DeploymentConfiguration != nil && out.DeploymentConfiguration.Alarms != nil && aws.BoolValue(out.DeploymentConfiguration.Alarms.Enable) {
		alarmNames := aws.StringValueSlice(out.DeploymentConfiguration.Alarms.AlarmNames)
		alarms, err = s.cw.AlarmStatuses(cloudwatch.WithNames(alarmNames))
		if err != nil {
			if request.IsErrorThrottle(err) {
				s.cwRetries += 1
				return nextFetchDate(s.clock, s.rand, s.cwRetries), false, nil
			}
			return next, false, fmt.Errorf("retrieve alarm statuses: %w", err)
		}
		s.cwRetries = 0
	}

	s.eventsToFlush = append(s.eventsToFlush, ECSService{
		Deployments:         deployments,
		LatestFailureEvents: failureMsgs,
		Alarms:              alarms,
		StoppedTasks:        stoppedTasks,
	})
	return nextFetchDate(s.clock, s.rand, 0), done, nil
}

// Notify flushes all new events to the streamer's subscribers.
func (s *ECSDeploymentStreamer) Notify() {
	// Copy current list of subscribers over, so that we can we add more subscribers while
	// notifying previous subscribers of older events.
	s.mu.Lock()
	var subs []chan ECSService
	subs = append(subs, s.subscribers...)
	s.mu.Unlock()

	for _, event := range s.eventsToFlush {
		for _, sub := range subs {
			sub <- event
		}
	}
	s.eventsToFlush = nil // reset after flushing all events.
}

// Close closes all subscribed channels notifying them that no more events will be sent.
func (s *ECSDeploymentStreamer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sub := range s.subscribers {
		close(sub)
	}
	s.isDone = true
}

// parseRevisionFromTaskDefARN returns the revision number as string given the ARN of a task definition.
// For example, given the input "arn:aws:ecs:us-west-2:1111:task-definition/webapp-test-frontend:3"
// the output is "3".
func parseRevisionFromTaskDefARN(arn string) string {
	familyName := strings.Split(arn, "/")[1]
	return strings.Split(familyName, ":")[1]
}

func isFailureServiceEvent(msg string) bool {
	for _, kw := range ecsEventFailureKeywords {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

func isDeploymentDone(d ECSDeployment, startTime time.Time) bool {
	if !d.isPrimary() {
		return false
	}
	if d.UpdatedAt.Before(startTime) {
		return false
	}
	return d.done()
}
