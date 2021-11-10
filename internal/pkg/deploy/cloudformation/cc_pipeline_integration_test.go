//go:build integration
// +build integration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	awsCF "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

func TestCCPipelineCreation(t *testing.T) {
	appSess, err := testSession(nil)
	require.NoError(t, err)
	appId := identity.New(appSess)
	appCallerInfo, err := appId.Get()
	require.NoError(t, err)
	appDeployer := cloudformation.New(appSess)

	t.Run("creates a cross-region pipeline in a region with no environment", func(t *testing.T) {
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
		s3Client := s3.New(envSess)
		uploader := template.New()
		var envBucketName string
		var appBucketName string

		environmentToDeploy := deploy.CreateEnvironmentInput{
			Name: randStringBytes(10),
			App: deploy.AppInformation{
				AccountPrincipalARN: envCallerInfo.RootUserARN,
				Name:                app.Name,
			},
			Version: deploy.LatestEnvTemplateVersion,
		}
		envStackName := fmt.Sprintf("%s-%s",
			environmentToDeploy.App.Name,
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
			stackInstances, err := appCfClient.ListStackInstances(&awsCF.ListStackInstancesInput{
				StackSetName: aws.String(appStackSetName),
			})
			require.NoError(t, err)
			require.Equal(t, 2, len(stackInstances.Summaries))

			err = s3Client.EmptyBucket(envBucketName)
			require.NoError(t, err)

			appS3Client := s3.New(appSess)
			err = appS3Client.EmptyBucket(appBucketName)
			require.NoError(t, err)

			_, err = appCfClient.DeleteStackInstances(&awsCF.DeleteStackInstancesInput{
				Accounts:     []*string{stackInstances.Summaries[0].Account},
				Regions:      []*string{aws.String(envRegion.ID()), appSess.Config.Region},
				RetainStacks: aws.Bool(false),
				StackSetName: aws.String(appStackSetName),
			})
			require.NoError(t, err)

			for _, summary := range stackInstances.Summaries {
				sess, err := testSession(summary.Region)
				require.NoError(t, err)
				client := awsCF.New(sess)
				err = client.WaitUntilStackDeleteComplete(&awsCF.DescribeStacksInput{
					StackName: summary.StackId,
				})
				require.NoError(t, err)
			}

			// Delete the StackSet once all the StackInstances are cleaned up. There could be a delay that
			// stack instances are all deleted but still returns OperationInProgressException error.
			retry := 0
			for ; retry < maxDeleteStackSetRetryNum; retry++ {
				if _, err = appCfClient.DeleteStackSet(&awsCF.DeleteStackSetInput{
					StackSetName: aws.String(appStackSetName),
				}); isOperationInProgress(err) {
					time.Sleep(deleteStackSetRetryInterval)
					continue
				}
				require.NoError(t, err)
				break
			}
			require.NotEqual(t, retry, maxDeleteStackSetRetryNum)

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
		}()

		// Given both the application stack and env we are deploying to do not
		// exist
		assertStackDoesNotExist(t, appCfClient, appRoleStackName)
		assertStackDoesNotExist(t, envCfClient, envStackName)

		// create a stackset
		err = appDeployer.DeployApp(&deploy.CreateAppInput{
			Name:      app.Name,
			AccountID: app.AccountID,
			Version:   deploy.LatestAppTemplateVersion,
		})
		require.NoError(t, err)

		err = appDeployer.AddEnvToApp(&cloudformation.AddEnvToAppOpts{
			App:          &app,
			EnvName:      environmentToDeploy.Name,
			EnvRegion:    envRegion.ID(),
			EnvAccountID: envCallerInfo.Account,
		})
		require.NoError(t, err)

		regionalResource, err := appDeployer.GetAppResourcesByRegion(&app, envRegion.ID())
		require.NoError(t, err)
		envBucketName = regionalResource.S3Bucket
		urls, err := uploader.UploadEnvironmentCustomResources(s3.CompressAndUploadFunc(func(key string, objects ...s3.NamedBinary) (string, error) {
			return s3Client.ZipAndUpload(envBucketName, key, objects...)
		}))
		require.NoError(t, err)
		environmentToDeploy.CustomResourcesURLs = urls

		// Deploy the environment in the same tools account but in different
		// region and wait for it to be complete
		require.NoError(t, envDeployer.DeployAndRenderEnvironment(os.Stderr, &environmentToDeploy))

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
		require.Equal(t, 2, len(stackInstances.Summaries),
			"application stack instance should exist")

		resources, err := appDeployer.GetRegionalAppResources(&app)
		require.NoError(t, err)
		artifactBuckets := regionalResourcesToArtifactBuckets(t, resources)

		pipelineInput := &deploy.CreatePipelineInput{
			AppName: app.Name,
			Name:    pipelineStackName,
			Source: &deploy.CodeCommitSource{
				ProviderName:  manifest.CodeCommitProviderName,
				Branch:        "main",
				RepositoryURL: "https://us-west-2.console.aws.amazon.com/codesuite/codecommit/repositories/repo-name/browse",
			},
			Build: deploy.PipelineBuildFromManifest(nil),
			Stages: []deploy.PipelineStage{
				{
					AssociatedEnvironment: &deploy.AssociatedEnvironment{
						Name:      environmentToDeploy.Name,
						Region:    *appSess.Config.Region,
						AccountID: app.AccountID,
					},
					LocalWorkloads: []string{"frontend", "backend"},
				},
			},
			ArtifactBuckets: artifactBuckets,
		}
		appRegionResources, err := appDeployer.GetAppResourcesByRegion(&app, *appSess.Config.Region)
		require.NoError(t, err)
		appBucketName = appRegionResources.S3Bucket
		require.NoError(t, appDeployer.CreatePipeline(pipelineInput, appBucketName))

		// Ensure that the new stack exists
		assertStackExists(t, appCfClient, pipelineStackName)
	})
}
