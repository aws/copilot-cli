// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

/** How to add `ApplyEnv` unit test to a new manifest field:

When writing tests for a field F (e.g. When writing `TestApplyEnv_Image`, where F would be the `image` field):
	For each subfield f in F:
		- If f has subfields || f is a composite type (e.g. `StringOrStringSlice`, `BuildStringOrArgs`) ->
			1. Write a test case when f field is nil.
			2. Write a test case when f field is non-nil.
		- Otherwise, write three test cases for f ->
			1. Write a test case when f field is nil.
			2. Write a test case when f field is non-nil, and the referenced value is empty (e.g., it is "", {}, 0).
			3. Write a test case when f field is non-nil, and the referenced value is NOT empty.

	For each subfield f in F:
		- If f is mutually exclusive with another subfield g of F (e.g. `image.location` and `image.build` are mutually exclusive) ->
			1. Write a test case that make sure f is nil when g is non-nil
			2. Write a test case that make sure g is nil when f is non-nil

	For each subfield f in F:
		- If f has subfields || if f is a composite field ->
			Write another test group for this field (e.g. F is `image` and f is `image.build`, write another test functions named `TestApplyEnv_Image_Build`)

Expected Behaviors:
	- Slice type: override-only.
		Take `security_groups` (which takes []string) as an example. If original is `[]string{1, 2}`, and environment override
		is `[]string{3}`, the result should be `[]string{3}`.
	- Map: override value of existing keys, append non-existing keys.
*/

