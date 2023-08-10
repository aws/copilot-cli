//go:build integration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation_test

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	awsCF "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/copilot-cli/internal/pkg/aws/iam"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	awss3 "github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/stretchr/testify/require"
)

const (
	fmtIAMRoleARN         = "arn:aws:iam::%s:role/%s"
	fmtCFNExecutionRoleID = "%s-%s-CFNExecutionRole"
	fmtEnvManagerRoleID   = "%s-%s-EnvManagerRole"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func Test_App_Infrastructure(t *testing.T) {
	sess, err := testSession(nil)
	require.NoError(t, err)
	identity := identity.New(sess)
	callerInfo, err := identity.Get()
	require.NoError(t, err)
	require.NoError(t, err)
	deployer := cloudformation.New(sess, cloudformation.WithProgressTracker(os.Stderr))
	cfClient := awsCF.New(sess)
	require.NoError(t, err)
	version.Version = "v1.28.0"

	t.Run("Deploys Application Admin Roles to CloudFormation and Creates StackSet", func(t *testing.T) {
		app := config.Application{Name: randStringBytes(10), AccountID: callerInfo.Account}
		appRoleStackName := fmt.Sprintf("%s-infrastructure-roles", app.Name)
		appStackSetName := fmt.Sprintf("%s-infrastructure", app.Name)

		// Given our stack doesn't exist
		roleStackOutput, err := cfClient.DescribeStacks(&awsCF.DescribeStacksInput{
			StackName: aws.String(appRoleStackName),
		})
		require.Error(t, err)
		require.True(t, len(roleStackOutput.Stacks) == 0, "Stack %s should not exist.", appRoleStackName)

		// Make sure we delete the stacks after the test is done
		defer func() {
			cfClient.DeleteStack(&awsCF.DeleteStackInput{
				StackName: aws.String(appRoleStackName),
			})

			cfClient.DeleteStackSet(&awsCF.DeleteStackSetInput{
				StackSetName: aws.String(appStackSetName),
			})

			cfClient.WaitUntilStackDeleteComplete(&awsCF.DescribeStacksInput{
				StackName: aws.String(appRoleStackName),
			})
		}()

		err = deployer.DeployApp(&deploy.CreateAppInput{
			Name:      app.Name,
			AccountID: app.AccountID,
			Version:   version.LatestTemplateVersion(),
		})
		require.NoError(t, err)

		// Query using our resources as well:
		resources, err := deployer.GetRegionalAppResources(&app)
		require.NoError(t, err)
		require.True(t, len(resources) == 0, "No resources for %s should exist.", appRoleStackName)

		// We should create the application StackSet
		_, err = cfClient.DescribeStackSet(&awsCF.DescribeStackSetInput{
			StackSetName: aws.String(appStackSetName),
		})
		require.NoError(t, err)

		// We should create the application roles stack
		roleStackOutput, err = cfClient.DescribeStacks(&awsCF.DescribeStacksInput{
			StackName: aws.String(appRoleStackName),
		})
		require.NoError(t, err)

		require.True(t, len(roleStackOutput.Stacks) == 1, "Stack %s should have been deployed.", appRoleStackName)
		deployedStack := roleStackOutput.Stacks[0]
		expectedResultsForKey := map[string]func(*awsCF.Output){
			"ExecutionRoleARN": func(output *awsCF.Output) {
				require.True(t,
					strings.HasSuffix(*output.OutputValue, fmt.Sprintf("role/%s-executionrole", app.Name)),
					fmt.Sprintf("ExecutionRoleARN should be named {app}-executionrole but was %s", *output.OutputValue))
			},
			"AdministrationRoleARN": func(output *awsCF.Output) {
				require.True(t,
					strings.HasSuffix(*output.OutputValue, fmt.Sprintf("role/%s-adminrole", app.Name)),
					fmt.Sprintf("AdministrationRoleARN should be named {app}-adminrole but was %s", *output.OutputValue))
			},
			"TemplateVersion": func(output *awsCF.Output) {
				require.Equal(t, *output.OutputValue, version.LatestTemplateVersion(),
					fmt.Sprintf("TemplateVersion should be %s but was %s", version.LatestTemplateVersion(), *output.OutputValue))
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

	t.Run("Deploys Application Infrastructure (KMS Key, ECR Repo, S3 Bucket)", func(t *testing.T) {
		app := config.Application{Name: randStringBytes(10), AccountID: callerInfo.Account}
		appRoleStackName := fmt.Sprintf("%s-infrastructure-roles", app.Name)
		appStackSetName := fmt.Sprintf("%s-infrastructure", app.Name)

		// Make sure we delete the stacks after the test is done
		defer func() {
			// Clean up any StackInstances we may have created.
			if stackInstances, err := cfClient.ListStackInstances(&awsCF.ListStackInstancesInput{
				StackSetName: aws.String(appStackSetName),
			}); err == nil && stackInstances.Summaries != nil && stackInstances.Summaries[0] != nil {
				appStackInstance := stackInstances.Summaries[0]
				cfClient.DeleteStackInstances(&awsCF.DeleteStackInstancesInput{
					Accounts:     []*string{appStackInstance.Account},
					Regions:      []*string{appStackInstance.Region},
					RetainStacks: aws.Bool(false),
					StackSetName: appStackInstance.StackSetId,
				})

				cfClient.WaitUntilStackDeleteComplete(&awsCF.DescribeStacksInput{
					StackName: appStackInstance.StackId,
				})
			}
			// Delete the StackSet once all the StackInstances are cleaned up
			cfClient.DeleteStackSet(&awsCF.DeleteStackSetInput{
				StackSetName: aws.String(appStackSetName),
			})

			cfClient.DeleteStack(&awsCF.DeleteStackInput{
				StackName: aws.String(appRoleStackName),
			})
		}()

		// Given our stack doesn't exist
		roleStackOutput, err := cfClient.DescribeStacks(&awsCF.DescribeStacksInput{
			StackName: aws.String(appRoleStackName),
		})
		require.Error(t, err)
		require.True(t, len(roleStackOutput.Stacks) == 0, "Stack %s should not exist.", appRoleStackName)

		err = deployer.DeployApp(&deploy.CreateAppInput{
			Name:      app.Name,
			AccountID: app.AccountID,
			Version:   version.LatestTemplateVersion(),
		})
		require.NoError(t, err)

		// Add a service only
		err = deployer.AddServiceToApp(
			&app,
			"mysvc",
		)

		require.NoError(t, err)

		// Add a service with dash
		err = deployer.AddServiceToApp(
			&app,
			"mysvc-frontend",
		)

		require.NoError(t, err)

		// No new substacks should be created
		stackInstances, err := cfClient.ListStackInstances(&awsCF.ListStackInstancesInput{
			StackSetName: aws.String(appStackSetName),
		})
		require.NoError(t, err)
		require.Equal(t, 0, len(stackInstances.Summaries), "Adding apps to an application without any envs shouldn't create any stack instances.")

		// Add an environment only
		err = deployer.AddEnvToApp(
			&cloudformation.AddEnvToAppOpts{
				App:          &app,
				EnvName:      "test",
				EnvAccountID: callerInfo.Account,
				EnvRegion:    *sess.Config.Region,
			},
		)
		require.NoError(t, err)

		// Query using our GetRegionalAppResources function.
		resources, err := deployer.GetRegionalAppResources(&app)
		require.NoError(t, err)
		require.True(t, len(resources) == 1, "One stack should exist.")
		stackResources := resources[0]
		require.True(t, len(stackResources.RepositoryURLs) == 2, "Two repos should exist")
		require.True(t, stackResources.RepositoryURLs["mysvc-frontend"] != "", "Repo URL shouldn't be blank")
		require.True(t, stackResources.RepositoryURLs["mysvc"] != "", "Repo URL shouldn't be blank")
		require.True(t, stackResources.S3Bucket != "", "S3 Bucket shouldn't be blank")
		require.True(t, stackResources.KMSKeyARN != "", "KMSKey ARN shouldn't be blank")

		// Validate resources by comparing physical output of the stacks.
		stackInstances, err = cfClient.ListStackInstances(&awsCF.ListStackInstancesInput{
			StackSetName: aws.String(appStackSetName),
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(stackInstances.Summaries), "Adding an env should create a new stack instance.")
		appStackInstance := stackInstances.Summaries[0]
		// We should create the application roles stack
		appInfraStacks, err := cfClient.DescribeStacks(&awsCF.DescribeStacksInput{
			StackName: appStackInstance.StackId,
		})
		require.NoError(t, err)

		deployedStack := appInfraStacks.Stacks[0]
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
			"TemplateVersion": func(output *awsCF.Output) {
				require.Equal(t, *output.OutputValue, version.LatestTemplateVersion(),
					fmt.Sprintf("TemplateVersion should be %s but was %s", version.LatestTemplateVersion(), *output.OutputValue))
			},
			"ECRRepomysvc": func(output *awsCF.Output) {
				require.True(t,
					strings.HasSuffix(*output.OutputValue, fmt.Sprintf("repository/%s/mysvc", app.Name)),
					fmt.Sprintf("ECRRepomysvc should be suffixed with repository/{app}/mysvc but was %s", *output.OutputValue))
			},
			// We replace dashes with the word DASH for logical IDss
			"ECRRepomysvcDASHfrontend": func(output *awsCF.Output) {
				require.True(t,
					strings.HasSuffix(*output.OutputValue, fmt.Sprintf("repository/%s/mysvc-frontend", app.Name)),
					fmt.Sprintf("ECRRepomysvcDASHfrontend should be suffixed with repository/{app}/mysvc but was %s", *output.OutputValue))
			},
			"StackSetOpId": func(output *awsCF.Output) {
				opID, err := strconv.Atoi(*output.OutputValue)
				require.NoError(t, err)
				require.GreaterOrEqual(t, opID, 1,
					fmt.Sprintf("StackSetOpId should be > 1 but was %s", *output.OutputValue))
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

	t.Run("Deploys supporting infrastructure for pipeline (KMS Key, S3 Bucket)", func(t *testing.T) {
		app := config.Application{Name: randStringBytes(10), AccountID: callerInfo.Account}
		appRoleStackName := fmt.Sprintf("%s-infrastructure-roles", app.Name)
		appStackSetName := fmt.Sprintf("%s-infrastructure", app.Name)

		// Make sure we delete the stacks after the test is done
		defer func() {
			// Clean up any StackInstances we may have created.
			if stackInstances, err := cfClient.ListStackInstances(&awsCF.ListStackInstancesInput{
				StackSetName: aws.String(appStackSetName),
			}); err == nil && stackInstances.Summaries != nil && stackInstances.Summaries[0] != nil {
				appStackInstance := stackInstances.Summaries[0]
				cfClient.DeleteStackInstances(&awsCF.DeleteStackInstancesInput{
					Accounts:     []*string{appStackInstance.Account},
					Regions:      []*string{appStackInstance.Region},
					RetainStacks: aws.Bool(false),
					StackSetName: appStackInstance.StackSetId,
				})

				cfClient.WaitUntilStackDeleteComplete(&awsCF.DescribeStacksInput{
					StackName: appStackInstance.StackId,
				})
			}
			// Delete the StackSet once all the StackInstances are cleaned up
			cfClient.DeleteStackSet(&awsCF.DeleteStackSetInput{
				StackSetName: aws.String(appStackSetName),
			})

			cfClient.DeleteStack(&awsCF.DeleteStackInput{
				StackName: aws.String(appRoleStackName),
			})
		}()

		// Given our stack doesn't exist
		_, err := cfClient.DescribeStacks(&awsCF.DescribeStacksInput{
			StackName: aws.String(appRoleStackName),
		})
		require.Error(t, err, "DescribeStacks should return an error because the stack does not exist")
		awsErr, ok := err.(awserr.Error)
		require.True(t, ok, "the returned error should be an awserr")
		require.Equal(t, awsErr.Code(), "ValidationError")
		require.Contains(t, awsErr.Message(), "does not exist", "the returned error should indicate that the stack does not exist")

		// create a stackset
		err = deployer.DeployApp(&deploy.CreateAppInput{
			Name:      app.Name,
			AccountID: app.AccountID,
			Version:   version.LatestTemplateVersion(),
		})
		require.NoError(t, err)

		// Add resources needed to support a pipeline in a region
		err = deployer.AddPipelineResourcesToApp(&app, *sess.Config.Region)
		require.NoError(t, err)

		// Add another pipeline to the same application and region. This should not create
		// Additional stack instance
		err = deployer.AddPipelineResourcesToApp(&app, *sess.Config.Region)
		require.NoError(t, err)

		stackInstances, err := cfClient.ListStackInstances(&awsCF.ListStackInstancesInput{
			StackSetName: aws.String(appStackSetName),
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(stackInstances.Summaries), "Adding 2 pipelines to the same application should not create 2 stack instances")

		// add an environment should not create new stack instance in the same region
		err = deployer.AddEnvToApp(&cloudformation.AddEnvToAppOpts{
			App:          &app,
			EnvName:      "test",
			EnvAccountID: callerInfo.Account,
			EnvRegion:    *sess.Config.Region,
		})
		require.NoError(t, err)

		stackInstances, err = cfClient.ListStackInstances(&awsCF.ListStackInstancesInput{
			StackSetName: aws.String(appStackSetName),
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(stackInstances.Summaries), "Adding 2 pipelines to the same application should not create 2 stack instances")

		// Ensure the bucket and KMS key were created
		resources, err := deployer.GetRegionalAppResources(&app)
		require.NoError(t, err)
		require.True(t, len(resources) == 1, "One stack should exist.")
		stackResources := resources[0]
		require.True(t, stackResources.S3Bucket != "", "S3 Bucket shouldn't be blank")
		require.True(t, stackResources.KMSKeyARN != "", "KMSKey ARN shouldn't be blank")
	})
}

// This test can take quite long as it spins up a CloudFormation stack
// and then waits for it to be deleted. If you find your CF stack
// is failing to be spun up because you've reached some limits, try
// switching your default region by running aws configure.
func Test_Environment_Deployment_Integration(t *testing.T) {
	version.Version = "v1.28.0"
	sess, err := testSession(nil)
	require.NoError(t, err)
	deployer := cloudformation.New(sess, cloudformation.WithProgressTracker(os.Stderr))
	cfClient := awsCF.New(sess)
	identity := identity.New(sess)
	s3ManagerClient := s3manager.NewUploader(sess)
	s3Client := awss3.New(sess)
	iamClient := iam.New(sess)
	id, err := identity.Get()
	require.NoError(t, err)

	envName := randStringBytes(10)
	appName := randStringBytes(10)
	bucketName := randStringBytes(10)
	environmentToDeploy := stack.EnvConfig{
		Name: envName,
		App: deploy.AppInformation{
			Name:                appName,
			AccountPrincipalARN: id.RootUserARN,
		},
		Version: version.LatestTemplateVersion(),
	}
	envStackName := fmt.Sprintf("%s-%s", environmentToDeploy.App.Name, environmentToDeploy.Name)

	// Given our stack doesn't exist
	output, err := cfClient.DescribeStacks(&awsCF.DescribeStacksInput{
		StackName: aws.String(envStackName),
	})
	require.Error(t, err)
	require.True(t, len(output.Stacks) == 0, "Stack %s should not exist.", envStackName)

	// Create a temporary S3 bucket to store custom resource scripts.
	_, err = s3ManagerClient.S3.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	require.NoError(t, err)

	// Make sure we delete the stack after the test is done.
	defer func() {
		_, err := cfClient.DeleteStack(&awsCF.DeleteStackInput{
			StackName: aws.String(envStackName),
		})
		require.NoError(t, err)

		err = deleteEnvRoles(appName, envName, id.Account, iamClient)
		require.NoError(t, err)

		err = s3Client.EmptyBucket(bucketName)
		require.NoError(t, err)
		_, err = s3ManagerClient.S3.DeleteBucket(&s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
		require.NoError(t, err)
	}()

	t.Run("Deploys bootstrap resources for the environment to CloudFormation", func(t *testing.T) {
		bucketARN := fmt.Sprintf("arn:aws:s3:::%s", bucketName)
		environmentToDeploy.ArtifactBucketKeyARN = "arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"
		environmentToDeploy.ArtifactBucketARN = bucketARN

		// Deploy the environment and wait for it to be complete
		require.NoError(t, deployer.CreateAndRenderEnvironment(stack.NewBootstrapEnvStackConfig(&environmentToDeploy), bucketARN))

		// Ensure that the new stack exists
		output, err := cfClient.DescribeStacks(&awsCF.DescribeStacksInput{
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
			"CFNExecutionRoleARN": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-CFNExecutionRoleARN", envStackName),
					*output.ExportName,
					"Should export CRNExecutionRole ARN")

				require.True(t,
					strings.HasSuffix(*output.OutputValue, fmt.Sprintf("role/%s-CFNExecutionRole", envStackName)),
					"CRNExecutionRole ARN value should not be nil.")
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

	t.Run("Deploys an environment to CloudFormation", func(t *testing.T) {
		crs, err := customresource.Env(template.New())
		require.NoError(t, err)
		urls, err := customresource.Upload(func(key string, dat io.Reader) (url string, err error) {
			return s3Client.Upload(bucketName, key, dat)
		}, crs)
		require.NoError(t, err)
		environmentToDeploy.CustomResourcesURLs = urls
		environmentToDeploy.ArtifactBucketKeyARN = "arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"
		environmentToDeploy.ArtifactBucketARN = fmt.Sprintf("arn:aws:s3:::%s", bucketName)
		environmentToDeploy.Mft = &manifest.Environment{
			Workload: manifest.Workload{
				Name: aws.String(envName),
				Type: aws.String("Environment"),
			},
		}

		// Deploy the environment and wait for it to be complete.
		oldParams, err := deployer.DeployedEnvironmentParameters(environmentToDeploy.App.Name, environmentToDeploy.Name)
		require.NoError(t, err)
		lastForceUpdateID, err := deployer.ForceUpdateOutputID(environmentToDeploy.App.Name, environmentToDeploy.Name)
		require.NoError(t, err)
		conf, err := stack.NewEnvConfigFromExistingStack(&environmentToDeploy, lastForceUpdateID, oldParams)
		require.NoError(t, err)
		// Deploy the environment and wait for it to be complete.
		require.NoError(t, deployer.UpdateAndRenderEnvironment(conf, environmentToDeploy.ArtifactBucketARN, false))

		// Ensure that the updated stack still exists.
		output, err := cfClient.DescribeStacks(&awsCF.DescribeStacksInput{
			StackName: aws.String(envStackName),
		})
		require.NoError(t, err)
		require.True(t, len(output.Stacks) == 1, "Stack %s should have been deployed.", envStackName)

		deployedStack := output.Stacks[0]
		expectedResultsForKey := map[string]func(*awsCF.Output){
			"EnabledFeatures": func(output *awsCF.Output) {
				require.Equal(t, ",,,,,", aws.StringValue(output.OutputValue), "no env features enabled by default")
			},
			"EnvironmentManagerRoleARN": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-EnvironmentManagerRoleARN", envStackName),
					*output.ExportName,
					"Should export EnvironmentManagerRole ARN")

				require.True(t,
					strings.HasSuffix(*output.OutputValue, fmt.Sprintf("role/%s-EnvManagerRole", envStackName)),
					"EnvironmentManagerRole ARN value should not be nil.")
			},
			"CFNExecutionRoleARN": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-CFNExecutionRoleARN", envStackName),
					*output.ExportName,
					"Should export CRNExecutionRole ARN")

				require.True(t,
					strings.HasSuffix(*output.OutputValue, fmt.Sprintf("role/%s-CFNExecutionRole", envStackName)),
					"CRNExecutionRole ARN value should not be nil.")
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
			"PublicSubnets": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-PublicSubnets", envStackName),
					*output.ExportName,
					"Should export PublicSubnets as stackname-PublicSubnets")

				require.NotNil(t,
					output.OutputValue,
					"PublicSubnets value should not be nil")
			},
			"ServiceDiscoveryNamespaceID": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-ServiceDiscoveryNamespaceID", envStackName),
					*output.ExportName,
					"Should export ServiceDiscoveryNamespaceID as stackname-ServiceDiscoveryNamespaceID")

				require.NotNil(t,
					output.OutputValue,
					"ServiceDiscoveryNamespaceID value should not be nil")
			},
			"InternetGatewayID": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-InternetGatewayID", envStackName),
					*output.ExportName,
					"Should export InternetGatewayID as stackname-InternetGatewayID")

				require.NotNil(t,
					output.OutputValue,
					"InternetGatewayID value should not be nil")
			},
			"PublicRouteTableID": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-PublicRouteTableID", envStackName),
					*output.ExportName,
					"Should export PublicRouteTableID as stackname-PublicRouteTableID when creating environment with default VPC")

				require.NotNil(t,
					output.OutputValue,
					"PublicRouteTableID value should not be nil")
			},
			"EnvironmentSecurityGroup": func(output *awsCF.Output) {
				require.Equal(t,
					fmt.Sprintf("%s-EnvironmentSecurityGroup", envStackName),
					*output.ExportName,
					"Should export EnvironmentSecurityGroup as stackname-EnvironmentSecurityGroup")

				require.NotNil(t,
					output.OutputValue,
					"EnvironmentSecurityGroup value should not be nil")
			},
			"LastForceDeployID": func(output *awsCF.Output) {
				require.Equal(t, lastForceUpdateID, aws.StringValue(output.OutputValue), "last force update id does not change by default")
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

func testSession(region *string) (*session.Session, error) {
	if region == nil {
		return session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
		})
	}

	// override with the provided region
	return session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			CredentialsChainVerboseErrors: aws.Bool(true),
			Region:                        region,
		},
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

func deleteEnvRoles(app, env, accountNumber string, iam *iam.IAM) error {
	cfnExecRoleID := fmt.Sprintf(fmtCFNExecutionRoleID, app, env)
	envManagerRoleID := fmt.Sprintf(fmtEnvManagerRoleID, app, env)
	cfnExecutionRoleARN := fmt.Sprintf(fmtIAMRoleARN, accountNumber, cfnExecRoleID)
	envManagerRoleARN := fmt.Sprintf(fmtIAMRoleARN, accountNumber, envManagerRoleID)

	err := iam.DeleteRole(cfnExecutionRoleARN)
	if err != nil {
		return err
	}
	err = iam.DeleteRole(envManagerRoleARN)
	if err != nil {
		return err
	}

	return nil
}
