// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"errors"
	"os"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/gobuffalo/packd"
	"github.com/stretchr/testify/require"
)

func TestLBFargateStackConfig_StackName(t *testing.T) {
	testCases := map[string]struct {
		inAppName     string
		inEnvName     string
		inProjectName string

		wantedStackName string
	}{
		"valid stack name": {
			inAppName:     "frontend",
			inEnvName:     "test",
			inProjectName: "phonetool",

			wantedStackName: "phonetool-test-frontend",
		},
		"longer than 128 characters": {
			inAppName:     "whatisthishorriblylongapplicationnamethatcantfitintocloudformationwhatarewesupposedtodoaboutthisaaaaaaaaaaaaaaaaaaaa",
			inEnvName:     "test",
			inProjectName: "phonetool",

			wantedStackName: "phonetool-test-whatisthishorriblylongapplicationnamethatcantfitintocloudformationwhatarewesupposedtodoaboutthisaaaaaaaaaaaaaaaaa",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			conf := &LBFargateStackConfig{
				CreateLBFargateAppInput: &deploy.CreateLBFargateAppInput{
					App: &manifest.LBFargateManifest{
						AppManifest: manifest.AppManifest{
							Name: tc.inAppName,
						},
					},
					Env: &archer.Environment{
						Project: tc.inProjectName,
						Name:    tc.inEnvName,
					},
				},
			}

			// WHEN
			n := conf.StackName()

			// THEN
			require.Equal(t, tc.wantedStackName, n, "expected stack names to be equal")
		})
	}
}

func TestLBFargateStackConfig_Template(t *testing.T) {
	testCases := map[string]struct {
		in *deploy.CreateLBFargateAppInput

		mockBox func(box *packd.MemoryBox)

		wantedTemplate string
		wantedError    error
	}{
		"unavailable app template": {
			mockBox: func(box *packd.MemoryBox) {
				box.AddString(lbFargateAppRulePriorityGeneratorPath, "javascript")
			},
			wantedTemplate: "",
			wantedError: &ErrTemplateNotFound{
				templateLocation: lbFargateAppTemplatePath,
				parentErr:        os.ErrNotExist,
			},
		},
		"unavailable custom resource template": {
			mockBox: func(box *packd.MemoryBox) {},

			wantedTemplate: "",
			wantedError: &ErrTemplateNotFound{
				templateLocation: lbFargateAppRulePriorityGeneratorPath,
				parentErr:        os.ErrNotExist,
			},
		},
		"render default template": {
			in: &deploy.CreateLBFargateAppInput{
				App: manifest.NewLoadBalancedFargateManifest("frontend", "frontend/Dockerfile", 80),
				Env: &archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					Region:    "us-west-2",
					AccountID: "12345",
					Prod:      false,
				},
				ImageRepoURL: "12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend",
				ImageTag:     "manual-bf3678c",
			},
			mockBox: func(box *packd.MemoryBox) {
				box.AddString(lbFargateAppRulePriorityGeneratorPath, "javascript")
				box.AddString(lbFargateAppTemplatePath, `Parameters:
  ProjectName: {{.Env.Project}}
  EnvName: {{.Env.Name}}
  AppName: {{.App.Name}}
  ContainerImage: {{.Image.URL}}
  ContainerPort: {{.Image.Port}}
  RulePath: '{{.App.Path}}'
  TaskCPU: '{{.App.CPU}}'
  TaskMemory: '{{.App.Memory}}'
  TaskCount: {{.App.Count}}`)
			},

			wantedTemplate: `Parameters:
  ProjectName: phonetool
  EnvName: test
  AppName: frontend
  ContainerImage: 12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend:manual-bf3678c
  ContainerPort: 80
  RulePath: '*'
  TaskCPU: '512'
  TaskMemory: '1024'
  TaskCount: 1`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			box := packd.NewMemoryBox()
			tc.mockBox(box)

			conf := &LBFargateStackConfig{
				CreateLBFargateAppInput: tc.in,
				box:                     box,
			}

			// WHEN
			template, err := conf.Template()

			// THEN
			require.True(t, errors.Is(err, tc.wantedError), "expected: %v, got: %v", tc.wantedError, err)
			require.Equal(t, tc.wantedTemplate, template)
		})
	}
}

