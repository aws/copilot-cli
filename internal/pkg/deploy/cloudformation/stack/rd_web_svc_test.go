// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var (
	testPrivatePlacement = manifest.PrivateSubnetPlacement
)

var testRDWebServiceManifest = &manifest.RequestDrivenWebService{
	Workload: manifest.Workload{
		Name: aws.String(testServiceName),
		Type: aws.String(manifest.RequestDrivenWebServiceType),
	},
	RequestDrivenWebServiceConfig: manifest.RequestDrivenWebServiceConfig{
		ImageConfig: manifest.ImageWithPort{
			Port: aws.Uint16(80),
		},
		InstanceConfig: manifest.AppRunnerInstanceConfig{
			CPU:    aws.Int(256),
			Memory: aws.Int(512),
		},
		Variables: map[string]string{
			"LOG_LEVEL": "info",
			"NODE_ENV":  "development",
		},
		RequestDrivenWebServiceHttpConfig: manifest.RequestDrivenWebServiceHttpConfig{
			HealthCheckConfiguration: manifest.HealthCheckArgsOrString{
				HealthCheckPath: aws.String("/"),
			},
		},
		Tags: map[string]string{
			"owner": "jeff",
		},
	},
}

func TestRequestDrivenWebService_NewRequestDrivenWebService(t *testing.T) {
	type testInput struct {
		mft     *manifest.RequestDrivenWebService
		env     string
		rc      RuntimeConfig
		appInfo deploy.AppInformation
		urls    map[string]string
	}

	testCases := map[string]struct {
		input            testInput
		mockDependencies func(t *testing.T, ctrl *gomock.Controller, c *RequestDrivenWebService)

		wantedStack *RequestDrivenWebService
		wantedError error
	}{
		"should return RequestDrivenWebService": {
			input: testInput{
				mft: testRDWebServiceManifest,
				env: testEnvName,
				rc:  RuntimeConfig{},
				appInfo: deploy.AppInformation{
					Name: testAppName,
				},
				urls: map[string]string{
					"custom-domain-app-runner": "mockURL1",
					"aws-sdk-layer":            "mockURL2",
				},
			},

			wantedStack: &RequestDrivenWebService{
				appRunnerWkld: &appRunnerWkld{
					wkld: &wkld{
						name:  aws.StringValue(testRDWebServiceManifest.Name),
						env:   testEnvName,
						app:   testAppName,
						rc:    RuntimeConfig{},
						image: testRDWebServiceManifest.ImageConfig.Image,
					},
					instanceConfig: testRDWebServiceManifest.InstanceConfig,
					imageConfig:    testRDWebServiceManifest.ImageConfig,
				},
				manifest: testRDWebServiceManifest,
				app: deploy.AppInformation{
					Name: testAppName,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			addons := mocks.NewMockaddons(ctrl)

			stack, err := NewRequestDrivenWebService(RequestDrivenWebServiceConfig{
				App:           tc.input.appInfo,
				Env:           tc.input.env,
				Manifest:      tc.input.mft,
				RuntimeConfig: tc.input.rc,
				Addons:        addons,
			})

			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedStack.name, stack.name)
			require.Equal(t, tc.wantedStack.env, stack.env)
			require.Equal(t, tc.wantedStack.app, stack.app)
			require.Equal(t, tc.wantedStack.rc, stack.rc)
			require.Equal(t, tc.wantedStack.image, stack.image)
			require.Equal(t, tc.wantedStack.manifest, stack.manifest)
			require.Equal(t, tc.wantedStack.instanceConfig, stack.instanceConfig)
			require.Equal(t, tc.wantedStack.imageConfig, stack.imageConfig)
			require.NotNil(t, stack.addons)
			require.NotNil(t, stack.parser)
		})
	}
}