func TestApplyEnv_Image(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"exclusive fields: build overridden if location is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String("mockBuild"),
				}
				svc.Environments["test"].ImageConfig.Location = aws.String("mockLocation")
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Location = aws.String("mockLocation")
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs:   DockerBuildArgs{},
				}
			},
		},
		"exclusive fields: location overridden if build is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Location = aws.String("mockLocation")
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Dockerfile: aws.String("mockDockerfile"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Location = nil
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Dockerfile: aws.String("mockDockerfile"),
					},
				}
			},
		},
		"build overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String("mockBuild"),
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String("mockBuildTest"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String("mockBuildTest"),
				}
			},
		},
		"build not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String("mockBuild"),
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String("mockBuild"),
				}
			},
		},
		"location overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Location = aws.String("mockLocation")
				svc.Environments["test"].ImageConfig.Location = aws.String("mockLocationTest")
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Location = aws.String("mockLocationTest")
			},
		},
		"location explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Location = aws.String("mockLocation")
				svc.Environments["test"].ImageConfig.Location = aws.String("")
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Location = aws.String("")
			},
		},
		"location not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Location = aws.String("mockLocation")
				svc.Environments["test"].ImageConfig.Image = Image{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Location = aws.String("mockLocation")
			},
		},
		"credentials overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Credentials = aws.String("mockCredentials")
				svc.Environments["test"].ImageConfig.Credentials = aws.String("mockCredentialsTest")
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Credentials = aws.String("mockCredentialsTest")
			},
		},
		"credentials explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Credentials = aws.String("mockCredentials")
				svc.Environments["test"].ImageConfig.Credentials = aws.String("")
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Credentials = aws.String("")
			},
		},
		"credentials not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Credentials = aws.String("mockCredentials")
				svc.Environments["test"].ImageConfig.Image = Image{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Credentials = aws.String("mockCredentials")
			},
		},
		"labels overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.DockerLabels = map[string]string{
					"mockLabel1": "1",
					"mockLabel2": "2",
				}
				svc.Environments["test"].ImageConfig.DockerLabels = map[string]string{
					"mockLabel1": "3", // Override the value of mockLabel1
					"mockLabel3": "3", // Append a new label mockLabel3
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.DockerLabels = map[string]string{
					"mockLabel1": "3",
					"mockLabel2": "2",
					"mockLabel3": "3",
				}
			},
		},
		"labels not overridden by empty map": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.DockerLabels = map[string]string{
					"mockLabel1": "mockValue1",
					"mockLabel2": "mockValue2",
				}
				svc.Environments["test"].ImageConfig.DockerLabels = map[string]string{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.DockerLabels = map[string]string{
					"mockLabel1": "mockValue1",
					"mockLabel2": "mockValue2",
				}
			},
		},
		"labels not overridden by nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.DockerLabels = map[string]string{
					"mockLabel1": "mockValue1",
					"mockLabel2": "mockValue2",
				}
				svc.Environments["test"].ImageConfig.Image = Image{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.DockerLabels = map[string]string{
					"mockLabel1": "mockValue1",
					"mockLabel2": "mockValue2",
				}
			},
		},
		"depends_on overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.DependsOn = map[string]string{
					"mockContainer1": "1",
					"mockContainer2": "2",
				}
				svc.Environments["test"].ImageConfig.DependsOn = map[string]string{
					"mockContainer1": "3", // Override the condition of mockContainer1
					"mockContainer3": "3", // Append a new dependency on mockContainer3
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.DependsOn = map[string]string{
					"mockContainer1": "3",
					"mockContainer2": "2",
					"mockContainer3": "3",
				}
			},
		},
		"depends_on not overridden by empty map": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.DependsOn = map[string]string{
					"mockContainer1": "1",
					"mockContainer2": "2",
				}
				svc.Environments["test"].ImageConfig.DependsOn = map[string]string{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.DependsOn = map[string]string{
					"mockContainer1": "1",
					"mockContainer2": "2",
				}
			},
		},
		"depends_on not overridden by nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.DependsOn = map[string]string{
					"mockContainer1": "1",
					"mockContainer2": "2",
				}
				svc.Environments["test"].ImageConfig.Image = Image{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.DependsOn = map[string]string{
					"mockContainer1": "1",
					"mockContainer2": "2",
				}
			},
		},
		"port overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(1)
				svc.Environments["test"].ImageConfig.Port = aws.Uint16(2)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(2)
			},
		},
		"port explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(1)
				svc.Environments["test"].ImageConfig.Port = aws.Uint16(0)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(0)
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: port not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(1)
				svc.Environments["test"].ImageConfig.Image = Image{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(1)
			},
		},
		"healthcheck overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Retries: aws.Int(3),
				}

				mockInterval1Minute := 60 * time.Second
				svc.Environments["test"].ImageConfig.HealthCheck = &ContainerHealthCheck{
					Interval: &mockInterval1Minute,
					Retries:  aws.Int(5),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockInterval1Minute := 60 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Interval: &mockInterval1Minute,
					Retries:  aws.Int(5),
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_STRUCT: healthcheck not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Retries: aws.Int(3),
				}
				svc.Environments["test"].ImageConfig.Image = Image{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Command:     nil,
					Interval:    nil,
					Retries:     aws.Int(3),
					Timeout:     nil,
					StartPeriod: nil,
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc LoadBalancedWebService
			inSvc.Environments = map[string]*LoadBalancedWebServiceConfig{
				"test": {},
			}

			tc.inSvc(&inSvc)
			tc.wanted(&wantedSvc)

			got, err := inSvc.ApplyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}

