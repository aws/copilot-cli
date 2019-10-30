// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"os"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
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
				ImageTag: "manual-bf3678c",
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
  ContainerImage: 12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/test/frontend:manual-bf3678c
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

}

func TestLBFargateStackConfig_SerializedParameters(t *testing.T) {

}

func TestLBFargateStackConfig_Tags(t *testing.T) {

}