func TestRequestDrivenWebService_Template(t *testing.T) {
	const mockSD = "app.env.svc.local.com"
	testCases := map[string]struct {
		inCustomResourceURLs map[string]string
		inManifest           func(manifest manifest.RequestDrivenWebService) manifest.RequestDrivenWebService
		mockDependencies     func(t *testing.T, ctrl *gomock.Controller, c *RequestDrivenWebService)
		wantedTemplate       string
		wantedError          error
	}{
		"should throw an error if addons template cannot be parsed": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *RequestDrivenWebService) {
				mockParser := mocks.NewMockrequestDrivenWebSvcReadParser(ctrl)
				addons := mockAddons{tplErr: errors.New("some error")}
				c.parser = mockParser
				c.wkld.addons = addons
			},
			wantedError: fmt.Errorf("generate addons template for %s: %w", testServiceName, errors.New("some error")),
		},
		"should be able to parse custom resource URLs when alias is enabled": {
			inManifest: func(mft manifest.RequestDrivenWebService) manifest.RequestDrivenWebService {
				mft.Alias = aws.String("convex.domain.com")
				mft.Network.VPC.Placement = manifest.PlacementArgOrString{
					PlacementString: &testPrivatePlacement,
				}
				return mft
			},
			inCustomResourceURLs: map[string]string{
				template.AppRunnerCustomDomainLambdaFileName: "https://mockbucket.s3-us-east-1.amazonaws.com/mockURL1",
			},
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *RequestDrivenWebService) {
				mockParser := mocks.NewMockrequestDrivenWebSvcReadParser(ctrl)
				addons := mockAddons{}
				mockParser.EXPECT().ParseRequestDrivenWebService(gomock.Any()).DoAndReturn(func(actual template.WorkloadOpts) (*template.Content, error) {
					require.Equal(t, template.WorkloadOpts{
						AppName:           "phonetool",
						EnvName:           "test",
						WorkloadName:      "frontend",
						WorkloadType:      manifest.RequestDrivenWebServiceType,
						Variables:         c.manifest.Variables,
						Tags:              c.manifest.Tags,
						EnableHealthCheck: true,
						Alias:             aws.String("convex.domain.com"),
						CustomResources: map[string]template.S3ObjectLocation{
							template.AppRunnerCustomDomainLambdaFileName: {
								Bucket: "mockbucket",
								Key:    "mockURL1",
							},
						},
						Network: template.NetworkOpts{
							SubnetsType: "PrivateSubnets",
						},
						ServiceDiscoveryEndpoint: mockSD,
						AWSSDKLayer:              aws.String("arn:aws:lambda:us-west-2:420165488524:layer:AWSLambda-Node-AWS-SDK:14"),
					}, actual)
					return &template.Content{Buffer: bytes.NewBufferString("template")}, nil
				})
				c.parser = mockParser
				c.wkld.addons = addons
			},
			wantedTemplate: "template",
		},
		"should parse template without addons/ directory": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *RequestDrivenWebService) {
				mockParser := mocks.NewMockrequestDrivenWebSvcReadParser(ctrl)
				addons := mockAddons{}
				mockParser.EXPECT().ParseRequestDrivenWebService(gomock.Any()).DoAndReturn(func(actual template.WorkloadOpts) (*template.Content, error) {
					require.Equal(t, template.WorkloadOpts{
						AppName:                  "phonetool",
						EnvName:                  "test",
						WorkloadName:             "frontend",
						WorkloadType:             manifest.RequestDrivenWebServiceType,
						Variables:                c.manifest.Variables,
						Tags:                     c.manifest.Tags,
						ServiceDiscoveryEndpoint: mockSD,
						EnableHealthCheck:        true,
						CustomResources:          make(map[string]template.S3ObjectLocation),
					}, actual)
					return &template.Content{Buffer: bytes.NewBufferString("template")}, nil
				})
				c.parser = mockParser
				c.addons = addons
			},
			wantedTemplate: "template",
		},
		"should parse template with addons": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *RequestDrivenWebService) {
				mockParser := mocks.NewMockrequestDrivenWebSvcReadParser(ctrl)
				addons := mockAddons{
					tpl: `Resources:
  AdditionalResourcesPolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      PolicyDocument:
        Statement:
        - Effect: Allow
          Action: '*'
          Resource: '*'
  DDBTable:
    Type: AWS::DynamoDB::Table
    Properties:
      TableName: 'Hello'
Outputs:
  AdditionalResourcesPolicyArn:
    Value: !Ref AdditionalResourcesPolicy
  DDBTableName:
    Value: !Ref DDBTable
  Hello:
    Value: hello`,
				}
				mockParser.EXPECT().ParseRequestDrivenWebService(gomock.Any()).DoAndReturn(func(actual template.WorkloadOpts) (*template.Content, error) {
					require.Equal(t, template.WorkloadOpts{
						AppName:                  "phonetool",
						EnvName:                  "test",
						WorkloadName:             "frontend",
						WorkloadType:             manifest.RequestDrivenWebServiceType,
						Variables:                c.manifest.Variables,
						Tags:                     c.manifest.Tags,
						ServiceDiscoveryEndpoint: mockSD,
						NestedStack: &template.WorkloadNestedStackOpts{
							StackName:       addon.StackName,
							VariableOutputs: []string{"DDBTableName", "Hello"},
							PolicyOutputs:   []string{"AdditionalResourcesPolicyArn"},
						},
						CustomResources:   make(map[string]template.S3ObjectLocation),
						EnableHealthCheck: true,
					}, actual)
					return &template.Content{Buffer: bytes.NewBufferString("template")}, nil
				})
				c.parser = mockParser
				c.addons = addons
			},
			wantedTemplate: "template",
		},
		"should return parsing error": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *RequestDrivenWebService) {
				mockParser := mocks.NewMockrequestDrivenWebSvcReadParser(ctrl)
				addons := mockAddons{}
				mockParser.EXPECT().ParseRequestDrivenWebService(gomock.Any()).Return(nil, errors.New("parsing error"))
				c.parser = mockParser
				c.addons = addons
			},
			wantedError: errors.New("parsing error"),
		},
		"should return error if a custom resource url cannot be parsed when alias is enabled": {
			inManifest: func(manifest manifest.RequestDrivenWebService) manifest.RequestDrivenWebService {
				manifest.Alias = aws.String("convex.domain.com")
				return manifest
			},
			inCustomResourceURLs: map[string]string{
				"CustomDomainFunction": "such-a-weird-url",
			},
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *RequestDrivenWebService) {
				addons := mockAddons{}
				c.wkld.addons = addons
			},
			wantedError: errors.New(`convert custom resource "CustomDomainFunction" url: cannot parse S3 URL such-a-weird-url into bucket name and key`),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mft := *testRDWebServiceManifest
			if tc.inManifest != nil {
				mft = tc.inManifest(mft)
			}
			conf := &RequestDrivenWebService{
				appRunnerWkld: &appRunnerWkld{
					wkld: &wkld{
						name: testServiceName,
						env:  testEnvName,
						app:  testAppName,
						rc: RuntimeConfig{
							Image: &ECRImage{
								RepoURL:  testImageRepoURL,
								ImageTag: testImageTag,
							},
							ServiceDiscoveryEndpoint: mockSD,
							AccountID:                "0123456789012",
							Region:                   "us-west-2",
							CustomResourcesURL:       tc.inCustomResourceURLs,
						},
					},
					healthCheckConfig: mft.HealthCheckConfiguration,
				},
				manifest: &mft,
			}
			tc.mockDependencies(t, ctrl, conf)

			// WHEN
			template, err := conf.Template()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
				require.Equal(t, "", template)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedTemplate, template)
			}
		})
	}
}