func TestApplyEnv_Image_Build(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"composite fields: build string is overridden if build args is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String("mockBuild"),
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Context:    aws.String("mockContext"),
						Dockerfile: aws.String("mockDockerfile"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Context:    aws.String("mockContext"),
						Dockerfile: aws.String("mockDockerfile"),
					},
				}
			},
		},
		"composite fields: build args is overridden if build string is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Context:    aws.String("mockContext"),
						Dockerfile: aws.String("mockDockerfile"),
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String("mockBuild"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String("mockBuild"),
					BuildArgs:   DockerBuildArgs{},
				}
			},
		},
		"build string overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String("mockBuild"),
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String("mockBuildTest"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String("mockBuildTest"),
					BuildArgs:   DockerBuildArgs{},
				}
			},
		},
		"build string explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String("mockBuild"),
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String(""),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String(""),
					BuildArgs:   DockerBuildArgs{},
				}
			},
		},
		"build string not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String("mockBuild"),
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: aws.String("mockBuild"),
					BuildArgs:   DockerBuildArgs{},
				}
			},
		},
		"build arg overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Context: aws.String("mockContext"),
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Context:    aws.String("mockContextTest"),
						Dockerfile: aws.String("mockDockerfileTest"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Context:    aws.String("mockContextTest"),
						Dockerfile: aws.String("mockDockerfileTest"),
					},
				}
			},
		},
		"build arg not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Context: aws.String("mockContext"),
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Context: aws.String("mockContext"),
					},
				}
			},
		},
		"context overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Context: aws.String("mockContext"),
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Context: aws.String("mockContextTest"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Context: aws.String("mockContextTest"),
					},
				}
			},
		},
		"context explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Context: aws.String("mockContext"),
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Context: aws.String(""),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Context: aws.String(""),
					},
				}
			},
		},
		"context not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Context: aws.String("mockContext"),
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Context: aws.String("mockContext"),
					},
				}
			},
		},
		"dockerfile overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Dockerfile: aws.String("mockDockerfile"),
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Dockerfile: aws.String("mockDockerfileTest"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Dockerfile: aws.String("mockDockerfileTest"),
					},
				}
			},
		},
		"dockerfile explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Dockerfile: aws.String("mockDockerfile"),
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Dockerfile: aws.String(""),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Dockerfile: aws.String(""),
					},
				}
			},
		},
		"dockerfile not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Dockerfile: aws.String("mockDockerfile"),
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Dockerfile: aws.String("mockDockerfile"),
					},
				}
			},
		},
		"FIXED_BUG: args overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Args: map[string]string{
							"mockArg1": "1",
							"mockArg2": "2",
						},
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Args: map[string]string{
							"mockArg1": "3", // Override value for mockArg1
							"mockArg3": "3", // Append an arg mockArg3
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Args: map[string]string{
							"mockArg1": "3",
							"mockArg2": "2",
							"mockArg3": "3",
						},
					},
				}
			},
		},
		"FIXED_BUG: args not overridden by empty map": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Args: map[string]string{
							"mockArg1": "1",
							"mockArg2": "2",
						},
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Args: map[string]string{},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Args: map[string]string{
							"mockArg1": "1",
							"mockArg2": "2",
						},
					},
				}
			},
		},
		"args not overridden by nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Args: map[string]string{
							"mockArg1": "1",
							"mockArg2": "2",
						},
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Args: map[string]string{
							"mockArg1": "1",
							"mockArg2": "2",
						},
					},
				}
			},
		},
		"target overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Target: aws.String("mockTarget"),
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Target: aws.String("mockTargetTest"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Target: aws.String("mockTargetTest"),
					},
				}
			},
		},
		"target explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Target: aws.String("mockTarget"),
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Target: aws.String(""),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Target: aws.String(""),
					},
				}
			},
		},
		"target not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Target: aws.String("mockTarget"),
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Target: aws.String("mockTarget"),
					},
				}
			},
		},
		"cacheFrom overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						CacheFrom: []string{"mock", "Cache"},
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						CacheFrom: []string{"mock", "CacheTest", "Test"},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						CacheFrom: []string{"mock", "CacheTest", "Test"},
					},
				}
			},
		},
		"cacheFrom overridden by zero slice": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						CacheFrom: []string{"mock", "Cache"},
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						CacheFrom: []string{},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						CacheFrom: []string{},
					},
				}
			},
		},
		"cacheFrom not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						CacheFrom: []string{"mock", "Cache"},
					},
				}
				svc.Environments["test"].ImageConfig.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						CacheFrom: []string{"mock", "Cache"},
					},
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc LoadBalancedWebService
			inSvc.Environments = map[string]*LoadBalancedWebServiceConfig{
				"test": {},
			}

			tc.inSvc(&inSvc)
			tc.wanted(&wantedSvc)

			got, err := inSvc.ApplyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}