func TestLBFargateStackConfig_Parameters(t *testing.T) {
	testCases := map[string]struct {
		httpsEnabled bool
		expectedHTTP string
	}{
		"HTTPS Enabled": {
			httpsEnabled: true,
			expectedHTTP: "true",
		},
		"HTTPS Not Enabled": {
			httpsEnabled: false,
			expectedHTTP: "false",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			// GIVEN
			conf := &LBFargateStackConfig{
				CreateLBFargateAppInput: &deploy.CreateLBFargateAppInput{
					App: manifest.NewLoadBalancedFargateManifest("frontend", "frontend/Dockerfile", 80),
					Env: &archer.Environment{
						Project:   "phonetool",
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "12345",
						Prod:      false,
					},
					ImageRepoURL: "12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend",
					ImageTag:     "manual-bf3678c",
				},
				httpsEnabled: tc.httpsEnabled,
			}

			// WHEN
			params := conf.Parameters()

			// THEN
			require.Equal(t, []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(LBFargateParamProjectNameKey),
					ParameterValue: aws.String("phonetool"),
				},
				{
					ParameterKey:   aws.String(LBFargateParamEnvNameKey),
					ParameterValue: aws.String("test"),
				},
				{
					ParameterKey:   aws.String(LBFargateParamAppNameKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(LBFargateParamContainerImageKey),
					ParameterValue: aws.String("12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend:manual-bf3678c"),
				},
				{
					ParameterKey:   aws.String(LBFargateParamContainerPortKey),
					ParameterValue: aws.String("80"),
				},
				{
					ParameterKey:   aws.String(LBFargateRulePathKey),
					ParameterValue: aws.String("*"),
				},
				{
					ParameterKey:   aws.String(LBFargateTaskCPUKey),
					ParameterValue: aws.String("512"),
				},
				{
					ParameterKey:   aws.String(LBFargateTaskMemoryKey),
					ParameterValue: aws.String("1024"),
				},
				{
					ParameterKey:   aws.String(LBFargateTaskCountKey),
					ParameterValue: aws.String("1"),
				},
				{
					ParameterKey:   aws.String(LBFargateParamHTTPSKey),
					ParameterValue: aws.String(tc.expectedHTTP),
				},
			}, params)
		})
	}
}

