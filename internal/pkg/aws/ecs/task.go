// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ecs provides a client to make API requests to Amazon Elastic Container Service.
package ecs

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ecs"
)

const (
	shortTaskIDLength = 8
	imageDigestPrefix = "sha256:"

	lastStatusRunning = "RUNNING"
	// These field names are not defined as const in sdk.
	networkInterfaceIDKey          = "networkInterfaceId"
	privateIPv4AddressKey          = "privateIPv4Address"
	networkInterfaceAttachmentType = "ElasticNetworkInterface"

	// TaskContainerHealthStatusUnknown wraps the ECS health status UNKNOWN.
	TaskContainerHealthStatusUnknown = ecs.HealthStatusUnknown
	// TaskContainerHealthStatusHealthy wraps the ECS health status HEALTHY.
	TaskContainerHealthStatusHealthy = ecs.HealthStatusHealthy
	// TaskContainerHealthStatusUnhealthy wraps the ECS health status UNHEALTHY.
	TaskContainerHealthStatusUnhealthy = ecs.HealthStatusUnhealthy

	// TaskCapacityProviderFargate is the capacity provider name for FARGATE.
	TaskCapacityProviderFargate = "FARGATE"
	// TaskCapacityProviderFargateSpot is the capacity provider name for FARGATE_SPOT.
	TaskCapacityProviderFargateSpot = "FARGATE_SPOT"
	// TaskStatusRunning is the task status running.
	TaskStatusRunning = "RUNNING"
)

// Image contains very basic info of a container image.
type Image struct {
	ID     string
	Digest string
}

// Task wraps up ECS Task struct.
type Task ecs.Task

// String returns the human readable format of an ECS task.
// For example, a task with ARN arn:aws:ecs:us-west-2:123456789:task/4082490ee6c245e09d2145010aa1ba8d
// and task definition ARN arn:aws:ecs:us-west-2:123456789012:task-definition/sample-fargate:2
// becomes "4082490e (sample-fargate:2)"
func (t Task) String() string {
	taskID, _ := TaskID(aws.StringValue(t.TaskArn))
	taskID = ShortTaskID(taskID)
	taskDefName, _ := taskDefinitionName(aws.StringValue(t.TaskDefinitionArn))
	return fmt.Sprintf("%s (%s)", taskID, taskDefName)
}

// TaskStatus returns the status of the running task.
func (t *Task) TaskStatus() (*TaskStatus, error) {
	taskID, err := TaskID(aws.StringValue(t.TaskArn))
	if err != nil {
		return nil, err
	}
	var startedAt, stoppedAt time.Time
	var stoppedReason string

	if t.StoppedAt != nil {
		stoppedAt = *t.StoppedAt
	}
	if t.StartedAt != nil {
		startedAt = *t.StartedAt
	}
	if t.StoppedReason != nil {
		stoppedReason = aws.StringValue(t.StoppedReason)
	}
	var images []Image
	for _, container := range t.Containers {
		images = append(images, Image{
			ID:     aws.StringValue(container.Image),
			Digest: imageDigestValue(aws.StringValue(container.ImageDigest)),
		})
	}
	return &TaskStatus{
		Health:           aws.StringValue(t.HealthStatus),
		ID:               taskID,
		Images:           images,
		LastStatus:       aws.StringValue(t.LastStatus),
		StartedAt:        startedAt,
		StoppedAt:        stoppedAt,
		StoppedReason:    stoppedReason,
		CapacityProvider: aws.StringValue(t.CapacityProviderName),
		TaskDefinition:   aws.StringValue(t.TaskDefinitionArn),
	}, nil
}

// ENI returns the network interface ID of the running task.
// Every Fargate task is provided with an ENI by default (https://docs.aws.amazon.com/AmazonECS/latest/userguide/fargate-task-networking.html).
func (t *Task) ENI() (string, error) {
	attachmentENI, err := t.attachmentENI()
	if err != nil {
		return "", err
	}

	for _, detail := range attachmentENI.Details {
		if aws.StringValue(detail.Name) == networkInterfaceIDKey {
			return aws.StringValue(detail.Value), nil
		}
	}
	return "", &ErrTaskENIInfoNotFound{
		MissingField: missingFieldDetailENIID,
		TaskARN:      aws.StringValue(t.TaskArn),
	}
}

