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

			wantedStackName: "phonetool-test-frontend-app",
		},
		"longer than 128 characters": {
			inAppName:     "whatisthishorriblylongapplicationnamethatcantfitintocloudformationwhatarewesupposedtodoaboutthisaaaaaaaaaaaaaaaaaaaa",
			inEnvName:     "test",
			inProjectName: "phonetool",

			wantedStackName: "ol-test-whatisthishorriblylongapplicationnamethatcantfitintocloudformationwhatarewesupposedtodoaboutthisaaaaaaaaaaaaaaaaaaaa-app",
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
		"unavailable template": {
			mockBox: func(box *packd.MemoryBox) {}, // empty box where template does not exist

			wantedTemplate: "",
			wantedError: &ErrTemplateNotFound{
				templateLocation: lbFargateAppTemplatePath,
				parentErr:        os.ErrNotExist,
			},
		},
		"render default template": {
			in: &deploy.CreateLBFargateAppInput{
				App: manifest.NewLoadBalancedFargateManifest("frontend", "frontend/Dockerfile"),
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
				box.AddString(lbFargateAppTemplatePath, `Parameters:
  ProjectName: {{.Env.Project}}
  EnvName: {{.Env.Name}}
  AppName: {{.App.Name}}
  ContainerImage: {{.Image.URL}}
  ContainerPort: {{.Image.Port}}
  RulePriority: {{.Priority}}
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
  RulePriority: 1
  RulePath: '*'
  TaskCPU: '256'
  TaskMemory: '512'
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
	// GIVEN
	conf := &LBFargateStackConfig{
		CreateLBFargateAppInput: &deploy.CreateLBFargateAppInput{
			App: manifest.NewLoadBalancedFargateManifest("frontend", "frontend/Dockerfile"),
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
	}

	// WHEN
	params := conf.Parameters()

	// THEN
	require.Equal(t, []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(lbFargateParamProjectNameKey),
			ParameterValue: aws.String("phonetool"),
		},
		{
			ParameterKey:   aws.String(lbFargateParamEnvNameKey),
			ParameterValue: aws.String("test"),
		},
		{
			ParameterKey:   aws.String(lbFargateParamAppNameKey),
			ParameterValue: aws.String("frontend"),
		},
		{
			ParameterKey:   aws.String(lbFargateParamContainerImageKey),
			ParameterValue: aws.String("12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend:manual-bf3678c"),
		},
		{
			ParameterKey:   aws.String(lbFargateParamContainerPortKey),
			ParameterValue: aws.String("80"),
		},
		{
			ParameterKey:   aws.String(lbFargateRulePriorityKey),
			ParameterValue: aws.String("1"),
		},
		{
			ParameterKey:   aws.String(lbFargateRulePathKey),
			ParameterValue: aws.String("*"),
		},
		{
			ParameterKey:   aws.String(lbFargateTaskCPUKey),
			ParameterValue: aws.String("256"),
		},
		{
			ParameterKey:   aws.String(lbFargateTaskMemoryKey),
			ParameterValue: aws.String("512"),
		},
		{
			ParameterKey:   aws.String(lbFargateTaskCountKey),
			ParameterValue: aws.String("1"),
		},
	}, params)

}

func TestLBFargateStackConfig_SerializedParameters(t *testing.T) {
	testCases := map[string]struct {
		in *deploy.CreateLBFargateAppInput

		mockBox func(box *packd.MemoryBox)

		wantedParams string
		wantedError  error
	}{
		"unavailable template": {
			mockBox: func(box *packd.MemoryBox) {}, // empty box where template does not exist

			wantedParams: "",
			wantedError: &ErrTemplateNotFound{
				templateLocation: lbFargateAppParamsPath,
				parentErr:        os.ErrNotExist,
			},
		},
		"render params template": {
			in: &deploy.CreateLBFargateAppInput{
				App: manifest.NewLoadBalancedFargateManifest("frontend", "frontend/Dockerfile"),
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
				box.AddString(lbFargateAppParamsPath, `{
  "Parameters" : {
    "ProjectName" : "{{.Env.Project}}",
    "EnvName": "{{.Env.Name}}",
    "AppName": "{{.App.Name}}",
    "ContainerImage": "{{.Image.URL}}",
    "ContainerPort": "{{.Image.Port}}",
    "RulePriority": "{{.Priority}}",
    "RulePath": "{{.App.Path}}",
    "TaskCPU": "{{.App.CPU}}",
    "TaskMemory": "{{.App.Memory}}",
    "TaskCount": "{{.App.Count}}"
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
    "RulePriority": "1",
    "RulePath": "*",
    "TaskCPU": "256",
    "TaskMemory": "512",
    "TaskCount": "1"
  }
}`,
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
			params, err := conf.SerializedParameters()

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
			App: manifest.NewLoadBalancedFargateManifest("frontend", "frontend/Dockerfile"),
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
