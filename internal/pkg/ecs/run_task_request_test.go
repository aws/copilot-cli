package ecs

import (
	"errors"
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
		setUpMock func(m *mocks.MockecsServiceDescriber)

		wantedRunTaskRequest *RunTaskRequest
		wantedError          error
	}{
		"success": {
			setUpMock: func(m *mocks.MockecsServiceDescriber) {
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
			setUpMock: func(m *mocks.MockecsServiceDescriber) {
				m.EXPECT().Service(testCluster, testService).Return(nil, errors.New("some error"))
				m.EXPECT().NetworkConfiguration(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedError: errors.New("retrieve service good-service in cluster crowded-cluster: some error"),
		},
		"unable to retrieve task definition": {
			setUpMock: func(m *mocks.MockecsServiceDescriber) {
				m.EXPECT().Service(testCluster, testService).Return(&ecs.Service{
					TaskDefinition: aws.String("task-def"),
				}, nil)
				m.EXPECT().TaskDefinition("task-def").Return(nil, errors.New("some error"))
				m.EXPECT().NetworkConfiguration(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedError: errors.New("retrieve task definition task-def: some error"),
		},
		"unable to retrieve network configuration": {
			setUpMock: func(m *mocks.MockecsServiceDescriber) {
				m.EXPECT().Service(gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().TaskDefinition(gomock.Any()).AnyTimes()
				m.EXPECT().NetworkConfiguration(testCluster, testService).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("retrieve network configuration for service good-service in cluster crowded-cluster: some error"),
		},
		"error if found more than one container": {
			setUpMock: func(m *mocks.MockecsServiceDescriber) {
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

			m := mocks.NewMockecsServiceDescriber(ctrl)
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
		setUpMock func(m *mocks.MockserviceDescriber)

		wantedRunTaskRequest *RunTaskRequest
		wantedError          error
	}{
		"returns RunTaskRequest with service's main container": {
			setUpMock: func(m *mocks.MockserviceDescriber) {
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
				m.EXPECT().ClusterARN(testApp, testEnv).Return("kamura-village", nil)
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

				cluster: "kamura-village",
			},
		},
		"unable to retrieve task definition": {
			setUpMock: func(m *mocks.MockserviceDescriber) {
				m.EXPECT().TaskDefinition(testApp, testEnv, testSvc).Return(nil, errors.New("some error"))
				m.EXPECT().NetworkConfiguration(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().ClusterARN(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedError: errors.New("retrieve task definition for service svc: some error"),
		},
		"unable to retrieve network configuration": {
			setUpMock: func(m *mocks.MockserviceDescriber) {
				m.EXPECT().TaskDefinition(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().NetworkConfiguration(testApp, testEnv, testSvc).Return(nil, errors.New("some error"))
				m.EXPECT().ClusterARN(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedError: errors.New("retrieve network configuration for service svc: some error"),
		},
		"unable to obtain cluster ARN": {
			setUpMock: func(m *mocks.MockserviceDescriber) {
				m.EXPECT().TaskDefinition(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().NetworkConfiguration(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().ClusterARN(testApp, testEnv).Return("", errors.New("some error"))
			},
			wantedError: errors.New("retrieve cluster ARN created for environment env in application app: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mocks.NewMockserviceDescriber(ctrl)
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