func TestApplyEnv_Image_HealthCheck(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"command overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Command: []string{"mock", "command"},
				}
				svc.Environments["test"].ImageConfig.HealthCheck = &ContainerHealthCheck{
					Command: []string{"mock", "command_test", "test"},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Command: []string{"mock", "command_test", "test"},
				}
			},
		},
		"command overridden by zero slice": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Command: []string{"mock", "command"},
				}
				svc.Environments["test"].ImageConfig.HealthCheck = &ContainerHealthCheck{
					Command: []string{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Command: []string{},
				}
			},
		},
		"FIXED BUG: command not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Command: []string{"mock", "command"},
				}
				svc.Environments["test"].ImageConfig.HealthCheck = &ContainerHealthCheck{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Command: []string{"mock", "command"},
				}
			},
		},
		"interval overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockInterval := 600 * time.Second
				mockIntervalTest := 50 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Interval: &mockInterval,
				}
				svc.Environments["test"].ImageConfig.HealthCheck = &ContainerHealthCheck{
					Interval: &mockIntervalTest,
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockIntervalTest := 50 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Interval: &mockIntervalTest,
				}
			},
		},
		"interval explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockInterval := 600 * time.Second
				mockIntervalTest := 0 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Interval: &mockInterval,
				}
				svc.Environments["test"].ImageConfig.HealthCheck = &ContainerHealthCheck{
					Interval: &mockIntervalTest,
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockIntervalTest := 0 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Interval: &mockIntervalTest,
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: interval not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockInterval := 600 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Interval: &mockInterval,
				}
				svc.Environments["test"].ImageConfig.HealthCheck = &ContainerHealthCheck{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockIntervalTest := 600 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Interval: &mockIntervalTest,
				}
			},
		},
		"retries overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Retries: aws.Int(13),
				}
				svc.Environments["test"].ImageConfig.HealthCheck = &ContainerHealthCheck{
					Retries: aws.Int(42),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Retries: aws.Int(42),
				}
			},
		},
		"retries explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Retries: aws.Int(13),
				}
				svc.Environments["test"].ImageConfig.HealthCheck = &ContainerHealthCheck{
					Retries: aws.Int(0),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Retries: aws.Int(0),
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: retries not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Retries: aws.Int(13),
				}
				svc.Environments["test"].ImageConfig.HealthCheck = &ContainerHealthCheck{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Retries: aws.Int(13),
				}
			},
		},
		"timeout overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockTimeout := 60 * time.Second
				mockTimeoutTest := 400 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Timeout: &mockTimeout,
				}
				svc.Environments["test"].ImageConfig.HealthCheck = &ContainerHealthCheck{
					Timeout: &mockTimeoutTest,
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockTimeoutTest := 400 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Timeout: &mockTimeoutTest,
				}
			},
		},
		"timeout explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockTimeout := 60 * time.Second
				mockTimeoutTest := 0 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Timeout: &mockTimeout,
				}
				svc.Environments["test"].ImageConfig.HealthCheck = &ContainerHealthCheck{
					Timeout: &mockTimeoutTest,
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockTimeoutTest := 0 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Timeout: &mockTimeoutTest,
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: timeout not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockTimeout := 60 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Timeout: &mockTimeout,
				}
				svc.Environments["test"].ImageConfig.HealthCheck = &ContainerHealthCheck{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockTimeoutTest := 60 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					Timeout: &mockTimeoutTest,
				}
			},
		},
		"start_period overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockStartPeriod := 10 * time.Second
				mockStartPeriodTest := 300 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					StartPeriod: &mockStartPeriod,
				}
				svc.Environments["test"].ImageConfig.HealthCheck = &ContainerHealthCheck{
					StartPeriod: &mockStartPeriodTest,
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockStartPeriod := 300 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					StartPeriod: &mockStartPeriod,
				}
			},
		},
		"start_period explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockStartPeriod := 10 * time.Second
				mockStartPeriodTest := 0 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					StartPeriod: &mockStartPeriod,
				}
				svc.Environments["test"].ImageConfig.HealthCheck = &ContainerHealthCheck{
					StartPeriod: &mockStartPeriodTest,
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockStartPeriod := 0 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					StartPeriod: &mockStartPeriod,
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: start_period not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockStartPeriod := 10 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					StartPeriod: &mockStartPeriod,
				}
				svc.Environments["test"].ImageConfig.HealthCheck = &ContainerHealthCheck{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockStartPeriod := 10 * time.Second
				svc.ImageConfig.HealthCheck = &ContainerHealthCheck{
					StartPeriod: &mockStartPeriod,
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc LoadBalancedWebService
			inSvc.Environments = map[string]*LoadBalancedWebServiceConfig{
				"test": {},
			}

			tc.inSvc(&inSvc)
			tc.wanted(&wantedSvc)

			got, err := inSvc.ApplyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}

func TestApplyEnv_Platform(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"FIXED_BUG: composite fields: platform string is overridden if platform args is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatform"),
				}
				svc.Environments["test"].Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String("mock"),
						Arch:     aws.String("platformTest"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String("mock"),
						Arch:     aws.String("platformTest"),
					},
				}
			},
		},
		"FIXED_BUG: composite fields: platform args is overridden if platform string is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String("mock"),
						Arch:     aws.String("platformTest"),
					},
				}
				svc.Environments["test"].Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatform"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatform"),
				}
			},
		},
		"platform string overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatform"),
				}
				svc.Environments["test"].Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatformTest"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatformTest"),
				}
			},
		},
		"platform string explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatform"),
				}
				svc.Environments["test"].Platform = &PlatformArgsOrString{
					PlatformString: aws.String(""),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformString: aws.String(""),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: platform string not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatform"),
				}
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatform"),
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: platform args overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String("mock"),
						Arch:     aws.String("platform"),
					},
				}
				svc.Environments["test"].Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						Arch: aws.String("platformTest"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String("mock"),
						Arch:     aws.String("platformTest"),
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: platform args not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String("mock"),
						Arch:     aws.String("platform"),
					},
				}
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String("mock"),
						Arch:     aws.String("platform"),
					},
				}
			},
		},
		"osfamily string overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String("mockOSFamily"),
					},
				}
				svc.Environments["test"].Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String("mockOSFamilyTest"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String("mockOSFamilyTest"),
					},
				}
			},
		},
		"osfamily string explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String("mockOSFamily"),
					},
				}
				svc.Environments["test"].Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String(""),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String(""),
					},
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: osfamily string not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String("mockOSFamily"),
					},
				}
				svc.Environments["test"].Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String("mockOSFamily"),
					},
				}
			},
		},
		"architecture string overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						Arch: aws.String("mockArch"),
					},
				}
				svc.Environments["test"].Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						Arch: aws.String("mockArchTest"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						Arch: aws.String("mockArchTest"),
					},
				}
			},
		},
		"architecture string explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						Arch: aws.String("mockArch"),
					},
				}
				svc.Environments["test"].Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						Arch: aws.String(""),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						Arch: aws.String(""),
					},
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: architecture string not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						Arch: aws.String("mockArch"),
					},
				}
				svc.Environments["test"].Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						Arch: aws.String("mockArch"),
					},
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc LoadBalancedWebService
			inSvc.Environments = map[string]*LoadBalancedWebServiceConfig{
				"test": {},
			}

			tc.inSvc(&inSvc)
			tc.wanted(&wantedSvc)

			got, err := inSvc.ApplyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}

