// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var testScheduledJobManifest = manifest.NewScheduledJob(&manifest.ScheduledJobProps{
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
				addons := mockTemplater{err: &addon.ErrAddonsDirNotExist{}}
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
				addons := mockTemplater{err: &addon.ErrAddonsDirNotExist{}}
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
						Image: &ECRImage{
							ImageTag: testJobImageTag,
							RepoURL:  testJobImageRepoURL,
						},
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

func TestScheduledJob_awsSchedule(t *testing.T) {
	testCases := map[string]struct {
		inputSchedule   string
		wantedSchedule  string
		wantedError     error
		wantedErrorType interface{}
	}{
		"simple rate": {
			inputSchedule:  "@every 1h30m",
			wantedSchedule: "rate(90 minutes)",
		},
		"missing schedule": {
			inputSchedule: "",
			wantedError:   errors.New(`missing required field "schedule" in manifest for job mailer`),
		},
		"one minute rate": {
			inputSchedule:  "@every 1m",
			wantedSchedule: "rate(1 minute)",
		},
		"round to minute if using small units": {
			inputSchedule:  "@every 60000ms",
			wantedSchedule: "rate(1 minute)",
		},
		"malformed rate": {
			inputSchedule:   "@every 402 seconds",
			wantedErrorType: &errScheduleInvalid{},
		},
		"malformed cron": {
			inputSchedule:   "every 4m",
			wantedErrorType: &errScheduleInvalid{},
		},
		"correctly converts predefined schedule": {
			inputSchedule:  "@daily",
			wantedSchedule: "cron(0 0 * * ? *)",
		},
		"unrecognized predefined schedule": {
			inputSchedule:   "@minutely",
			wantedErrorType: &errScheduleInvalid{},
		},
		"correctly converts cron with all asterisks": {
			inputSchedule:  "* * * * *",
			wantedSchedule: "cron(* * * * ? *)",
		},
		"correctly converts cron with one ? in DOW": {
			inputSchedule:  "* * * * ?",
			wantedSchedule: "cron(* * * * ? *)",
		},
		"correctly converts cron with one ? in DOM": {
			inputSchedule:  "* * ? * *",
			wantedSchedule: "cron(* * * * ? *)",
		},
		"correctly convert two ? in DOW and DOM": {
			inputSchedule:  "* * ? * ?",
			wantedSchedule: "cron(* * * * ? *)",
		},
		"correctly converts cron with specified DOW": {
			inputSchedule:  "* * * * MON-FRI",
			wantedSchedule: "cron(* * ? * MON-FRI *)",
		},
		"correctly parse provided ? with DOW": {
			inputSchedule:  "* * ? * MON",
			wantedSchedule: "cron(* * ? * MON *)",
		},
		"correctly parse provided ? with DOM": {
			inputSchedule:  "* * 1 * ?",
			wantedSchedule: "cron(* * 1 * ? *)",
		},
		"correctly converts cron with specified DOM": {
			inputSchedule:  "* * 1 * *",
			wantedSchedule: "cron(* * 1 * ? *)",
		},
		"correctly increments 0-indexed DOW": {
			inputSchedule:  "* * ? * 2-6",
			wantedSchedule: "cron(* * ? * 3-7 *)",
		},
		"zero-indexed DOW with un?ed DOM": {
			inputSchedule:  "* * * * 2-6",
			wantedSchedule: "cron(* * ? * 3-7 *)",
		},
		"returns error if both DOM and DOW specified": {
			inputSchedule: "* * 1 * SUN",
			wantedError:   errors.New("parse cron schedule: cannot specify both DOW and DOM in cron expression"),
		},
		"returns error if fixed interval less than one minute": {
			inputSchedule: "@every -5m",
			wantedError:   errors.New("parse fixed interval: duration must be greater than or equal to 1 minute"),
		},
		"returns error if fixed interval is 0": {
			inputSchedule: "@every 0m",
			wantedError:   errors.New("parse fixed interval: duration must be greater than or equal to 1 minute"),
		},
		"error on non-whole-number of minutes": {
			inputSchedule: "@every 89s",
			wantedError:   errors.New("parse fixed interval: duration must be a whole number of minutes or hours"),
		},
		"error on too many inputs": {
			inputSchedule:   "* * * * * *",
			wantedErrorType: &errScheduleInvalid{},
		},
		"cron syntax error": {
			inputSchedule:   "* * * malformed *",
			wantedErrorType: &errScheduleInvalid{},
		},
		"passthrogh AWS flavored cron": {
			inputSchedule:  "cron(0 * * * ? *)",
			wantedSchedule: "cron(0 * * * ? *)",
		},
		"passthrough AWS flavored rate": {
			inputSchedule:  "rate(5 minutes)",
			wantedSchedule: "rate(5 minutes)",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			job := &ScheduledJob{
				wkld: &wkld{
					name: "mailer",
				},
				manifest: &manifest.ScheduledJob{
					ScheduledJobConfig: manifest.ScheduledJobConfig{
						On: manifest.JobTriggerConfig{
							Schedule: tc.inputSchedule,
						},
					},
				},
			}
			// WHEN
			parsedSchedule, err := job.awsSchedule()

			// THEN
			if tc.wantedErrorType != nil {
				ok := errors.As(err, &tc.wantedErrorType)
				require.True(t, ok)
				require.NotEmpty(t, tc.wantedErrorType)
			} else if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedSchedule, parsedSchedule)
			}
		})
	}
}

