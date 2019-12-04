// +build integration

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	awsCF "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
)

func TestPipelineCreation(t *testing.T) {
	projectSess, err := testSession(nil)
	require.NoError(t, err)
	projectId := identity.New(projectSess)
	projectCallerInfo, err := projectId.Get()
	require.NoError(t, err)
	projectDeployer := cloudformation.New(projectSess)

	t.Run("creates a cross-region pipeline in a region with no environment", func(t *testing.T) {
		projCfClient := awsCF.New(projectSess)

		const pipelineName = "pipepiper"
		project := archer.Project{
			Name:      randStringBytes(10),
			AccountID: projectCallerInfo.Account,
		}
		pipelineStackName := project.Name + "-" + pipelineName
		projectRoleStackName := fmt.Sprintf("%s-infrastructure-roles", project.Name)
		projectStackSetName := fmt.Sprintf("%s-infrastructure", project.Name)

		// find another region (different from the project region,
		// i.e. *sess.Config.Region) for us to deploy an environment in.
		envRegion, err := findUnusedRegion("us-west", *projectSess.Config.Region)
		require.NoError(t, err)
		envSess, err := testSession(aws.String(envRegion.ID()))
		require.NoError(t, err)
		envCfClient := awsCF.New(envSess)
		envId := identity.New(envSess)
		envCallerInfo, err := envId.Get()
		require.NoError(t, err)
		envDeployer := cloudformation.New(envSess)

		environmentToDeploy := deploy.CreateEnvironmentInput{
			Name:                     randStringBytes(10),
			Project:                  project.Name,
			PublicLoadBalancer:       true,
			ToolsAccountPrincipalARN: envCallerInfo.RootUserARN,
		}
		envStackName := fmt.Sprintf("%s-%s",
			environmentToDeploy.Project,
			environmentToDeploy.Name)

		// Make sure we delete the stacks after the test is done
		defer func() {
			// delete the pipeline first because it relies on stackset
			_, err := projCfClient.DeleteStack(&awsCF.DeleteStackInput{
				StackName: aws.String(pipelineStackName),
			})
			require.NoError(t, err)
			err = projCfClient.WaitUntilStackDeleteComplete(&awsCF.DescribeStacksInput{
				StackName: aws.String(pipelineStackName),
			})
			require.NoError(t, err)

			// Clean up any StackInstances we may have created.
			if stackInstances, err := projCfClient.ListStackInstances(&awsCF.ListStackInstancesInput{
				StackSetName: aws.String(projectStackSetName),
			}); err == nil && stackInstances.Summaries != nil && stackInstances.Summaries[0] != nil {
				projectStackInstance := stackInstances.Summaries[0]
				_, err := projCfClient.DeleteStackInstances(&awsCF.DeleteStackInstancesInput{
					Accounts:     []*string{projectStackInstance.Account},
					Regions:      []*string{projectStackInstance.Region},
					RetainStacks: aws.Bool(false),
					StackSetName: projectStackInstance.StackSetId,
				})
				require.NoError(t, err)

				err = projCfClient.WaitUntilStackDeleteComplete(&awsCF.DescribeStacksInput{
					StackName: projectStackInstance.StackId,
				})
				require.NoError(t, err)
			}
			// Delete the StackSet once all the StackInstances are cleaned up
			_, err = projCfClient.DeleteStackSet(&awsCF.DeleteStackSetInput{
				StackSetName: aws.String(projectStackSetName),
			})
			require.NoError(t, err)

			_, err = projCfClient.DeleteStack(&awsCF.DeleteStackInput{
				StackName: aws.String(projectRoleStackName),
			})
			require.NoError(t, err)
			err = projCfClient.WaitUntilStackDeleteComplete(&awsCF.DescribeStacksInput{
				StackName: aws.String(projectRoleStackName),
			})
			require.NoError(t, err)

			// delete the environment stack once we are done
			_, err = envCfClient.DeleteStack(&awsCF.DeleteStackInput{
				StackName: aws.String(envStackName),
			})
			require.NoError(t, err)
			err = envCfClient.WaitUntilStackDeleteComplete(&awsCF.DescribeStacksInput{
				StackName: aws.String(envStackName),
			})
			require.NoError(t, err)
		}()

		// Given both the project stack and env we are deploying to do not
		// exist
		assertStackDoesNotExist(t, projCfClient, projectRoleStackName)
		assertStackDoesNotExist(t, envCfClient, envStackName)

		// create a stackset
		err = projectDeployer.DeployProject(&deploy.CreateProjectInput{
			Project:   project.Name,
			AccountID: project.AccountID,
		})
		require.NoError(t, err)

		// Deploy the environment in the same tools account but in different
		// region and wait for it to be complete
		require.NoError(t, envDeployer.DeployEnvironment(&environmentToDeploy))
		// Make sure the environment was deployed succesfully
		_, responses := envDeployer.StreamEnvironmentCreation(&environmentToDeploy)
		resp := <-responses
		require.NoError(t, resp.Err)

		// Ensure that the newly created env stack exists
		assertStackExists(t, envCfClient, envStackName)

		// Provision resources needed to support a pipeline in a region with
		// no existing archer environment.
		err = projectDeployer.AddPipelineResourcesToProject(
			&project,
			*projectSess.Config.Region)
		require.NoError(t, err)

		stackInstances, err := projCfClient.ListStackInstances(&awsCF.ListStackInstancesInput{
			StackSetName: aws.String(projectStackSetName),
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(stackInstances.Summaries),
			"project stack instance should exist")

		resources, err := projectDeployer.GetRegionalProjectResources(&project)
		require.NoError(t, err)
		artifactBuckets := regionalResourcesToArtifactBuckets(t, resources)

		pipelineInput := &deploy.CreatePipelineInput{
			ProjectName: project.Name,
			Name:        pipelineName,
			Source: &deploy.Source{
				ProviderName: manifest.GithubProviderName,
				Properties: map[string]interface{}{
					"repository":                   "chicken/wings",
					"branch":                       "master",
					manifest.GithubSecretIdKeyName: "testGitHubSecret",
				},
			},
			Stages: []deploy.PipelineStage{
				{
					AssociatedEnvironment: &deploy.AssociatedEnvironment{
						Name:      environmentToDeploy.Name,
						Region:    *projectSess.Config.Region,
						AccountID: project.AccountID,
						Prod:      true,
					},
					LocalApplications: []deploy.AppInStage{
						{
							Name:                   "frontend",
							IntegTestBuildspecPath: filepath.Join("frontend", manifest.IntegTestBuildspecFileName),
						},
						{
							Name: "backend",
						},
					},
				},
			},
			ArtifactBuckets: artifactBuckets,
		}
		require.NoError(t, projectDeployer.DeployPipeline(pipelineInput))

		// Ensure that the new stack exists
		assertStackExists(t, projCfClient, pipelineStackName)
	})
}

func assertStackDoesNotExist(t *testing.T, cfClient *awsCF.CloudFormation, stackName string) {
	_, err := cfClient.DescribeStacks(&awsCF.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	require.Error(t, err, "DescribeStacks should return an error because the stack does not exist")
	awsErr, ok := err.(awserr.Error)
	require.True(t, ok, "the returned error should be an awserr")
	require.Equal(t, awsErr.Code(), "ValidationError")
	require.Contains(t, awsErr.Message(), "does not exist", "the returned error should indicate that the stack does not exist")
}

func assertStackExists(t *testing.T, cfClient *awsCF.CloudFormation, stackName string) *awsCF.DescribeStacksOutput {
	resp, err := cfClient.DescribeStacks(&awsCF.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	require.NoError(t, err)
	require.True(t, len(resp.Stacks) == 1, "Stack %s should have been deployed.", stackName)
	return resp
}

func regionalResourcesToArtifactBuckets(t *testing.T, resources []*archer.ProjectRegionalResources) []deploy.ArtifactBucket {
	buckets := make([]deploy.ArtifactBucket, 0, len(resources))
	for _, res := range resources {
		require.True(t, res.S3Bucket != "", "S3 Bucket shouldn't be blank")
		require.True(t, res.KMSKeyARN != "", "KMSKey ARN shouldn't be blank")
		buckets = append(buckets, deploy.ArtifactBucket{
			BucketName: res.S3Bucket,
			KeyArn:     res.KMSKeyARN,
		})
	}
	return buckets
}

func findUnusedRegion(regionPrefix string, usedRegions ...string) (*endpoints.Region, error) {
	usedRegionsMap := make(map[string]bool)
	for _, usedRegion := range usedRegions {
		usedRegionsMap[usedRegion] = true
	}
	availableRegions := endpoints.AwsPartition().Services()[endpoints.CloudformationServiceID].Regions()
	for _, r := range availableRegions {
		if _, exists := usedRegionsMap[r.ID()]; !exists && strings.HasPrefix(r.ID(), regionPrefix) {
			return &r, nil
		}
	}
	return nil, errors.New("all regions are used")
}
