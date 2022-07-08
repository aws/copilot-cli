// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ecs

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/ecs/mocks"

	"github.com/aws/aws-sdk-go/aws"
	awsecs "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func Test_RunTaskRequestFromECSService(t *testing.T) {
	var (
		testCluster = "crowded-cluster"
		testService = "good-service"
	)
	testCases := map[string]struct {
		setUpMock func(m *mocks.MockECSServiceDescriber)

		wantedRunTaskRequest *RunTaskRequest
		wantedError          error
	}{
		"success": {
			setUpMock: func(m *mocks.MockECSServiceDescriber) {
				m.EXPECT().Service(testCluster, testService).Return(&ecs.Service{
					TaskDefinition: aws.String("task-def"),
				}, nil)
				m.EXPECT().TaskDefinition("task-def").Return(&ecs.TaskDefinition{
					ExecutionRoleArn: aws.String("execution-role"),
					TaskRoleArn:      aws.String("task-role"),
					ContainerDefinitions: []*awsecs.ContainerDefinition{
						{
							Name:       aws.String("the-one-and-only-one-container"),
							Image:      aws.String("beautiful-image"),
							EntryPoint: aws.StringSlice([]string{"enter", "here"}),
							Command:    aws.StringSlice([]string{"do", "not", "enter", "here"}),
							Environment: []*awsecs.KeyValuePair{
								{
									Name:  aws.String("enter"),
									Value: aws.String("no"),
								},
								{
									Name:  aws.String("kidding"),
									Value: aws.String("yes"),
								},
							},
							Secrets: []*awsecs.Secret{
								{
									Name:      aws.String("truth"),
									ValueFrom: aws.String("go-ask-the-wise"),
								},
							},
						},
					},
				}, nil)
				m.EXPECT().NetworkConfiguration(testCluster, testService).Return(&ecs.NetworkConfiguration{
					AssignPublicIp: "1.2.3.4",
					Subnets:        []string{"sbn-1", "sbn-2"},
					SecurityGroups: []string{"sg-1", "sg-2"},
				}, nil)
			},
			wantedRunTaskRequest: &RunTaskRequest{
				networkConfiguration: ecs.NetworkConfiguration{
					AssignPublicIp: "1.2.3.4",
					Subnets:        []string{"sbn-1", "sbn-2"},
					SecurityGroups: []string{"sg-1", "sg-2"},
				},

				executionRole: "execution-role",
				taskRole:      "task-role",

				containerInfo: containerInfo{
					image:      "beautiful-image",
					entryPoint: []string{"enter", "here"},
					command:    []string{"do", "not", "enter", "here"},
					envVars: map[string]string{
						"enter":   "no",
						"kidding": "yes",
					},
					secrets: map[string]string{
						"truth": "go-ask-the-wise",
					},
				},

				cluster: testCluster,
			},
		},
		"unable to retrieve service": {
			setUpMock: func(m *mocks.MockECSServiceDescriber) {
				m.EXPECT().Service(testCluster, testService).Return(nil, errors.New("some error"))
				m.EXPECT().NetworkConfiguration(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedError: errors.New("retrieve service good-service in cluster crowded-cluster: some error"),
		},
		"unable to retrieve task definition": {
			setUpMock: func(m *mocks.MockECSServiceDescriber) {
				m.EXPECT().Service(testCluster, testService).Return(&ecs.Service{
					TaskDefinition: aws.String("task-def"),
				}, nil)
				m.EXPECT().TaskDefinition("task-def").Return(nil, errors.New("some error"))
				m.EXPECT().NetworkConfiguration(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedError: errors.New("retrieve task definition task-def: some error"),
		},
		"unable to retrieve network configuration": {
			setUpMock: func(m *mocks.MockECSServiceDescriber) {
				m.EXPECT().Service(gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().TaskDefinition(gomock.Any()).AnyTimes()
				m.EXPECT().NetworkConfiguration(testCluster, testService).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("retrieve network configuration for service good-service in cluster crowded-cluster: some error"),
		},
		"error if found more than one container": {
			setUpMock: func(m *mocks.MockECSServiceDescriber) {
				m.EXPECT().Service(testCluster, testService).Return(&ecs.Service{
					TaskDefinition: aws.String("task-def"),
				}, nil)
				m.EXPECT().TaskDefinition("task-def").Return(&ecs.TaskDefinition{
					ExecutionRoleArn: aws.String("execution-role"),
					TaskRoleArn:      aws.String("task-role"),
					ContainerDefinitions: []*awsecs.ContainerDefinition{
						{
							Name: aws.String("the-first-container"),
						},
						{
							Name: aws.String("sad-container"),
						},
					},
				}, nil)
				m.EXPECT().NetworkConfiguration(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedError: errors.New("found more than one container in task definition: task-def"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mocks.NewMockECSServiceDescriber(ctrl)
			tc.setUpMock(m)

			got, err := RunTaskRequestFromECSService(m, testCluster, testService)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedRunTaskRequest, got)
			}
		})
	}
}

func Test_RunTaskRequestFromService(t *testing.T) {
	var (
		testApp = "app"
		testEnv = "env"
		testSvc = "svc"
	)
	testCases := map[string]struct {
		setUpMock func(m *mocks.MockServiceDescriber)

		wantedRunTaskRequest *RunTaskRequest
		wantedError          error
	}{
		"returns RunTaskRequest with service's main container": {
			setUpMock: func(m *mocks.MockServiceDescriber) {
				m.EXPECT().TaskDefinition(testApp, testEnv, testSvc).Return(&ecs.TaskDefinition{
					ExecutionRoleArn: aws.String("execution-role"),
					TaskRoleArn:      aws.String("task-role"),
					ContainerDefinitions: []*awsecs.ContainerDefinition{
						{
							Name:       aws.String(testSvc),
							Image:      aws.String("beautiful-image"),
							EntryPoint: aws.StringSlice([]string{"enter", "here"}),
							Command:    aws.StringSlice([]string{"do", "not", "enter", "here"}),
							Environment: []*awsecs.KeyValuePair{
								{
									Name:  aws.String("enter"),
									Value: aws.String("no"),
								},
								{
									Name:  aws.String("kidding"),
									Value: aws.String("yes"),
								},
							},
							Secrets: []*awsecs.Secret{
								{
									Name:      aws.String("truth"),
									ValueFrom: aws.String("go-ask-the-wise"),
								},
							},
						},
						{
							Name: aws.String("random-container-that-we-do-not-care"),
						},
					},
				}, nil)
				m.EXPECT().NetworkConfiguration(testApp, testEnv, testSvc).Return(&ecs.NetworkConfiguration{
					AssignPublicIp: "1.2.3.4",
					Subnets:        []string{"sbn-1", "sbn-2"},
					SecurityGroups: []string{"sg-1", "sg-2"},
				}, nil)
			},
			wantedRunTaskRequest: &RunTaskRequest{
				networkConfiguration: ecs.NetworkConfiguration{
					AssignPublicIp: "1.2.3.4",
					SecurityGroups: []string{"sg-1", "sg-2"},
				},

				executionRole: "execution-role",
				taskRole:      "task-role",

				appName: testApp,
				envName: testEnv,

				containerInfo: containerInfo{
					image:      "beautiful-image",
					entryPoint: []string{"enter", "here"},
					command:    []string{"do", "not", "enter", "here"},
					envVars: map[string]string{
						"enter":   "no",
						"kidding": "yes",
					},
					secrets: map[string]string{
						"truth": "go-ask-the-wise",
					},
				},
			},
		},
		"unable to retrieve task definition": {
			setUpMock: func(m *mocks.MockServiceDescriber) {
				m.EXPECT().TaskDefinition(testApp, testEnv, testSvc).Return(nil, errors.New("some error"))
				m.EXPECT().NetworkConfiguration(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().ClusterARN(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedError: errors.New("retrieve task definition for service svc: some error"),
		},
		"unable to retrieve network configuration": {
			setUpMock: func(m *mocks.MockServiceDescriber) {
				m.EXPECT().TaskDefinition(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().NetworkConfiguration(testApp, testEnv, testSvc).Return(nil, errors.New("some error"))
				m.EXPECT().ClusterARN(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedError: errors.New("retrieve network configuration for service svc: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mocks.NewMockServiceDescriber(ctrl)
			tc.setUpMock(m)

			got, err := RunTaskRequestFromService(m, testApp, testEnv, testSvc)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedRunTaskRequest, got)
			}
		})
	}
}

func Test_RunTaskRequestFromJob(t *testing.T) {
	var (
		testApp = "test-app"
		testEnv = "test-env"
		testJob = "test-job"
	)
	testCases := map[string]struct {
		setUpMock func(m *mocks.MockJobDescriber)

		wantedRunTaskRequest *RunTaskRequest
		wantedError          error
	}{
		"returns RunTaskRequest with job's main container": {
			setUpMock: func(m *mocks.MockJobDescriber) {
				m.EXPECT().TaskDefinition(testApp, testEnv, testJob).Return(&ecs.TaskDefinition{
					ExecutionRoleArn: aws.String("execution-role"),
					TaskRoleArn:      aws.String("task-role"),
					ContainerDefinitions: []*awsecs.ContainerDefinition{
						{
							Name:       aws.String(testJob),
							Image:      aws.String("beautiful-image"),
							EntryPoint: aws.StringSlice([]string{"enter", "here"}),
							Command:    aws.StringSlice([]string{"do", "not", "enter", "here"}),
							Environment: []*awsecs.KeyValuePair{
								{
									Name:  aws.String("enter"),
									Value: aws.String("no"),
								},
								{
									Name:  aws.String("kidding"),
									Value: aws.String("yes"),
								},
							},
							Secrets: []*awsecs.Secret{
								{
									Name:      aws.String("truth"),
									ValueFrom: aws.String("go-ask-the-wise"),
								},
							},
						},
						{
							Name: aws.String("random-container-that-we-do-not-care"),
						},
					},
				}, nil)
				m.EXPECT().NetworkConfigurationForJob(testApp, testEnv, testJob).Return(&ecs.NetworkConfiguration{
					AssignPublicIp: "1.2.3.4",
					Subnets:        []string{"sbn-1", "sbn-2"},
					SecurityGroups: []string{"sg-1", "sg-2"},
				}, nil)
			},
			wantedRunTaskRequest: &RunTaskRequest{
				networkConfiguration: ecs.NetworkConfiguration{
					AssignPublicIp: "1.2.3.4",
					SecurityGroups: []string{"sg-1", "sg-2"},
				},

				executionRole: "execution-role",
				taskRole:      "task-role",

				appName: testApp,
				envName: testEnv,

				containerInfo: containerInfo{
					image:      "beautiful-image",
					entryPoint: []string{"enter", "here"},
					command:    []string{"do", "not", "enter", "here"},
					envVars: map[string]string{
						"enter":   "no",
						"kidding": "yes",
					},
					secrets: map[string]string{
						"truth": "go-ask-the-wise",
					},
				},
			},
		},
		"unable to retrieve task definition": {
			setUpMock: func(m *mocks.MockJobDescriber) {
				m.EXPECT().TaskDefinition(testApp, testEnv, testJob).Return(nil, errors.New("some error"))
				m.EXPECT().NetworkConfigurationForJob(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().ClusterARN(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedError: errors.New("retrieve task definition for job test-job: some error"),
		},
		"unable to retrieve network configuration": {
			setUpMock: func(m *mocks.MockJobDescriber) {
				m.EXPECT().TaskDefinition(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().NetworkConfigurationForJob(testApp, testEnv, testJob).Return(nil, errors.New("some error"))
				m.EXPECT().ClusterARN(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedError: errors.New("retrieve network configuration for job test-job: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mocks.NewMockJobDescriber(ctrl)
			tc.setUpMock(m)

			got, err := RunTaskRequestFromJob(m, testApp, testEnv, testJob)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedRunTaskRequest, got)
			}
		})
	}
}

func TestRunTaskRequest_CLIString(t *testing.T) {
	var (
		testApp = "test-app"
		testEnv = "test-env"
	)
	testCases := map[string]struct {
		in     RunTaskRequest
		wanted string
	}{
		"generates copilot service cmd with --app and --env and --security-groups": {
			in: RunTaskRequest{
				networkConfiguration: ecs.NetworkConfiguration{
					AssignPublicIp: "1.2.3.4",
					SecurityGroups: []string{"sg-1", "sg-2"},
				},

				executionRole: "execution-role",
				taskRole:      "task-role",

				appName: testApp,
				envName: testEnv,

				containerInfo: containerInfo{
					image:      "beautiful-image",
					entryPoint: []string{"enter", "here"},
					command:    []string{"do", "not", "enter", "here"},
					envVars: map[string]string{
						"enter":   "no",
						"kidding": "yes",
					},
					secrets: map[string]string{
						"truth": "go-ask-the-wise",
					},
				},
			},
			wanted: strings.Join([]string{
				"copilot task run",
				"--execution-role execution-role",
				"--task-role task-role",
				"--image beautiful-image",
				"--entrypoint \"enter here\"",
				"--command \"do not enter here\"",
				`--env-vars 'enter=no,kidding=yes'`,
				`--secrets 'truth=go-ask-the-wise'`,
				"--security-groups sg-1,sg-2",
				fmt.Sprintf("--app %s", testApp),
				fmt.Sprintf("--env %s", testEnv),
			}, " \\\n"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.in.CLIString()
			require.Nil(t, err)
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestRunTaskRequest_fmtStringMapToString(t *testing.T) {
	testCases := map[string]struct {
		in     map[string]string
		wanted string
	}{
		"with internal commas": {
			in:     map[string]string{"a": "1,2,3", "b": "2"},
			wanted: `'"a=1,2,3",b=2'`,
		},
		"with internal quotes": {
			in:     map[string]string{"name": `john "nickname" doe`, "single": "single 'quote'"},
			wanted: `'"name=john ""nickname"" doe",single=single '\''quote'\'''`,
		},
		"with internal equals sign": {
			in:     map[string]string{"a": "4=2+2=4", "b": "b"},
			wanted: `'a=4=2+2=4,b=b'`,
		},
		"with a json env var": {
			in:     map[string]string{"myval": `{"key1":"val1","key2":5}`},
			wanted: `'"myval={""key1"":""val1"",""key2"":5}"'`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := fmtStringMapToString(tc.in)
			require.Nil(t, err)
			require.Equal(t, tc.wanted, got)
		})
	}
}