func TestScheduledJob_stateMachine(t *testing.T) {
	testCases := map[string]struct {
		inputTimeout    string
		inputRetries    int
		wantedConfig    template.StateMachineOpts
		wantedError     error
		wantedErrorType interface{}
	}{
		"timeout and retries": {
			inputTimeout: "3h",
			inputRetries: 5,
			wantedConfig: template.StateMachineOpts{
				Timeout: aws.Int(10800),
				Retries: aws.Int(5),
			},
		},
		"just timeout": {
			inputTimeout: "1h",
			wantedConfig: template.StateMachineOpts{
				Timeout: aws.Int(3600),
				Retries: nil,
			},
		},
		"just retries": {
			inputRetries: 2,
			wantedConfig: template.StateMachineOpts{
				Timeout: nil,
				Retries: aws.Int(2),
			},
		},
		"negative retries": {
			inputRetries: -4,
			wantedError:  errors.New("number of retries cannot be negative"),
		},
		"timeout too small": {
			inputTimeout: "500ms",
			wantedError:  errors.New("timeout must be greater than or equal to 1 second"),
		},
		"invalid timeout": {
			inputTimeout:    "5 hours",
			wantedErrorType: &errDurationInvalid{},
		},
		"timeout non-integer number of seconds": {
			inputTimeout: "1s40ms",
			wantedError:  errors.New("timeout must be a whole number of seconds, minutes, or hours"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			job := &ScheduledJob{
				wkld: &wkld{
					name: "mailer",
				},
				manifest: &manifest.ScheduledJob{
					ScheduledJobConfig: manifest.ScheduledJobConfig{
						JobFailureHandlerConfig: manifest.JobFailureHandlerConfig{
							Retries: tc.inputRetries,
							Timeout: tc.inputTimeout,
						},
					},
				},
			}
			// WHEN
			parsedStateMachine, err := job.stateMachineOpts()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else if tc.wantedErrorType != nil {
				require.True(t, errors.As(err, tc.wantedErrorType))
			} else {
				require.NoError(t, err)
				require.Equal(t, aws.IntValue(tc.wantedConfig.Retries), aws.IntValue(parsedStateMachine.Retries))
				require.Equal(t, aws.IntValue(tc.wantedConfig.Timeout), aws.IntValue(parsedStateMachine.Timeout))
			}
		})
	}
}

