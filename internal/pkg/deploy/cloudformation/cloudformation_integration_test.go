// +build integration

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation_test

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	awsCF "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/stretchr/testify/require"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func Test_Project_Infrastructure(t *testing.T) {
	sess, err := testSession()
	require.NoError(t, err)
	identity := identity.New(sess)
	callerInfo, err := identity.Get()
	require.NoError(t, err)
	require.NoError(t, err)
	deployer := cloudformation.New(sess)
	cfClient := awsCF.New(sess)
	require.NoError(t, err)

	t.Run("Deploys Project Admin Roles to CloudFormation and Creates StackSet", func(t *testing.T) {
		project := archer.Project{Name: randStringBytes(10), AccountID: callerInfo.Account}
		projectRoleStackName := fmt.Sprintf("%s-infrastructure-roles", project.Name)
		projectStackSetName := fmt.Sprintf("%s-infrastructure", project.Name)

		// Given our stack doesn't exist
		roleStackOutput, err := cfClient.DescribeStacks(&awsCF.DescribeStacksInput{
			StackName: aws.String(projectRoleStackName),
		})

		require.True(t, len(roleStackOutput.Stacks) == 0, "Stack %s should not exist.", projectRoleStackName)

		// Make sure we delete the stacks after the test is done
		defer func() {
			cfClient.DeleteStack(&awsCF.DeleteStackInput{
				StackName: aws.String(projectRoleStackName),
			})

			cfClient.DeleteStackSet(&awsCF.DeleteStackSetInput{
				StackSetName: aws.String(projectStackSetName),
			})

			cfClient.WaitUntilStackDeleteComplete(&awsCF.DescribeStacksInput{
				StackName: aws.String(projectRoleStackName),
			})
		}()

		err = deployer.CreateProjectResources(&project)
		require.NoError(t, err)

		// We should create the project StackSet
		_, err = cfClient.DescribeStackSet(&awsCF.DescribeStackSetInput{
			StackSetName: aws.String(projectStackSetName),
		})
		require.NoError(t, err)

		// We should create the project roles stack
		roleStackOutput, err = cfClient.DescribeStacks(&awsCF.DescribeStacksInput{
			StackName: aws.String(projectRoleStackName),
		})
		require.NoError(t, err)

		require.True(t, len(roleStackOutput.Stacks) == 1, "Stack %s should have been deployed.", projectRoleStackName)
		deployedStack := roleStackOutput.Stacks[0]
		expectedResultsForKey := map[string]func(*awsCF.Output){
			"ExecutionRoleARN": func(output *awsCF.Output) {
				require.True(t,
					strings.HasSuffix(*output.OutputValue, fmt.Sprintf("role/%s-executionrole", project.Name)),
					fmt.Sprintf("ExecutionRoleARN should be named {project}-executionrole but was %s", *output.OutputValue))
			},
			"AdministrationRoleARN": func(output *awsCF.Output) {
				require.True(t,
					strings.HasSuffix(*output.OutputValue, fmt.Sprintf("role/%s-adminrole", project.Name)),
					fmt.Sprintf("AdministrationRoleARN should be named {project}-adminrole but was %s", *output.OutputValue))
			},
		}
		require.True(t, len(deployedStack.Outputs) == len(expectedResultsForKey),
			"There should have been %d output values - instead there were %d. The value of the CF stack was %s",
			len(expectedResultsForKey),
			len(deployedStack.Outputs),
			deployedStack.GoString(),
		)
		for _, output := range deployedStack.Outputs {
			key := *output.OutputKey
			validationFunction := expectedResultsForKey[key]
			require.NotNil(t, validationFunction, "Unexpected output key %s in stack.", key)
			validationFunction(output)
		}
	})

	t.Run("Deploys Project Infrastructure (KMS Key, ECR Repo, S3 Bucket)", func(t *testing.T) {
		project := archer.Project{Name: randStringBytes(10), AccountID: callerInfo.Account}
		projectRoleStackName := fmt.Sprintf("%s-infrastructure-roles", project.Name)
		projectStackSetName := fmt.Sprintf("%s-infrastructure", project.Name)

		// Make sure we delete the stacks after the test is done
		defer func() {
			// Clean up any StackInstances we may have created.
			if stackInstances, err := cfClient.ListStackInstances(&awsCF.ListStackInstancesInput{
				StackSetName: aws.String(projectStackSetName),
			}); err == nil && stackInstances.Summaries != nil && stackInstances.Summaries[0] != nil {
				projectStackInstance := stackInstances.Summaries[0]
				cfClient.DeleteStackInstances(&awsCF.DeleteStackInstancesInput{
					Accounts:     []*string{projectStackInstance.Account},
					Regions:      []*string{projectStackInstance.Region},
					RetainStacks: aws.Bool(false),
					StackSetName: projectStackInstance.StackSetId,
				})

				cfClient.WaitUntilStackDeleteComplete(&awsCF.DescribeStacksInput{
					StackName: projectStackInstance.StackId,
				})
			}
			// Delete the StackSet once all the StackInstances are cleaned up
			cfClient.DeleteStackSet(&awsCF.DeleteStackSetInput{
				StackSetName: aws.String(projectStackSetName),
			})

			cfClient.DeleteStack(&awsCF.DeleteStackInput{
				StackName: aws.String(projectRoleStackName),
			})
		}()

		// Given our stack doesn't exist
		roleStackOutput, err := cfClient.DescribeStacks(&awsCF.DescribeStacksInput{
			StackName: aws.String(projectRoleStackName),
		})

		require.True(t, len(roleStackOutput.Stacks) == 0, "Stack %s should not exist.", projectRoleStackName)

		err = deployer.CreateProjectResources(&project)
		require.NoError(t, err)

		// Add an application only
		err = deployer.AddAppToProject(
			&project,
			&archer.Application{
				Name: "myapp",
			},
		)

		require.NoError(t, err)

		// Add an application with dash
		err = deployer.AddAppToProject(
			&project,
			&archer.Application{
				Name: "myapp-frontend",
			},
		)

		require.NoError(t, err)

		// No new substacks should be created
		stackInstances, err := cfClient.ListStackInstances(&awsCF.ListStackInstancesInput{
			StackSetName: aws.String(projectStackSetName),
		})
		require.NoError(t, err)
		require.Equal(t, 0, len(stackInstances.Summaries), "Adding apps to a project without any envs shouldn't create any stack instancess.")

		// Add an application only
		err = deployer.AddEnvToProject(
			&project,
			&archer.Environment{
				Name:      "test",
				Region:    *sess.Config.Region,
				AccountID: "000312697014",
			},
		)
		stackInstances, err = cfClient.ListStackInstances(&awsCF.ListStackInstancesInput{
			StackSetName: aws.String(projectStackSetName),
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(stackInstances.Summaries), "Adding an env should create a new stack instance.")
		projectStackInstance := stackInstances.Summaries[0]
		// We should create the project roles stack
		projectInfraStacks, err := cfClient.DescribeStacks(&awsCF.DescribeStacksInput{
			StackName: projectStackInstance.StackId,
		})
		require.NoError(t, err)

		deployedStack := projectInfraStacks.Stacks[0]
		expectedResultsForKey := map[string]func(*awsCF.Output){
			"KMSKeyARN": func(output *awsCF.Output) {
				require.NotNil(t,
					*output.OutputValue,
					"KMSKeyARN should not be nil")
			},
			"PipelineBucket": func(output *awsCF.Output) {
				require.NotNil(t,
					*output.OutputValue,
					"PipelineBucket should not be nil")
			},
			"ECRRepomyapp": func(output *awsCF.Output) {
				require.True(t,
					strings.HasSuffix(*output.OutputValue, fmt.Sprintf("repository/%s/myapp", project.Name)),
					fmt.Sprintf("ECRRepomyapp should be suffixed with repository/{project}/myapp but was %s", *output.OutputValue))
			},
			// We replace dashes with the word DASH for logical IDss
			"ECRRepomyappDASHfrontend": func(output *awsCF.Output) {
				require.True(t,
					strings.HasSuffix(*output.OutputValue, fmt.Sprintf("repository/%s/myapp-frontend", project.Name)),
					fmt.Sprintf("ECRRepomyappDASHfrontend should be suffixed with repository/{project}/myapp but was %s", *output.OutputValue))
			},
		}
		require.True(t, len(deployedStack.Outputs) == len(expectedResultsForKey),
			"There should have been %d output values - instead there were %d. The value of the CF stack was %s",
			len(expectedResultsForKey),
			len(deployedStack.Outputs),
			deployedStack.GoString(),
		)
		for _, output := range deployedStack.Outputs {
			key := *output.OutputKey
			validationFunction := expectedResultsForKey[key]
			require.NotNil(t, validationFunction, "Unexpected output key %s in stack.", key)
			validationFunction(output)
		}
	})
}

// This test can take quite long as it spins up a CloudFormation stack
// and then waits for it to be deleted. If you find your CF stack
// is failing to be spun up because you've reached some limits, try
// switching your default region by running aws configure.
func Test_Environment_Deployment_Integration(t *testing.T) {
	sess, err := testSession()
	require.NoError(t, err)
	deployer := cloudformation.New(sess)
	cfClient := awsCF.New(sess)
	identity := identity.New(sess)

	id, err := identity.Get()
	require.NoError(t, err)

	environmentToDeploy := deploy.CreateEnvironmentInput{Name: randStringBytes(10), Project: randStringBytes(10), PublicLoadBalancer: true, ToolsAccountPrincipalARN: id.ARN}
	envStackName := fmt.Sprintf("%s-%s", environmentToDeploy.Project, environmentToDeploy.Name)

	t.Run("Deploys an environment to CloudFormation", func(t *testing.T) {
		// Given our stack doesn't exist
		output, err := cfClient.DescribeStacks(&awsCF.DescribeStacksInput{
			StackName: aws.String(envStackName),
		})
		require.True(t, len(output.Stacks) == 0, "Stack %s should not exist.", envStackName)

		// Make sure we delete the stack after the test is done
		defer func() {
			cfClient.DeleteStack(&awsCF.DeleteStackInput{
				StackName: aws.String(envStackName),
			})
		}()

		// Deploy the environment and wait for it to be complete
		require.NoError(t, deployer.DeployEnvironment(&environmentToDeploy))
		// Make sure the environment was deployed succesfully

		deployedEnv, err := deployer.WaitForEnvironmentCreation(&environmentToDeploy)
		require.NoError(t, err)

		// And that we saved the state from the stack into our environment.
		require.True(t,
			strings.HasSuffix(deployedEnv.RegistryURL,
				fmt.Sprintf("%s/%s", environmentToDeploy.Project, environmentToDeploy.Name)),
			"Repository URL should end with project/env - saved URL was "+deployedEnv.RegistryURL)

		// Ensure that the new stack exists
		output, err = cfClient.DescribeStacks(&awsCF.DescribeStacksInput{
			StackName: aws.String(envStackName),
		})
		require.NoError(t, err)
		require.True(t, len(output.Stacks) == 1, "Stack %s should have been deployed.", envStackName)

		deployedStack := output.Stacks[0]
		expectedResultsForKey := map[string]func(*awsCF.Output){
			"EnvironmentManagerRoleARN": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-EnvironmentManagerRoleARN", envStackName),
					*output.ExportName,
					"Should export EnvironmentManagerRole ARN")

				require.True(t,
					strings.HasSuffix(*output.OutputValue, fmt.Sprintf("role/%s-EnvManagerRole", envStackName)),
					"EnvironmentManagerRole ARN value should not be nil.")
			},
			"ECRRepositoryArn": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-ECRArn", envStackName),
					*output.ExportName,
					"Should export ECR Arn as stackname-Arn")

				require.True(t,
					strings.HasSuffix(*output.OutputValue,
						fmt.Sprintf("repository/%s/%s", environmentToDeploy.Project, environmentToDeploy.Name)),
					"ECR Repo Should be named repository/project_name/env_name")
			},
			"ECRRepositoryName": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-ECRName", envStackName),
					*output.ExportName,
					"Should export ECR name as stackname-ECRName")
				require.Equal(t,
					*output.OutputValue,
					fmt.Sprintf("%s/%s", environmentToDeploy.Project, environmentToDeploy.Name),
					"ECR Repo Name Should be named project_name/env_name",
				)
			},
			"ClusterId": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-ClusterId", envStackName),
					*output.ExportName,
					"Should export Cluster as stackname-ClusterId")

				require.NotNil(t,
					output.OutputValue,
					"Cluster value should not be nil")
			},
			"PrivateSubnets": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-PrivateSubnets", envStackName),
					*output.ExportName,
					"Should PrivateSubnets as stackname-PrivateSubnets")

				require.NotNil(t,
					output.OutputValue,
					"Private Subnet values should not be nil")
			},
			"VpcId": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-VpcId", envStackName),
					*output.ExportName,
					"Should export VpcId as stackname-VpcId")

				require.NotNil(t,
					output.OutputValue,
					"VpcId value should not be nil")
			},
			"PublicLoadBalancerSecurityGroupId": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-PublicLoadBalancerSecurityGroupId", envStackName),
					*output.ExportName,
					"Should export PublicLoadBalancerSecurityGroupId as stackname-PublicLoadBalancerSecurityGroupId")

				require.NotNil(t,
					output.OutputValue,
					"PublicLoadBalancerSecurityGroupId value should not be nil")
			},
			"PublicSubnets": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-PublicSubnets", envStackName),
					*output.ExportName,
					"Should export PublicSubnets as stackname-PublicSubnets")

				require.NotNil(t,
					output.OutputValue,
					"PublicSubnets value should not be nil")
			},
			"PublicLoadBalancerArn": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-PublicLoadBalancerArn", envStackName),
					*output.ExportName,
					"Should export PublicLoadBalancerArn as stackname-PublicLoadBalancerArn")

				require.NotNil(t,
					output.OutputValue,
					"PublicLoadBalancerArn value should not be nil")
			},
			"PublicLoadBalancerDNSName": func(output *awsCF.Output) {
				require.NotNil(t,
					output.OutputValue,
					"PublicLoadBalancerDNSName value should not be nil")
			},
		}
		require.True(t, len(deployedStack.Outputs) == len(expectedResultsForKey),
			"There should have been %d output values - instead there were %d. The value of the CF stack was %s",
			len(expectedResultsForKey),
			len(deployedStack.Outputs),
			deployedStack.GoString(),
		)
		for _, output := range deployedStack.Outputs {
			key := *output.OutputKey
			validationFunction := expectedResultsForKey[key]
			require.NotNil(t, validationFunction, "Unexpected output key %s in stack.", key)
			validationFunction(output)
		}
	})
}

const letterBytes = "abcdefghijklmnopqrstuvwxyz"

func testSession() (*session.Session, error) {
	return session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
}

func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