func TestRequestDrivenWebService_Parameters(t *testing.T) {
	testCases := map[string]struct {
		imageConfig    manifest.ImageWithPort
		instanceConfig manifest.AppRunnerInstanceConfig

		wantedParams []*cloudformation.Parameter
		wantedError  error
	}{
		"all required fields specified": {
			imageConfig: manifest.ImageWithPort{
				Image: manifest.Image{Location: aws.String("public.ecr.aws/aws-containers/hello-app-runner:latest")},
				Port:  aws.Uint16(80),
			},
			instanceConfig: manifest.AppRunnerInstanceConfig{
				CPU:    aws.Int(1024),
				Memory: aws.Int(1024),
			},
			wantedParams: []*cloudformation.Parameter{{
				ParameterKey:   aws.String("AppName"),
				ParameterValue: aws.String("phonetool"),
			}, {
				ParameterKey:   aws.String("EnvName"),
				ParameterValue: aws.String("test"),
			}, {
				ParameterKey:   aws.String("WorkloadName"),
				ParameterValue: aws.String("frontend"),
			}, {
				ParameterKey:   aws.String("ContainerImage"),
				ParameterValue: aws.String("public.ecr.aws/aws-containers/hello-app-runner:latest"),
			}, {
				ParameterKey:   aws.String("AddonsTemplateURL"),
				ParameterValue: aws.String(""),
			}, {
				ParameterKey:   aws.String(RDWkldImageRepositoryType),
				ParameterValue: aws.String("ECR_PUBLIC"),
			}, {
				ParameterKey:   aws.String(WorkloadContainerPortParamKey),
				ParameterValue: aws.String("80"),
			}, {
				ParameterKey:   aws.String(RDWkldInstanceCPUParamKey),
				ParameterValue: aws.String("1024"),
			}, {
				ParameterKey:   aws.String(RDWkldInstanceMemoryParamKey),
				ParameterValue: aws.String("1024"),
			}},
		},
		"error when port unspecified": {
			imageConfig: manifest.ImageWithPort{
				Image: manifest.Image{Location: aws.String("public.ecr.aws/aws-containers/hello-app-runner:latest")},
			},
			instanceConfig: manifest.AppRunnerInstanceConfig{
				CPU:    aws.Int(1024),
				Memory: aws.Int(1024),
			},
			wantedError: errors.New("field `image.port` is required for Request Driven Web Services"),
		},
		"error when CPU unspecified": {
			imageConfig: manifest.ImageWithPort{
				Port:  aws.Uint16(80),
				Image: manifest.Image{Location: aws.String("public.ecr.aws/aws-containers/hello-app-runner:latest")},
			},
			instanceConfig: manifest.AppRunnerInstanceConfig{
				Memory: aws.Int(1024),
			},
			wantedError: errors.New("field `cpu` is required for Request Driven Web Services"),
		},
		"error when memory unspecified": {
			imageConfig: manifest.ImageWithPort{
				Port:  aws.Uint16(80),
				Image: manifest.Image{Location: aws.String("public.ecr.aws/aws-containers/hello-app-runner:latest")},
			},
			instanceConfig: manifest.AppRunnerInstanceConfig{
				CPU: aws.Int(1024),
			},
			wantedError: errors.New("field `memory` is required for Request Driven Web Services"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			c := &RequestDrivenWebService{
				appRunnerWkld: &appRunnerWkld{
					wkld: &wkld{
						name:  aws.StringValue(testRDWebServiceManifest.Name),
						env:   testEnvName,
						app:   testAppName,
						image: tc.imageConfig.Image,
					},
					instanceConfig: tc.instanceConfig,
					imageConfig:    tc.imageConfig,
				},
				manifest: testRDWebServiceManifest,
			}
			p, err := c.Parameters()
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedParams, p)
			}
		})
	}
}