func TestLBFargateStackConfig_SerializedParameters(t *testing.T) {
	testCases := map[string]struct {
		in *LBFargateStackConfig

		mockBox func(box *packd.MemoryBox)

		wantedParams string
		wantedError  error
	}{
		"unavailable template": {
			mockBox:      func(box *packd.MemoryBox) {}, // empty box where template does not exist
			in:           &LBFargateStackConfig{},
			wantedParams: "",
			wantedError: &ErrTemplateNotFound{
				templateLocation: lbFargateAppParamsPath,
				parentErr:        os.ErrNotExist,
			},
		},
		"render params template": {
			in: &LBFargateStackConfig{
				CreateLBFargateAppInput: &deploy.CreateLBFargateAppInput{
					App: manifest.NewLoadBalancedFargateManifest("frontend", "frontend/Dockerfile", 80),
					Env: &archer.Environment{
						Project:   "phonetool",
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "12345",
						Prod:      false,
					},
					ImageRepoURL: "12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend",
					ImageTag:     "manual-bf3678c",
				},
				httpsEnabled: false,
			},

			mockBox: func(box *packd.MemoryBox) {
				box.AddString(lbFargateAppParamsPath, `{
  "Parameters" : {
    "ProjectName" : "{{.Env.Project}}",
    "EnvName": "{{.Env.Name}}",
    "AppName": "{{.App.Name}}",
    "ContainerImage": "{{.Image.URL}}",
    "ContainerPort": "{{.Image.Port}}",
    "RulePath": "{{.App.Path}}",
    "TaskCPU": "{{.App.CPU}}",
    "TaskMemory": "{{.App.Memory}}",
    "TaskCount": "{{.App.Count}}",
    "HTTPSEnabled": "{{.HTTPSEnabled}}"
  }
}`)
			},
			wantedParams: `{
  "Parameters" : {
    "ProjectName" : "phonetool",
    "EnvName": "test",
    "AppName": "frontend",
    "ContainerImage": "12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend:manual-bf3678c",
    "ContainerPort": "80",
    "RulePath": "*",
    "TaskCPU": "512",
    "TaskMemory": "1024",
    "TaskCount": "1",
    "HTTPSEnabled": "false"
  }
}`,
		},
		"render params template for https": {
			in: &LBFargateStackConfig{
				CreateLBFargateAppInput: &deploy.CreateLBFargateAppInput{
					App: manifest.NewLoadBalancedFargateManifest("frontend", "frontend/Dockerfile", 80),
					Env: &archer.Environment{
						Project:   "phonetool",
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "12345",
						Prod:      false,
					},
					ImageRepoURL: "12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend",
					ImageTag:     "manual-bf3678c",
				},
				httpsEnabled: true,
			},
			mockBox: func(box *packd.MemoryBox) {
				box.AddString(lbFargateAppParamsPath, `{
  "Parameters" : {
    "ProjectName" : "{{.Env.Project}}",
    "EnvName": "{{.Env.Name}}",
    "AppName": "{{.App.Name}}",
    "ContainerImage": "{{.Image.URL}}",
    "ContainerPort": "{{.Image.Port}}",
    "RulePath": "{{.App.Path}}",
    "TaskCPU": "{{.App.CPU}}",
    "TaskMemory": "{{.App.Memory}}",
    "TaskCount": "{{.App.Count}}",
    "HTTPSEnabled": "{{.HTTPSEnabled}}"
  }
}`)
			},
			wantedParams: `{
  "Parameters" : {
    "ProjectName" : "phonetool",
    "EnvName": "test",
    "AppName": "frontend",
    "ContainerImage": "12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend:manual-bf3678c",
    "ContainerPort": "80",
    "RulePath": "*",
    "TaskCPU": "512",
    "TaskMemory": "1024",
    "TaskCount": "1",
    "HTTPSEnabled": "true"
  }
}`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			box := packd.NewMemoryBox()
			tc.mockBox(box)
			tc.in.box = box

			// WHEN
			params, err := tc.in.SerializedParameters()

			// THEN
			require.True(t, errors.Is(err, tc.wantedError), "expected: %v, got: %v", tc.wantedError, err)
			require.Equal(t, tc.wantedParams, params)
		})
	}
}

func TestLBFargateStackConfig_Tags(t *testing.T) {
	// GIVEN
	conf := &LBFargateStackConfig{
		CreateLBFargateAppInput: &deploy.CreateLBFargateAppInput{
			App: manifest.NewLoadBalancedFargateManifest("frontend", "frontend/Dockerfile", 80),
			Env: &archer.Environment{
				Project:   "phonetool",
				Name:      "test",
				Region:    "us-west-2",
				AccountID: "12345",
				Prod:      false,
			},
			ImageTag: "manual-bf3678c",
		},
	}

	// WHEN
	tags := conf.Tags()

	// THEN
	require.Equal(t, []*cloudformation.Tag{
		{
			Key:   aws.String(ProjectTagKey),
			Value: aws.String("phonetool"),
		},
		{
			Key:   aws.String(EnvTagKey),
			Value: aws.String("test"),
		},
		{
			Key:   aws.String(AppTagKey),
			Value: aws.String("frontend"),
		},
	}, tags)
}
