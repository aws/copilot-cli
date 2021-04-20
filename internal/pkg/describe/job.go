//// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
//// SPDX-License-Identifier: Apache-2.0
//
//// package describe blahblah TODO
package describe

//
//import (
//	"encoding/json"
//	"fmt"
//	"strings"
//
//	"github.com/aws/copilot-cli/internal/pkg/deploy"
//
//	"github.com/aws/aws-sdk-go/aws"
//	"github.com/aws/aws-sdk-go/aws/arn"
//	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
//)
//
//const (
//	fmtStateMachineName = "%s-%s-%s" // refer to workload's state-machine partial template.
//	targetState         = "Run Fargate Task"
//)
//
//type stepFunctionsClient interface {
//	StateMachineDefinition(stateMachineARN string) (string, error)
//}
//
//type resourceGetter interface {
//	GetResourcesByTags(resourceType string, tags map[string]string) ([]*resourcegroups.Resource, error)
//}
//
//// JobDescriber retrieves information about a job.
//type JobDescriber struct {
//	App string
//	Env string
//	Job string
//
//	ECSClient           ecsClient
//	StepFunctionsClient stepFunctionsClient
//	resourceGetter      resourceGetter
//}
//
//func (d *JobDescriber) TaskDefinition() (*TaskDefinition, error) {
//	taskDefName := fmt.Sprintf("%s-%s-%s", d.App, d.Env, d.Job)
//	taskDefinition, err := d.ECSClient.TaskDefinition(taskDefName)
//	if err != nil {
//		return nil, fmt.Errorf("get task definition %s of service %s: %w", taskDefName, d.Job, err)
//	}
//
//	return &TaskDefinition{
//		Images:        taskDefinition.Images(),
//		ExecutionRole: aws.StringValue(taskDefinition.ExecutionRoleArn),
//		TaskRole:      aws.StringValue(taskDefinition.TaskRoleArn),
//		EnvVars:       taskDefinition.EnvironmentVariables(),
//		Secrets:       taskDefinition.Secrets(),
//		EntryPoints:   taskDefinition.EntryPoints(),
//		Commands:      taskDefinition.Commands(),
//	}, nil
//}
//
//type NetworkConfiguration struct {
//	AWSvpcConfiguration struct {
//		Subnets        []string `json:"Subnets"`
//		AssignPublicIp string   `json:"AssignPublicIp"`
//		SecurityGroups []string `json:"SecurityGroups"`
//	} `json:"AwsvpcConfiguration"`
//}
//
//type parameter struct {
//	NetworkConfiguration `json:"NetworkConfiguration"`
//}
//
//type state struct {
//	parameter `json:"Parameters"`
//}
//
//type definition struct {
//	States map[string]state `json:"states"`
//}
//
//func (d *JobDescriber) NetworkConfiguration() (*NetworkConfiguration, error) {
//	arn, err := d.getStateMachineARN()
//	if err != nil {
//		return nil, err
//	}
//
//	raw, err := d.StepFunctionsClient.StateMachineDefinition(arn)
//	if err != nil {
//		return nil, fmt.Errorf("get state machine definition for job %s: %w", d.Job, err)
//	}
//
//	var definition definition
//	err = json.Unmarshal([]byte(raw), &definition)
//	if err != nil {
//		return nil, fmt.Errorf("unmarshal state machine definition: %w", err)
//	}
//
//	state := definition.States[targetState]
//	return &state.NetworkConfiguration, nil
//}
//
//func (d *JobDescriber) getStateMachineARN() (string, error) {
//	resources, err := d.resourceGetter.GetResourcesByTags(resourcegroups.ResourceTypeStateMachine, map[string]string{
//		deploy.AppTagKey:     d.App,
//		deploy.EnvTagKey:     d.Env,
//		deploy.ServiceTagKey: d.Job,
//	})
//	if err != nil {
//		return "", fmt.Errorf("get state machine resource for job %s: %w", d.Job, err)
//	}
//
//	var stateMachineARN string
//	targetName := fmt.Sprintf(fmtStateMachineName, d.App, d.Env, d.Job)
//	for _, r := range resources {
//		parsedARN, err := arn.Parse(stateMachineARN)
//		if err != nil {
//			continue
//		}
//		parts := strings.Split(parsedARN.Resource, ":")
//		if len(parts) != 2 {
//			return "", fmt.Errorf("unable to parse ARN %s", r.ARN)
//		}
//		if parts[1] == targetName {
//			stateMachineARN = r.ARN
//			break
//		}
//	}
//
//	if stateMachineARN == "" {
//		return "", fmt.Errorf("state machine for job %s not found", d.Job)
//	}
//	return stateMachineARN, nil
//}
