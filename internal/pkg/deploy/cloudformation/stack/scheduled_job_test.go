// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var testScheduledJobManifest = manifest.NewScheduledJob(manifest.ScheduledJobProps{
	WorkloadProps: &manifest.WorkloadProps{
		Name:       "mailer",
		Dockerfile: "mailer/Dockerfile",
	},
	Schedule: "@daily",
	Timeout:  "1h30m",
	Retries:  3,
})

// mockTemplater is declared in lb_web_svc_test.go
const (
	testJobAppName      = "cuteoverload"
	testJobEnvName      = "test"
	testJobImageRepoURL = "123456789012.dkr.ecr.us-west-2.amazonaws.com/cuteoverload/mailer"
	testJobImageTag     = "stable"
)

func TestScheduledJob_Template(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(t *testing.T, ctrl *gomock.Controller, j *ScheduledJob)

		wantedTemplate string
		wantedError    error
	}{
		"render template without addons successfully": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, j *ScheduledJob) {
				m := mocks.NewMockscheduledJobParser(ctrl)
				m.EXPECT().ParseScheduledJob(gomock.Eq(template.WorkloadOpts{
					ScheduleExpression: "cron(0 0 * * ? *)",
					StateMachine: &template.StateMachineOpts{
						Timeout: aws.Int(5400),
						Retries: aws.Int(3),
					},
				})).Return(&template.Content{Buffer: bytes.NewBufferString("template")}, nil)
				addons := mockTemplater{err: &addon.ErrDirNotExist{}}
				j.parser = m
				j.wkld.addons = addons
			},
			wantedTemplate: "template",
		},
		"render template with addons": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, j *ScheduledJob) {
				m := mocks.NewMockscheduledJobParser(ctrl)
				m.EXPECT().ParseScheduledJob(gomock.Eq(template.WorkloadOpts{
					NestedStack: &template.WorkloadNestedStackOpts{
						StackName:       addon.StackName,
						VariableOutputs: []string{"Hello"},
						SecretOutputs:   []string{"MySecretArn"},
						PolicyOutputs:   []string{"AdditionalResourcesPolicyArn"},
					},
					ScheduleExpression: "cron(0 0 * * ? *)",
					StateMachine: &template.StateMachineOpts{
						Timeout: aws.Int(5400),
						Retries: aws.Int(3),
					},
				})).Return(&template.Content{Buffer: bytes.NewBufferString("template")}, nil)
				addons := mockTemplater{
					tpl: `Resources:
  AdditionalResourcesPolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      PolicyDocument:
        Statement:
        - Effect: Allow
          Action: '*'
          Resource: '*'
  MySecret:
    Type: AWS::SecretsManager::Secret
    Properties:
      Description: 'This is my rds instance secret'
      GenerateSecretString:
        SecretStringTemplate: '{"username": "admin"}'
        GenerateStringKey: 'password'
        PasswordLength: 16
        ExcludeCharacters: '"@/\'
Outputs:
  AdditionalResourcesPolicyArn:
    Value: !Ref AdditionalResourcesPolicy
  MySecretArn:
    Value: !Ref MySecret
  Hello:
    Value: hello`,
				}
				j.parser = m
				j.wkld.addons = addons
			},
			wantedTemplate: "template",
		},
		"error parsing addons": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, j *ScheduledJob) {
				m := mocks.NewMockscheduledJobParser(ctrl)
				addons := mockTemplater{err: errors.New("some error")}
				j.parser = m
				j.wkld.addons = addons
			},
			wantedError: fmt.Errorf("generate addons template for %s: %w", aws.StringValue(testScheduledJobManifest.Name), errors.New("some error")),
		},
		"template parsing error": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, j *ScheduledJob) {
				m := mocks.NewMockscheduledJobParser(ctrl)
				m.EXPECT().ParseScheduledJob(gomock.Any()).Return(nil, errors.New("some error"))
				addons := mockTemplater{err: &addon.ErrDirNotExist{}}
				j.parser = m
				j.wkld.addons = addons
			},
			wantedError: fmt.Errorf("parse scheduled job template: some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			conf := &ScheduledJob{
				wkld: &wkld{
					name: aws.StringValue(testScheduledJobManifest.Name),
					env:  testJobEnvName,
					app:  testJobAppName,
					rc: RuntimeConfig{
						ImageRepoURL: testJobImageRepoURL,
						ImageTag:     testJobImageTag,
					},
				},
				manifest: testScheduledJobManifest,
			}
			tc.mockDependencies(t, ctrl, conf)

			// WHEN
			template, err := conf.Template()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedTemplate, template)
			}
		})
	}
}