// PrivateIP returns the private IPv4 address of the task.
func (t *Task) PrivateIP() (string, error) {
	attachmentENI, err := t.attachmentENI()
	if err != nil {
		return "", err
	}
	for _, detail := range attachmentENI.Details {
		if aws.StringValue(detail.Name) == privateIPv4AddressKey {
			return aws.StringValue(detail.Value), nil
		}
	}
	return "", &ErrTaskENIInfoNotFound{
		MissingField: missingFieldPrivateIPv4Address,
		TaskARN:      aws.StringValue(t.TaskArn),
	}
}

func (t *Task) attachmentENI() (*ecs.Attachment, error) {
	// Every Fargate task is provided with an ENI by default (https://docs.aws.amazon.com/AmazonECS/latest/userguide/fargate-task-networking.html).
	// So an error is warranted if there is no ENI found.
	var attachmentENI *ecs.Attachment
	for _, attachment := range t.Attachments {
		if aws.StringValue(attachment.Type) == networkInterfaceAttachmentType {
			attachmentENI = attachment
			break
		}
	}
	if attachmentENI == nil {
		return nil, &ErrTaskENIInfoNotFound{
			MissingField: missingFieldAttachment,
			TaskARN:      aws.StringValue(t.TaskArn),
		}
	}
	return attachmentENI, nil
}

// TaskStatus contains the status info of a task.
type TaskStatus struct {
	Health           string    `json:"health"`
	ID               string    `json:"id"`
	Images           []Image   `json:"images"`
	LastStatus       string    `json:"lastStatus"`
	StartedAt        time.Time `json:"startedAt"`
	StoppedAt        time.Time `json:"stoppedAt"`
	StoppedReason    string    `json:"stoppedReason"`
	CapacityProvider string    `json:"capacityProvider"`
	TaskDefinition   string    `json:"taskDefinitionARN"`
}

// TaskDefinition wraps up ECS TaskDefinition struct.
type TaskDefinition ecs.TaskDefinition

// ContainerPlatform holds basic info of a container's platform.
type ContainerPlatform struct {
	OperatingSystem string
	Architecture    string
}

// Platform returns the platform of the task definition.
func (t *TaskDefinition) Platform() *ContainerPlatform {
	if t.RuntimePlatform == nil {
		return nil
	}
	return &ContainerPlatform{
		OperatingSystem: aws.StringValue(t.RuntimePlatform.OperatingSystemFamily),
		Architecture:    aws.StringValue(t.RuntimePlatform.CpuArchitecture),
	}
}

// ContainerEnvVar holds basic info of an environment variable.
type ContainerEnvVar struct {
	Name      string
	Container string
	Value     string
}

// EnvironmentVariables returns environment variables of the task definition.
func (t *TaskDefinition) EnvironmentVariables() []*ContainerEnvVar {
	var envs []*ContainerEnvVar
	for _, container := range t.ContainerDefinitions {
		for _, env := range container.Environment {
			envs = append(envs, &ContainerEnvVar{
				aws.StringValue(env.Name),
				aws.StringValue(container.Name),
				aws.StringValue(env.Value),
			})
		}
	}
	return envs
}

// ContainerSecret holds basic info of a secret.
type ContainerSecret struct {
	Name      string
	Container string
	ValueFrom string
}

// Secrets returns secrets of the task definition.
func (t *TaskDefinition) Secrets() []*ContainerSecret {
	var secrets []*ContainerSecret
	for _, container := range t.ContainerDefinitions {
		for _, secret := range container.Secrets {
			secrets = append(secrets, &ContainerSecret{
				aws.StringValue(secret.Name),
				aws.StringValue(container.Name),
				aws.StringValue(secret.ValueFrom),
			})
		}
	}
	return secrets
}