func TestApplyEnv_Entrypoint(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"composite fields: string slice is overridden if string is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = &EntryPointOverride{
					StringSlice: []string{"mock", "entrypoint"},
				}
				svc.Environments["test"].EntryPoint = &EntryPointOverride{
					String: aws.String("mock entrypoint test"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = &EntryPointOverride{
					String: aws.String("mock entrypoint test"),
				}
			},
		},
		"FIXED_BUG: composite fields: string is overridden if string slice is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = &EntryPointOverride{
					String: aws.String("mock entrypoint"),
				}
				svc.Environments["test"].EntryPoint = &EntryPointOverride{
					StringSlice: []string{"mock", "entrypoint_test", "test"},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = &EntryPointOverride{
					StringSlice: []string{"mock", "entrypoint_test", "test"},
				}
			},
		},
		"string overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = &EntryPointOverride{
					String: aws.String("mock entrypoint"),
				}
				svc.Environments["test"].EntryPoint = &EntryPointOverride{
					String: aws.String("mock entrypoint test"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = &EntryPointOverride{
					String: aws.String("mock entrypoint test"),
				}
			},
		},
		"string explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = &EntryPointOverride{
					String: aws.String("mock entrypoint"),
				}
				svc.Environments["test"].EntryPoint = &EntryPointOverride{
					String: aws.String(""),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = &EntryPointOverride{
					String: aws.String(""),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: string not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = &EntryPointOverride{
					String: aws.String("mock entrypoint"),
				}
				svc.Environments["test"].ImageOverride = ImageOverride{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = &EntryPointOverride{
					String: aws.String("mock entrypoint"),
				}
			},
		},
		"string slice overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = &EntryPointOverride{
					StringSlice: []string{"mock", "entrypoint"},
				}
				svc.Environments["test"].EntryPoint = &EntryPointOverride{
					StringSlice: []string{"mock", "entrypoint_test", "test"},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = &EntryPointOverride{
					StringSlice: []string{"mock", "entrypoint_test", "test"},
				}
			},
		},
		"string slice explicitly overridden by zero slice": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = &EntryPointOverride{
					StringSlice: []string{"mock", "entrypoint"},
				}
				svc.Environments["test"].EntryPoint = &EntryPointOverride{
					StringSlice: []string{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = &EntryPointOverride{
					StringSlice: []string{},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: string slice not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = &EntryPointOverride{
					StringSlice: []string{"mock", "entrypoint"},
				}
				svc.Environments["test"].ImageOverride = ImageOverride{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = &EntryPointOverride{
					StringSlice: []string{"mock", "entrypoint"},
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc LoadBalancedWebService
			inSvc.Environments = map[string]*LoadBalancedWebServiceConfig{
				"test": {},
			}

			tc.inSvc(&inSvc)
			tc.wanted(&wantedSvc)

			got, err := inSvc.ApplyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}

func TestApplyEnv_Command(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"composite fields: string slice is overridden if string is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Command = &CommandOverride{
					StringSlice: []string{"mock", "command"},
				}
				svc.Environments["test"].Command = &CommandOverride{
					String: aws.String("mock command test"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Command = &CommandOverride{
					String: aws.String("mock command test"),
				}
			},
		},
		"FIXED_BUG: composite fields: string is overridden if string slice is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Command = &CommandOverride{
					String: aws.String("mock command"),
				}
				svc.Environments["test"].Command = &CommandOverride{
					StringSlice: []string{"mock", "command_test", "test"},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Command = &CommandOverride{
					StringSlice: []string{"mock", "command_test", "test"},
				}
			},
		},
		"string overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Command = &CommandOverride{
					String: aws.String("mock command"),
				}
				svc.Environments["test"].Command = &CommandOverride{
					String: aws.String("mock command test"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Command = &CommandOverride{
					String: aws.String("mock command test"),
				}
			},
		},
		"string explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Command = &CommandOverride{
					String: aws.String("mock command"),
				}
				svc.Environments["test"].Command = &CommandOverride{
					String: aws.String(""),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Command = &CommandOverride{
					String: aws.String(""),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: string not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Command = &CommandOverride{
					String: aws.String("mock command"),
				}
				svc.Environments["test"].ImageOverride = ImageOverride{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Command = &CommandOverride{
					String: aws.String("mock command"),
				}
			},
		},
		"string slice overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Command = &CommandOverride{
					StringSlice: []string{"mock", "command"},
				}
				svc.Environments["test"].Command = &CommandOverride{
					StringSlice: []string{"mock", "command_test", "test"},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Command = &CommandOverride{
					StringSlice: []string{"mock", "command_test", "test"},
				}
			},
		},
		"string slice explicitly overridden by zero slice": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Command = &CommandOverride{
					StringSlice: []string{"mock", "command"},
				}
				svc.Environments["test"].Command = &CommandOverride{
					StringSlice: []string{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Command = &CommandOverride{
					StringSlice: []string{},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: string slice not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Command = &CommandOverride{
					StringSlice: []string{"mock", "command"},
				}
				svc.Environments["test"].ImageOverride = ImageOverride{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Command = &CommandOverride{
					StringSlice: []string{"mock", "command"},
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc LoadBalancedWebService
			inSvc.Environments = map[string]*LoadBalancedWebServiceConfig{
				"test": {},
			}

			tc.inSvc(&inSvc)
			tc.wanted(&wantedSvc)

			got, err := inSvc.ApplyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}

func TestApplyEnv_Logging(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"image overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					Image: aws.String("mockImage"),
				}
				svc.Environments["test"].Logging = &Logging{
					Image: aws.String("mockImageTest"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					Image: aws.String("mockImageTest"),
				}
			},
		},
		"image explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					Image: aws.String("mockImage"),
				}
				svc.Environments["test"].Logging = &Logging{
					Image: aws.String(""),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					Image: aws.String(""),
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: image not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					Image: aws.String("mockImage"),
				}
				svc.Environments["test"].Logging = &Logging{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					Image: aws.String("mockImage"),
				}
			},
		},
		"destination overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					Destination: map[string]string{
						"mockDestination1": "1",
						"mockDestination2": "2",
					},
				}
				svc.Environments["test"].Logging = &Logging{
					Destination: map[string]string{
						"mockDestination1": "3", // Modify the value of mockDestination1.
						"mockDestination3": "3", // Append mockDestination3.
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					Destination: map[string]string{
						"mockDestination1": "3",
						"mockDestination2": "2",
						"mockDestination3": "3",
					},
				}
			},
		},
		"destination not overridden by empty map": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					Destination: map[string]string{
						"mockDestination1": "1",
						"mockDestination2": "2",
					},
				}
				svc.Environments["test"].Logging = &Logging{
					Destination: map[string]string{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					Destination: map[string]string{
						"mockDestination1": "1",
						"mockDestination2": "2",
					},
				}
			},
		},
		"destination not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					Destination: map[string]string{
						"mockDestination1": "1",
						"mockDestination2": "2",
					},
				}
				svc.Environments["test"].Logging = &Logging{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					Destination: map[string]string{
						"mockDestination1": "1",
						"mockDestination2": "2",
					},
				}
			},
		},
		"enableMetadata overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					EnableMetadata: aws.Bool(false),
				}
				svc.Environments["test"].Logging = &Logging{
					EnableMetadata: aws.Bool(true),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					EnableMetadata: aws.Bool(true),
				}
			},
		},
		"enableMetadata explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					EnableMetadata: aws.Bool(true),
				}
				svc.Environments["test"].Logging = &Logging{
					EnableMetadata: aws.Bool(false),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					EnableMetadata: aws.Bool(false),
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: enableMetadata not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					EnableMetadata: aws.Bool(true),
				}
				svc.Environments["test"].Logging = &Logging{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					EnableMetadata: aws.Bool(true),
				}
			},
		},
		"secretOptions overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					SecretOptions: map[string]string{
						"mockSecretOption1": "1",
						"mockSecretOption2": "2",
					},
				}
				svc.Environments["test"].Logging = &Logging{
					SecretOptions: map[string]string{
						"mockSecretOption1": "3", // Modify the value of mockSecretOption1.
						"mockSecretOption3": "3", // Append mockSecretOption3.
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					SecretOptions: map[string]string{
						"mockSecretOption1": "3",
						"mockSecretOption2": "2",
						"mockSecretOption3": "3",
					},
				}
			},
		},
		"secretOptions not overridden by empty map": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					SecretOptions: map[string]string{
						"mockSecretOption1": "1",
						"mockSecretOption2": "2",
					},
				}
				svc.Environments["test"].Logging = &Logging{
					SecretOptions: map[string]string{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					SecretOptions: map[string]string{
						"mockSecretOption1": "1",
						"mockSecretOption2": "2",
					},
				}
			},
		},
		"secretOptions not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					SecretOptions: map[string]string{
						"mockSecretOption1": "1",
						"mockSecretOption2": "2",
					},
				}
				svc.Environments["test"].Logging = &Logging{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					SecretOptions: map[string]string{
						"mockSecretOption1": "1",
						"mockSecretOption2": "2",
					},
				}
			},
		},
		"configFilePath overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					ConfigFile: aws.String("mockPath"),
				}
				svc.Environments["test"].Logging = &Logging{
					ConfigFile: aws.String("mockPathTest"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					ConfigFile: aws.String("mockPathTest"),
				}
			},
		},
		"configFilePath explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					ConfigFile: aws.String("mockPath"),
				}
				svc.Environments["test"].Logging = &Logging{
					ConfigFile: aws.String(""),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					ConfigFile: aws.String(""),
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: configFilePath not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					ConfigFile: aws.String("mockPath"),
				}
				svc.Environments["test"].Logging = &Logging{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = &Logging{
					ConfigFile: aws.String("mockPath"),
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc LoadBalancedWebService
			inSvc.Environments = map[string]*LoadBalancedWebServiceConfig{
				"test": {},
			}

			tc.inSvc(&inSvc)
			tc.wanted(&wantedSvc)

			got, err := inSvc.ApplyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}

func TestApplyEnv_Network(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"vpc overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement: aws.String("mockPlacement"),
					},
				}
				svc.Environments["test"].Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement:      aws.String("mockPlacementTest"),
						SecurityGroups: []string{"mock", "security", "group"},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement:      aws.String("mockPlacementTest"),
						SecurityGroups: []string{"mock", "security", "group"},
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: vpc not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement: aws.String("mockPlacement"),
					},
				}
				svc.Environments["test"].Network = &NetworkConfig{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement: aws.String("mockPlacement"),
					},
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc LoadBalancedWebService
			inSvc.Environments = map[string]*LoadBalancedWebServiceConfig{
				"test": {},
			}

			tc.inSvc(&inSvc)
			tc.wanted(&wantedSvc)

			got, err := inSvc.ApplyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}

