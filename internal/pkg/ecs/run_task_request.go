package ecs

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/cli"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
)

type ecsServiceDescriber interface {
	Service(clusterName, serviceName string) (*ecs.Service, error)
	TaskDefinition(taskDefName string) (*ecs.TaskDefinition, error)
	NetworkConfiguration(cluster, serviceName string) (*ecs.NetworkConfiguration, error)
}

type serviceDescriber interface {
	TaskDefinition(app, env, svc string) (*ecs.TaskDefinition, error)
	NetworkConfiguration(app, env, svc string) (*ecs.NetworkConfiguration, error)
	ClusterARN(app, env string) (string, error)
}

func RunTaskRequestFromECSService(client ecsServiceDescriber, cluster, service string) (*cli.RunTaskRequest, error) {
	networkConfig, err := client.NetworkConfiguration(cluster, service)
	if err != nil {
		return nil, fmt.Errorf("retrieve network configuration for service %s in cluster %s: %w", service, cluster, err)
	}

	svc, err := client.Service(cluster, service)
	if err != nil {
		return nil, fmt.Errorf("retrieve service %s in cluster %s: %w", service, cluster, err)
	}

	taskDefNameOrARN := aws.StringValue(svc.TaskDefinition)
	taskDef, err := client.TaskDefinition(taskDefNameOrARN)
	if err != nil {
		return nil, fmt.Errorf("retrieve task definition %s: %w", taskDefNameOrARN, err)
	}

	if len(taskDef.ContainerDefinitions) > 1 {
		return nil, fmt.Errorf("found more than one container in task definition: %s", taskDefNameOrARN)
	}

	containerName := aws.StringValue(taskDef.ContainerDefinitions[0].Name)
	containerInfo, err := containerInformation(taskDef, containerName)
	if err != nil {
		return nil, err
	}

	return &cli.RunTaskRequest{
		NetworkConfiguration: *networkConfig,
		ExecutionRole:        aws.StringValue(taskDef.ExecutionRoleArn),
		TaskRole:             aws.StringValue(taskDef.TaskRoleArn),
		ContainerInfo:        *containerInfo,
		Cluster:              cluster,
	}, nil
}

func RunTaskRequestFromService(client serviceDescriber, app, env, svc string) (*cli.RunTaskRequest, error) {
	networkConfig, err := client.NetworkConfiguration(app, env, svc)
	if err != nil {
		return nil, fmt.Errorf("retrieve network configuration for service %s: %w", svc, err)
	}

	cluster, err := client.ClusterARN(app, env)
	if err != nil {
		return nil, fmt.Errorf("retrieve cluster ARN created for environment %s in application %s: %w", env, app, err)
	}

	taskDef, err := client.TaskDefinition(app, env, svc)
	if err != nil {
		return nil, fmt.Errorf("retrieve task definition for service %s: %w", svc, err)
	}

	containerName := svc // NOTE: refer to workload's CloudFormation template. The container name is set to be the workload's name.
	containerInfo, err := containerInformation(taskDef, containerName)
	if err != nil {
		return nil, err
	}

	return &cli.RunTaskRequest{
		NetworkConfiguration: *networkConfig,
		ExecutionRole:        aws.StringValue(taskDef.ExecutionRoleArn),
		TaskRole:             aws.StringValue(taskDef.TaskRoleArn),
		ContainerInfo:        *containerInfo,
		Cluster:              cluster,
	}, nil
}

func containerInformation(taskDef *ecs.TaskDefinition, containerName string) (*cli.ContainerInfo, error) {
	image, err := taskDef.Image(containerName)
	if err != nil {
		return nil, err
	}

	entrypoint, err := taskDef.EntryPoint(containerName)
	if err != nil {
		return nil, err
	}

	command, err := taskDef.Command(containerName)
	if err != nil {
		return nil, err
	}

	envVars := make(map[string]string)
	for _, envVar := range taskDef.EnvironmentVariables() {
		if envVar.Container == containerName {
			envVars[envVar.Name] = envVar.Value
		}
	}

	secrets := make(map[string]string)
	for _, secret := range taskDef.Secrets() {
		if secret.Container == containerName {
			secrets[secret.Name] = secret.ValueFrom
		}
	}

	return &cli.ContainerInfo{
		Image:      image,
		EntryPoint: entrypoint,
		Command:    command,
		EnvVars:    envVars,
		Secrets:    secrets,
	}, nil
}