func TestScheduledJob_Parameters(t *testing.T) {
	baseProps := &manifest.ScheduledJobProps{
		WorkloadProps: &manifest.WorkloadProps{
			Name:       "frontend",
			Dockerfile: "frontend/Dockerfile",
		},
		Schedule: "@daily",
	}
	testScheduledJobManifest := manifest.NewScheduledJob(baseProps)
	testScheduledJobManifest.Count = manifest.Count{
		Value: aws.Int(1),
	}
	expectedParams := []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(WorkloadAppNameParamKey),
			ParameterValue: aws.String("phonetool"),
		},
		{
			ParameterKey:   aws.String(WorkloadEnvNameParamKey),
			ParameterValue: aws.String("test"),
		},
		{
			ParameterKey:   aws.String(WorkloadNameParamKey),
			ParameterValue: aws.String("frontend"),
		},
		{
			ParameterKey:   aws.String(WorkloadContainerImageParamKey),
			ParameterValue: aws.String("12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend:manual-bf3678c"),
		},
		{
			ParameterKey:   aws.String(WorkloadTaskCPUParamKey),
			ParameterValue: aws.String("256"),
		},
		{
			ParameterKey:   aws.String(WorkloadTaskMemoryParamKey),
			ParameterValue: aws.String("512"),
		},
		{
			ParameterKey:   aws.String(WorkloadTaskCountParamKey),
			ParameterValue: aws.String("1"),
		},
		{
			ParameterKey:   aws.String(WorkloadLogRetentionParamKey),
			ParameterValue: aws.String("30"),
		},
		{
			ParameterKey:   aws.String(WorkloadAddonsTemplateURLParamKey),
			ParameterValue: aws.String(""),
		},
		{
			ParameterKey:   aws.String(ScheduledJobScheduleParamKey),
			ParameterValue: aws.String("cron(0 0 * * ? *)"),
		},
	}
	testCases := map[string]struct {
		httpsEnabled bool
		manifest     *manifest.ScheduledJob

		expectedParams []*cloudformation.Parameter
		expectedErr    error
	}{
		"renders all parameters": {
			manifest: testScheduledJobManifest,

			expectedParams: expectedParams,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			// GIVEN
			conf := &ScheduledJob{
				wkld: &wkld{
					name: aws.StringValue(tc.manifest.Name),
					env:  testEnvName,
					app:  testAppName,
					tc:   tc.manifest.TaskConfig,
					rc: RuntimeConfig{
						Image: &ECRImage{
							RepoURL:  testImageRepoURL,
							ImageTag: testImageTag,
						},
					},
				},
				manifest: tc.manifest,
			}

			// WHEN
			params, err := conf.Parameters()

			// THEN
			if err == nil {
				require.ElementsMatch(t, tc.expectedParams, params)
			} else {
				require.EqualError(t, tc.expectedErr, err.Error())
			}
		})
	}
}

func TestScheduledJob_SerializedParameters(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, c *ScheduledJob)

		wantedParams string
		wantedError  error
	}{
		"unavailable template": {
			mockDependencies: func(ctrl *gomock.Controller, c *ScheduledJob) {
				m := mocks.NewMockloadBalancedWebSvcReadParser(ctrl)
				m.EXPECT().Parse(wkldParamsTemplatePath, gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
				c.wkld.parser = m
			},
			wantedParams: "",
			wantedError:  errors.New("some error"),
		},
		"render params template": {
			mockDependencies: func(ctrl *gomock.Controller, c *ScheduledJob) {
				m := mocks.NewMockloadBalancedWebSvcReadParser(ctrl)
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
			c := &ScheduledJob{
				wkld: &wkld{
					name: aws.StringValue(testScheduledJobManifest.Name),
					env:  testEnvName,
					app:  testAppName,
					tc:   testScheduledJobManifest.TaskConfig,
					rc: RuntimeConfig{
						Image: &ECRImage{
							RepoURL:  testImageRepoURL,
							ImageTag: testImageTag,
						},
						AdditionalTags: map[string]string{
							"owner": "boss",
						},
					},
				},
				manifest: testScheduledJobManifest,
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
