// +build integration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	awsCF "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
)

func TestPipelineCreation(t *testing.T) {
	appSess, err := testSession(nil)
	require.NoError(t, err)
	appId := identity.New(appSess)
	appCallerInfo, err := appId.Get()
	require.NoError(t, err)
	appDeployer := cloudformation.New(appSess)
	sm := secretsmanager.New(appSess)
	secretId := "testGitHubSecret" + randStringBytes(10)

	t.Run("creates a cross-region pipeline in a region with no environment", func(t *testing.T) {
		createMockSecret(t, sm, secretId)
		appCfClient := awsCF.New(appSess)

		app := config.Application{
			Name:      randStringBytes(10),
			AccountID: appCallerInfo.Account,
		}
		pipelineStackName := app.Name + "-pipepiper"
		appRoleStackName := fmt.Sprintf("%s-infrastructure-roles", app.Name)
		appStackSetName := fmt.Sprintf("%s-infrastructure", app.Name)

		// find another region (different from the application region,
		// i.e. *sess.Config.Region) for us to deploy an environment in.
		envRegion, err := findUnusedRegion("us-west", *appSess.Config.Region)
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
			AppName:                  app.Name,
			PublicLoadBalancer:       true,
			ToolsAccountPrincipalARN: envCallerInfo.RootUserARN,
		}
		envStackName := fmt.Sprintf("%s-%s",
			environmentToDeploy.AppName,
			environmentToDeploy.Name)

		// Make sure we delete the stacks after the test is done
		defer func() {
			// delete the pipeline first because it relies on stackset
			_, err := appCfClient.DeleteStack(&awsCF.DeleteStackInput{
				StackName: aws.String(pipelineStackName),
			})
			require.NoError(t, err)
			err = appCfClient.WaitUntilStackDeleteComplete(&awsCF.DescribeStacksInput{
				StackName: aws.String(pipelineStackName),
			})
			require.NoError(t, err)

			// Clean up any StackInstances we may have created.
			if stackInstances, err := appCfClient.ListStackInstances(&awsCF.ListStackInstancesInput{
				StackSetName: aws.String(appStackSetName),
			}); err == nil && stackInstances.Summaries != nil && stackInstances.Summaries[0] != nil {
				appStackInstance := stackInstances.Summaries[0]
				_, err := appCfClient.DeleteStackInstances(&awsCF.DeleteStackInstancesInput{
					Accounts:     []*string{appStackInstance.Account},
					Regions:      []*string{appStackInstance.Region},
					RetainStacks: aws.Bool(false),
					StackSetName: appStackInstance.StackSetId,
				})
				require.NoError(t, err)

				err = appCfClient.WaitUntilStackDeleteComplete(&awsCF.DescribeStacksInput{
					StackName: appStackInstance.StackId,
				})
				require.NoError(t, err)
			}
			// Delete the StackSet once all the StackInstances are cleaned up
			_, err = appCfClient.DeleteStackSet(&awsCF.DeleteStackSetInput{
				StackSetName: aws.String(appStackSetName),
			})
			require.NoError(t, err)

			_, err = appCfClient.DeleteStack(&awsCF.DeleteStackInput{
				StackName: aws.String(appRoleStackName),
			})
			require.NoError(t, err)
			err = appCfClient.WaitUntilStackDeleteComplete(&awsCF.DescribeStacksInput{
				StackName: aws.String(appRoleStackName),
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

			deleteMockSecretImmediately(t, sm, secretId)
		}()

		// Given both the application stack and env we are deploying to do not
		// exist
		assertStackDoesNotExist(t, appCfClient, appRoleStackName)
		assertStackDoesNotExist(t, envCfClient, envStackName)

		// create a stackset
		err = appDeployer.DeployApp(&deploy.CreateAppInput{
			Name:      app.Name,
			AccountID: app.AccountID,
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
		// no existing copilot environment.
		err = appDeployer.AddPipelineResourcesToApp(
			&app,
			*appSess.Config.Region)
		require.NoError(t, err)

		stackInstances, err := appCfClient.ListStackInstances(&awsCF.ListStackInstancesInput{
			StackSetName: aws.String(appStackSetName),
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(stackInstances.Summaries),
			"application stack instance should exist")

		resources, err := appDeployer.GetRegionalAppResources(&app)
		require.NoError(t, err)
		artifactBuckets := regionalResourcesToArtifactBuckets(t, resources)

		pipelineInput := &deploy.CreatePipelineInput{
			AppName: app.Name,
			Name:    pipelineStackName,
			Source: &deploy.Source{
				ProviderName: manifest.GithubProviderName,
				Properties: map[string]interface{}{
					"repository":                   "chicken/wings",
					"branch":                       "master",
					manifest.GithubSecretIdKeyName: secretId,
				},
			},
			Stages: []deploy.PipelineStage{
				{
					AssociatedEnvironment: &deploy.AssociatedEnvironment{
						Name:      environmentToDeploy.Name,
						Region:    *appSess.Config.Region,
						AccountID: app.AccountID,
						Prod:      true,
					},
					LocalServices: []string{"frontend", "backend"},
				},
			},
			ArtifactBuckets: artifactBuckets,
		}
		require.NoError(t, appDeployer.CreatePipeline(pipelineInput))

		// Ensure that the new stack exists
		assertStackExists(t, appCfClient, pipelineStackName)
	})
}

func createMockSecret(t *testing.T, sm secretsmanageriface.SecretsManagerAPI, secretId string) {
	_, err := sm.GetSecretValue(&secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretId),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case secretsmanager.ErrCodeResourceNotFoundException:
				_, err = sm.CreateSecret(&secretsmanager.CreateSecretInput{
					Name:         aws.String(secretId),
					SecretString: aws.String("dontCare"),
				})
				require.NoError(t, err, "CreateSecret should not return an error")
				return
			default:
				require.Fail(t, "GetSecretValue failed: %w", awsErr)
			}
		}
		require.Fail(t, "GetSecretValue failed: %w", err)
	}
}

func deleteMockSecretImmediately(t *testing.T, sm secretsmanageriface.SecretsManagerAPI, secretId string) {
	_, err := sm.DeleteSecret(&secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(secretId),
		ForceDeleteWithoutRecovery: aws.Bool(true),
	})
	require.NoError(t, err, "DeleteSecret should not return an error")
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

func regionalResourcesToArtifactBuckets(t *testing.T, resources []*stack.AppRegionalResources) []deploy.ArtifactBucket {
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