// Image returns the container's image of the task definition.
func (t *TaskDefinition) Image(containerName string) (string, error) {
	for _, container := range t.ContainerDefinitions {
		if aws.StringValue(container.Name) == containerName {
			return aws.StringValue(container.Image), nil
		}
	}
	return "", fmt.Errorf("container %s not found", containerName)
}

// Command returns the container's command overrides of the task definition.
func (t *TaskDefinition) Command(containerName string) ([]string, error) {
	for _, container := range t.ContainerDefinitions {
		if aws.StringValue(container.Name) == containerName {
			return aws.StringValueSlice(container.Command), nil
		}
	}
	return nil, fmt.Errorf("container %s not found", containerName)
}

// EntryPoint returns the container's entrypoint overrides of the task definition.
func (t *TaskDefinition) EntryPoint(containerName string) ([]string, error) {
	for _, container := range t.ContainerDefinitions {
		if aws.StringValue(container.Name) == containerName {
			return aws.StringValueSlice(container.EntryPoint), nil
		}
	}
	return nil, fmt.Errorf("container %s not found", containerName)
}

// TaskID parses the task ARN and returns the task ID.
// For example: arn:aws:ecs:us-west-2:123456789:task/my-project-test-Cluster-9F7Y0RLP60R7/4082490ee6c245e09d2145010aa1ba8d,
// arn:aws:ecs:us-west-2:123456789:task/4082490ee6c245e09d2145010aa1ba8d
// return 4082490ee6c245e09d2145010aa1ba8d.
func TaskID(taskARN string) (string, error) {
	parsedARN, err := arn.Parse(taskARN)
	if err != nil {
		return "", fmt.Errorf("parse ECS task ARN: %w", err)
	}
	resources := strings.Split(parsedARN.Resource, "/")
	taskID := resources[len(resources)-1]
	return taskID, nil
}

// TaskDefinitionVersion takes a task definition ARN and returns its version.
// For example, given "arn:aws:ecs:us-east-1:568623488001:task-definition/some-task-def:6", it returns 6.
func TaskDefinitionVersion(taskDefARN string) (int, error) {
	parsedARN, err := arn.Parse(taskDefARN)
	if err != nil {
		return 0, fmt.Errorf("parse ARN %s: %w", taskDefARN, err)
	}

	resource := parsedARN.Resource
	parts := strings.Split(resource, ":")
	version, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return 0, fmt.Errorf("convert version %s from string to int: %w", parts[len(parts)-1], err)
	}
	return version, nil
}

// ShortTaskID truncates a string to a maximum length of shortTaskIDLength.
// If the input is shorter, it remains unchanged.
func ShortTaskID(id string) string {
	if len(id) >= shortTaskIDLength {
		return id[:shortTaskIDLength]
	}
	return id
}

// FilterRunningTasks returns only tasks with the last status to be RUNNING.
func FilterRunningTasks(tasks []*Task) []*Task {
	var filtered []*Task
	for _, task := range tasks {
		if aws.StringValue(task.LastStatus) == lastStatusRunning {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

// imageDigestValue strips the hash function prefix, such as "sha256:", from the digest.
// For example: sha256:18f7eb6cff6e63e5f5273fb53f672975fe6044580f66c354f55d2de8dd28aec7
// becomes 18f7eb6cff6e63e5f5273fb53f672975fe6044580f66c354f55d2de8dd28aec7.
func imageDigestValue(digest string) string {
	return strings.TrimPrefix(digest, imageDigestPrefix)
}

// taskDefinitionName parses the task definition ARN and returns the task definition name.
// For example: arn:aws:ecs:us-west-2:123456789012:task-definition/sample-fargate:2
// returns sample-fargate:2
func taskDefinitionName(taskDefARN string) (string, error) {
	parsedARN, err := arn.Parse(taskDefARN)
	if err != nil {
		return "", fmt.Errorf("parse ECS task definition ARN: %w", err)
	}
	resources := strings.Split(parsedARN.Resource, "/")
	return resources[len(resources)-1], nil
}