func TestRequestDrivenWebService_SerializedParameters(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, c *RequestDrivenWebService)

		wantedParams string
		wantedError  error
	}{
		"unavailable template": {
			mockDependencies: func(ctrl *gomock.Controller, c *RequestDrivenWebService) {
				m := mocks.NewMockrequestDrivenWebSvcReadParser(ctrl)
				m.EXPECT().Parse(wkldParamsTemplatePath, gomock.Any(), gomock.Any()).Return(nil, errors.New("serialization error"))
				c.wkld.parser = m
			},
			wantedParams: "",
			wantedError:  errors.New("serialization error"),
		},
		"render params template": {
			mockDependencies: func(ctrl *gomock.Controller, c *RequestDrivenWebService) {
				m := mocks.NewMockrequestDrivenWebSvcReadParser(ctrl)
				m.EXPECT().Parse(wkldParamsTemplatePath, gomock.Any(), gomock.Any()).Return(&template.Content{Buffer: bytes.NewBufferString("params")}, nil)
				c.wkld.parser = m
			},
			wantedParams: "params",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := &RequestDrivenWebService{
				appRunnerWkld: &appRunnerWkld{
					wkld: &wkld{
						name: aws.StringValue(testRDWebServiceManifest.Name),
						env:  testEnvName,
						app:  testAppName,
						rc: RuntimeConfig{
							Image: &ECRImage{
								RepoURL:  testImageRepoURL,
								ImageTag: testImageTag,
							},
						},
					},
					instanceConfig: testRDWebServiceManifest.InstanceConfig,
					imageConfig:    testRDWebServiceManifest.ImageConfig,
				},
				manifest: testRDWebServiceManifest,
			}
			tc.mockDependencies(ctrl, c)

			// WHEN
			params, err := c.SerializedParameters()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedParams, params)
		})
	}
}
