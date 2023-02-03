// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"

	"gopkg.in/yaml.v3"

	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/override"
)

func TestLbWebSvcDeployer_GenerateCloudFormationTemplate(t *testing.T) {
	t.Run("ensure resulting CloudFormation template custom resource paths are empty", func(t *testing.T) {
		// GIVEN
		lbws := mockLoadBalancedWebServiceDeployer()

		// WHEN
		out, err := lbws.GenerateCloudFormationTemplate(&GenerateCloudFormationTemplateInput{})

		// THEN
		require.NoError(t, err)

		type lambdaFn struct {
			Properties struct {
				Code struct {
					S3Bucket string `yaml:"S3Bucket"`
					S3Key    string `yaml:"S3Key"`
				} `yaml:"Code"`
			} `yaml:"Properties"`
		}
		dat := struct {
			Resources struct {
				EnvControllerFunction lambdaFn `yaml:"EnvControllerFunction"`
				RulePriorityFunction  lambdaFn `yaml:"RulePriorityFunction"`
			} `yaml:"Resources"`
		}{}
		require.NoError(t, yaml.Unmarshal([]byte(out.Template), &dat))
		require.Equal(t, "stackset-demo-bucket", dat.Resources.EnvControllerFunction.Properties.Code.S3Bucket)
		require.Contains(t, dat.Resources.EnvControllerFunction.Properties.Code.S3Key, "manual/scripts/custom-resources/")

		require.Equal(t, "stackset-demo-bucket", dat.Resources.RulePriorityFunction.Properties.Code.S3Bucket)
		require.Contains(t, dat.Resources.RulePriorityFunction.Properties.Code.S3Key, "manual/scripts/custom-resources/")
	})
}

func mockLoadBalancedWebServiceDeployer(opts ...func(deployer *lbWebSvcDeployer)) *lbWebSvcDeployer {
	deployer := &lbWebSvcDeployer{
		svcDeployer: &svcDeployer{
			workloadDeployer: &workloadDeployer{
				name: "example",
				app: &config.Application{
					Name: "demo",
				},
				env: &config.Environment{
					App:  "demo",
					Name: "test",
				},
				resources: &stack.AppRegionalResources{
					Region:   "us-west-2",
					S3Bucket: "stackset-demo-bucket",
				},
				envConfig:        new(manifest.Environment),
				endpointGetter:   &mockEndpointGetter{endpoint: "demo.test.local"},
				envVersionGetter: &mockEnvVersionGetter{version: "v1.0.0"},
				overrider:        new(override.Noop),
				templateFS:       fakeTemplateFS(),
				customResources:  lbwsCustomResources,
			},
			newSvcUpdater: func(f func(*session.Session) serviceForceUpdater) serviceForceUpdater {
				return nil
			},
			now: func() time.Time {
				return time.Date(2020, 11, 23, 0, 0, 0, 0, time.UTC)
			},
		},
		lbMft: &manifest.LoadBalancedWebService{
			Workload: manifest.Workload{
				Name: aws.String("example"),
			},
			LoadBalancedWebServiceConfig: manifest.LoadBalancedWebServiceConfig{
				TaskConfig: manifest.TaskConfig{
					Count: manifest.Count{
						Value: aws.Int(1),
					},
				},
				ImageConfig: manifest.ImageWithPortAndHealthcheck{
					ImageWithPort: manifest.ImageWithPort{
						Image: manifest.Image{
							Build: manifest.BuildArgsOrString{BuildString: aws.String("/Dockerfile")},
						},
						Port: aws.Uint16(80),
					},
				},
				RoutingRule: manifest.RoutingRuleConfigOrBool{
					RoutingRuleConfiguration: manifest.RoutingRuleConfiguration{
						Path: aws.String("/"),
					},
				},
			},
		},
	}
	for _, opt := range opts {
		opt(deployer)
	}
	return deployer
}