func TestApplyEnv_Network_VPC(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"placement overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement: aws.String("mockPlacement"),
					},
				}
				svc.Environments["test"].Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement: aws.String("mockPlacementTest"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement: aws.String("mockPlacementTest"),
					},
				}
			},
		},
		"placement explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement: aws.String("mockPlacement"),
					},
				}
				svc.Environments["test"].Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement: aws.String(""),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement: aws.String(""),
					},
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: placement not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement: aws.String("mockPlacement"),
					},
				}
				svc.Environments["test"].Network = &NetworkConfig{
					VPC: &vpcConfig{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement: aws.String("mockPlacement"),
					},
				}
			},
		},
		"security_groups overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Network = &NetworkConfig{
					VPC: &vpcConfig{
						SecurityGroups: []string{"mock", "security_group"},
					},
				}
				svc.Environments["test"].Network = &NetworkConfig{
					VPC: &vpcConfig{
						SecurityGroups: []string{"mock", "security_group_test", "test"},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Network = &NetworkConfig{
					VPC: &vpcConfig{
						SecurityGroups: []string{"mock", "security_group_test", "test"},
					},
				}
			},
		},
		"security_groups overridden by zero slice": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Network = &NetworkConfig{
					VPC: &vpcConfig{
						SecurityGroups: []string{"mock", "security_group"},
					},
				}
				svc.Environments["test"].Network = &NetworkConfig{
					VPC: &vpcConfig{
						SecurityGroups: []string{},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Network = &NetworkConfig{
					VPC: &vpcConfig{
						SecurityGroups: []string{},
					},
				}
			},
		},
		"FIXED_BUG: security_groups not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Network = &NetworkConfig{
					VPC: &vpcConfig{
						SecurityGroups: []string{"mock", "security_group"},
					},
				}
				svc.Environments["test"].Network = &NetworkConfig{
					VPC: &vpcConfig{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Network = &NetworkConfig{
					VPC: &vpcConfig{
						SecurityGroups: []string{"mock", "security_group"},
					},
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc LoadBalancedWebService
			inSvc.Environments = map[string]*LoadBalancedWebServiceConfig{
				"test": {},
			}

			tc.inSvc(&inSvc)
			tc.wanted(&wantedSvc)

			got, err := inSvc.ApplyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}
