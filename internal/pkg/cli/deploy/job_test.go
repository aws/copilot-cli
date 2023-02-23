// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/override"
)

func TestJobDeployer_GenerateCloudFormationTemplate(t *testing.T) {
	t.Run("ensure resulting CloudFormation template custom resource paths are empty", func(t *testing.T) {
		// GIVEN
		job := mockJobDeployer()

		// WHEN
		out, err := job.GenerateCloudFormationTemplate(&GenerateCloudFormationTemplateInput{})

		// THEN
		require.NoError(t, err)

		type lambdaFn struct {
			Properties struct {
				Code struct {
					S3Bucket string `yaml:"S3bucket"`
					S3Key    string `yaml:"S3Key"`
				} `yaml:"Code"`
			} `yaml:"Properties"`
		}
		dat := struct {
			Resources struct {
				EnvControllerFunction lambdaFn `yaml:"EnvControllerFunction"`
			} `yaml:"Resources"`
		}{}
		require.NoError(t, yaml.Unmarshal([]byte(out.Template), &dat))
		require.Empty(t, dat.Resources.EnvControllerFunction.Properties.Code.S3Bucket)
		require.Empty(t, dat.Resources.EnvControllerFunction.Properties.Code.S3Key)
	})
}

func mockJobDeployer(opts ...func(*jobDeployer)) *jobDeployer {
	deployer := &jobDeployer{
		workloadDeployer: &workloadDeployer{
			name: "example",
			app: &config.Application{
				Name: "demo",
			},
			env: &config.Environment{
				App:  "demo",
				Name: "test",
			},
			resources:        &stack.AppRegionalResources{},
			envConfig:        new(manifest.Environment),
			endpointGetter:   &mockEndpointGetter{endpoint: "demo.test.local"},
			envVersionGetter: &mockEnvVersionGetter{version: "v1.0.0"},
			overrider:        new(override.Noop),
		},
		jobMft: &manifest.ScheduledJob{
			Workload: manifest.Workload{
				Name: aws.String("example"),
			},
			ScheduledJobConfig: manifest.ScheduledJobConfig{
				TaskConfig: manifest.TaskConfig{
					Count: manifest.Count{
						Value: aws.Int(1),
					},
				},
				On: manifest.JobTriggerConfig{
					Schedule: aws.String("@daily"),
				},
				ImageConfig: manifest.ImageWithHealthcheck{
					Image: manifest.Image{
						ImageLocationOrBuild: manifest.ImageLocationOrBuild{
							Build: manifest.BuildArgsOrString{BuildString: aws.String("/Dockerfile")},
						},
					},
				},
			},
		},
		newStack: func() cloudformation.StackConfiguration {
			return new(stubCloudFormationStack)
		},
	}
	for _, opt := range opts {
		opt(deployer)
	}
	return deployer
}
