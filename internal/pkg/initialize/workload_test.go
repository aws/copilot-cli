// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package initialize

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/initialize/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestWorkloadInitializer_Job(t *testing.T) {
	testCases := map[string]struct {
		inJobType        string
		inJobName        string
		inDockerfilePath string
		inImage          string
		inAppName        string
		inPlatform       manifest.PlatformArgsOrString

		inSchedule string
		inRetries  int
		inTimeout  string

		mockWriter      func(m *mocks.MockWorkspace)
		mockstore       func(m *mocks.MockStore)
		mockappDeployer func(m *mocks.MockWorkloadAdder)
		mockProg        func(m *mocks.MockProg)

		wantedErr error
	}{
		"writes Scheduled Job manifest, and creates repositories successfully": {
			inJobType:        manifest.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			inSchedule: "@hourly",

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Path().Return("/resizer/copilot", nil)
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Return("/resizer/copilot/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().CreateJob(gomock.Any()).
					Do(func(app *config.Workload) {
						require.Equal(t, &config.Workload{
							Name: "resizer",
							App:  "app",
							Type: manifest.ScheduledJobType,
						}, app)
					}).
					Return(nil)
				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddJobToApp(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, "resizer")
			},
			mockProg: func(m *mocks.MockProg) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddWlToAppStart, "job", "resizer"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddWlToAppComplete, "job", "resizer"))
			},
		},
		"using existing image": {
			inJobType: manifest.ScheduledJobType,
			inAppName: "app",
			inJobName: "resizer",
			inImage:   "mockImage",

			inSchedule: "@hourly",

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Do(func(m *manifest.ScheduledJob, _ string) {
					require.Equal(t, *m.Workload.Type, manifest.ScheduledJobType)
					require.Equal(t, *m.ImageConfig.Image.Location, "mockImage")
				}).Return("/resizer/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().CreateJob(gomock.Any()).
					Do(func(app *config.Workload) {
						require.Equal(t, &config.Workload{
							Name: "resizer",
							App:  "app",
							Type: manifest.ScheduledJobType,
						}, app)
					}).
					Return(nil)
				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddJobToApp(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, "resizer")
			},
			mockProg: func(m *mocks.MockProg) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddWlToAppStart, "job", "resizer"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddWlToAppComplete, "job", "resizer"))
			},
		},
		"write manifest error": {
			inJobType:        manifest.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			inSchedule: "@hourly",

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Path().Return("/resizer", nil)
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Return("/resizer/manifest.yml", errors.New("some error"))
			},
			mockstore: func(m *mocks.MockStore) {},
			wantedErr: errors.New("write job manifest: some error"),
		},
		"app error": {
			inJobType:        manifest.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			inSchedule: "@hourly",
			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Path().Return("/copilot", nil)
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Return("/copilot/resizer/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().GetApplication(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New("get application app: some error"),
		},
		"add job to app fails": {
			inJobType:        manifest.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "frontend/Dockerfile",

			inSchedule: "@hourly",

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Path().Return("/resizer", nil)
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Return("/resizer/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockProg: func(m *mocks.MockProg) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddWlToAppStart, "job", "resizer"))
				m.EXPECT().Stop(log.Serrorf(fmtAddWlToAppFailed, "job", "resizer"))
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddJobToApp(gomock.Any(), gomock.Any()).Return(errors.New("some error"))
			},
			wantedErr: errors.New("add job resizer to application app: some error"),
		},
		"error saving app": {
			inJobType:        manifest.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			inSchedule: "@hourly",

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Path().Return("/resizer", nil)
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Return("/resizer/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().CreateJob(gomock.Any()).
					Return(fmt.Errorf("oops"))
				m.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{}, nil)
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddJobToApp(gomock.Any(), gomock.Any()).Return(nil)
			},
			mockProg: func(m *mocks.MockProg) {
				m.EXPECT().Start(gomock.Any())
				m.EXPECT().Stop(gomock.Any())
			},
			wantedErr: fmt.Errorf("saving job resizer: oops"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWriter := mocks.NewMockWorkspace(ctrl)
			mockstore := mocks.NewMockStore(ctrl)
			mockProg := mocks.NewMockProg(ctrl)
			mockappDeployer := mocks.NewMockWorkloadAdder(ctrl)
			if tc.mockWriter != nil {
				tc.mockWriter(mockWriter)
			}
			if tc.mockstore != nil {
				tc.mockstore(mockstore)
			}
			if tc.mockappDeployer != nil {
				tc.mockappDeployer(mockappDeployer)
			}
			if tc.mockProg != nil {
				tc.mockProg(mockProg)
			}

			initializer := &WorkloadInitializer{
				Store:    mockstore,
				Ws:       mockWriter,
				Prog:     mockProg,
				Deployer: mockappDeployer,
			}

			initJobProps := &JobProps{
				WorkloadProps: WorkloadProps{
					App:            tc.inAppName,
					Name:           tc.inJobName,
					DockerfilePath: tc.inDockerfilePath,
					Image:          tc.inImage,
					Type:           tc.inJobType,
					Platform:       tc.inPlatform,
				},
				Schedule: tc.inSchedule,
				Retries:  tc.inRetries,
				Timeout:  tc.inTimeout,
			}

			// WHEN
			_, err := initializer.Job(initJobProps)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAppInitOpts_createLoadBalancedAppManifest(t *testing.T) {
	testCases := map[string]struct {
		inSvcPort        uint16
		inSvcName        string
		inDockerfilePath string
		inAppName        string
		mockstore        func(m *mocks.MockStore)

		wantedErr  error
		wantedPath string
	}{
		"creates manifest with / as the path when there are no other apps": {
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "/Dockerfile",

			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().ListServices("app").Return([]*config.Workload{}, nil)
			},

			wantedPath: "/",
		},
		"creates manifest with / as the path when it's the only app": {
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "/Dockerfile",

			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().ListServices("app").Return([]*config.Workload{
					{
						Name: "frontend",
						Type: manifest.LoadBalancedWebServiceType,
					},
				}, nil)
			},

			wantedPath: "/",
		},
		"creates manifest with / as the path when it's the only LBWebApp": {
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "/Dockerfile",

			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().ListServices("app").Return([]*config.Workload{
					{
						Name: "another-app",
						Type: "backend",
					},
				}, nil)
			},

			wantedPath: "/",
		},
		"creates manifest with {app name} as the path if there's another LBWebApp": {
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "/Dockerfile",

			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().ListServices("app").Return([]*config.Workload{
					{
						Name: "another-app",
						Type: manifest.LoadBalancedWebServiceType,
					},
				}, nil)
			},

			wantedPath: "frontend",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockStore(ctrl)
			if tc.mockstore != nil {
				tc.mockstore(mockstore)
			}

			props := ServiceProps{
				WorkloadProps: WorkloadProps{
					Name:           tc.inSvcName,
					App:            tc.inAppName,
					DockerfilePath: tc.inDockerfilePath,
				},
				Port: tc.inSvcPort,
			}

			initter := &WorkloadInitializer{
				Store: mockstore,
			}

			// WHEN
			manifest, err := initter.newLoadBalancedWebServiceManifest(&props)

			// THEN
			if tc.wantedErr == nil {
				require.NoError(t, err)
				require.Equal(t, tc.inSvcName, aws.StringValue(manifest.Workload.Name))
				require.Equal(t, tc.inSvcPort, aws.Uint16Value(manifest.ImageConfig.Port))
				require.Contains(t, tc.inDockerfilePath, aws.StringValue(manifest.ImageConfig.Image.Build.BuildArgs.Dockerfile))
				require.Equal(t, tc.wantedPath, aws.StringValue(manifest.Path))
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}

func TestAppInitOpts_createRequestDrivenWebServiceManifest(t *testing.T) {
	testCases := map[string]struct {
		inSvcPort        uint16
		inSvcName        string
		inDockerfilePath string
		inImage          string
		inAppName        string

		wantedErr error
	}{
		"creates manifest with dockerfile path": {
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "/Dockerfile",
		},
		"creates manifest with container image": {
			inAppName: "app",
			inSvcName: "frontend",
			inSvcPort: 80,
			inImage:   "111111111111.dkr.ecr.us-east-1.amazonaws.com/app/frontend",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			props := ServiceProps{
				WorkloadProps: WorkloadProps{
					Name:           tc.inSvcName,
					App:            tc.inAppName,
					DockerfilePath: tc.inDockerfilePath,
					Image:          tc.inImage,
				},
				Port: tc.inSvcPort,
			}

			initter := &WorkloadInitializer{}

			// WHEN
			manifest := initter.newRequestDrivenWebServiceManifest(&props)

			// THEN
			require.Equal(t, tc.inSvcName, *manifest.Name)
			require.Equal(t, tc.inSvcPort, *manifest.ImageConfig.Port)
			if tc.inImage != "" {
				require.Equal(t, tc.inImage, *manifest.ImageConfig.Image.Location)
			}
			if tc.inDockerfilePath != "" {
				require.Equal(t, tc.inDockerfilePath, *manifest.ImageConfig.Image.Build.BuildArgs.Dockerfile)
			}
		})
	}
}

func TestWorkloadInitializer_Service(t *testing.T) {
	var (
		testInterval    = 10 * time.Second
		testRetries     = 2
		testTimeout     = 5 * time.Second
		testStartPeriod = 0 * time.Second
	)

	testCases := map[string]struct {
		inSvcPort        uint16
		inSvcType        string
		inSvcName        string
		inDockerfilePath string
		inAppName        string
		inImage          string
		inHealthCheck    manifest.ContainerHealthCheck
		inTopics         []manifest.TopicSubscription

		mockWriter      func(m *mocks.MockWorkspace)
		mockstore       func(m *mocks.MockStore)
		mockappDeployer func(m *mocks.MockWorkloadAdder)
		mockProg        func(m *mocks.MockProg)

		wantedErr error
	}{
		"writes Load Balanced Web Service manifest, and creates repositories successfully": {
			inSvcType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",
			inSvcPort:        80,

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Path().Return("/frontend", nil)
				m.EXPECT().WriteServiceManifest(gomock.Any(), "frontend").Return("/frontend/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().ListServices("app").Return([]*config.Workload{}, nil)
				m.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Workload) {
						require.Equal(t, &config.Workload{
							Name: "frontend",
							App:  "app",
							Type: manifest.LoadBalancedWebServiceType,
						}, app)
					}).
					Return(nil)
				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddServiceToApp(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, "frontend")
			},
			mockProg: func(m *mocks.MockProg) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddWlToAppStart, "service", "frontend"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddWlToAppComplete, "service", "frontend"))
			},
		},
		"app error": {
			inSvcType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "frontend/Dockerfile",

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Path().Return("/frontend", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().GetApplication("app").Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New("get application app: some error"),
		},
		"write manifest error": {
			inSvcType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",
			inSvcPort:        80,

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Path().Return("/frontend", nil)
				m.EXPECT().WriteServiceManifest(gomock.Any(), "frontend").Return("/frontend/manifest.yml", errors.New("some error"))
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().ListServices("app")
				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			wantedErr: errors.New("write service manifest: some error"),
		},
		"add service to app fails": {
			inSvcType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "frontend/Dockerfile",

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Path().Return("/frontend", nil)
				m.EXPECT().WriteServiceManifest(gomock.Any(), "frontend").Return("/frontend/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().ListServices("app").Return([]*config.Workload{}, nil)
				m.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockProg: func(m *mocks.MockProg) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddWlToAppStart, "service", "frontend"))
				m.EXPECT().Stop(log.Serrorf(fmtAddWlToAppFailed, "service", "frontend"))
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddServiceToApp(gomock.Any(), gomock.Any()).Return(errors.New("some error"))
			},
			wantedErr: errors.New("add service frontend to application app: some error"),
		},
		"error saving app": {
			inSvcType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Path().Return("/frontend", nil)
				m.EXPECT().WriteServiceManifest(gomock.Any(), "frontend").Return("/frontend/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().ListServices("app").Return([]*config.Workload{}, nil)
				m.EXPECT().CreateService(gomock.Any()).
					Return(fmt.Errorf("oops"))
				m.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{}, nil)
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddServiceToApp(gomock.Any(), gomock.Any()).Return(nil)
			},
			mockProg: func(m *mocks.MockProg) {
				m.EXPECT().Start(gomock.Any())
				m.EXPECT().Stop(gomock.Any())
			},
			wantedErr: fmt.Errorf("saving service frontend: oops"),
		},
		"using existing image": {
			inSvcType: manifest.BackendServiceType,
			inAppName: "app",
			inSvcName: "backend",
			inImage:   "mockImage",
			inSvcPort: 80,

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().WriteServiceManifest(gomock.Any(), "backend").
					Do(func(m *manifest.BackendService, _ string) {
						require.Equal(t, *m.Workload.Type, manifest.BackendServiceType)
						require.Equal(t, *m.ImageConfig.Image.Location, "mockImage")
						require.Empty(t, m.ImageConfig.HealthCheck)
					}).Return("/backend/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Workload) {
						require.Equal(t, &config.Workload{
							Name: "backend",
							App:  "app",
							Type: manifest.BackendServiceType,
						}, app)
					}).
					Return(nil)

				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddServiceToApp(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, "backend")
			},
			mockProg: func(m *mocks.MockProg) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddWlToAppStart, "service", "backend"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddWlToAppComplete, "service", "backend"))
			},
		},
		"no healthcheck options": {
			inSvcType:        manifest.BackendServiceType,
			inAppName:        "app",
			inSvcName:        "backend",
			inDockerfilePath: "backend/Dockerfile",
			inSvcPort:        80,

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Path().Return("/backend", nil)
				m.EXPECT().WriteServiceManifest(gomock.Any(), "backend").
					Do(func(m *manifest.BackendService, _ string) {
						require.Equal(t, *m.Workload.Type, manifest.BackendServiceType)
						require.Empty(t, m.ImageConfig.HealthCheck)
					}).Return("/backend/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Workload) {
						require.Equal(t, &config.Workload{
							Name: "backend",
							App:  "app",
							Type: manifest.BackendServiceType,
						}, app)
					}).
					Return(nil)

				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddServiceToApp(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, "backend")
			},
			mockProg: func(m *mocks.MockProg) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddWlToAppStart, "service", "backend"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddWlToAppComplete, "service", "backend"))
			},
		},
		"default healthcheck options": {
			inSvcType:        manifest.BackendServiceType,
			inAppName:        "app",
			inSvcName:        "backend",
			inDockerfilePath: "backend/Dockerfile",
			inSvcPort:        80,
			inHealthCheck: manifest.ContainerHealthCheck{
				Interval:    &testInterval,
				Retries:     &testRetries,
				Timeout:     &testTimeout,
				StartPeriod: &testStartPeriod,
				Command:     []string{"CMD curl -f http://localhost/ || exit 1"},
			},

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Path().Return("/backend", nil)
				m.EXPECT().WriteServiceManifest(gomock.Any(), "backend").
					Do(func(m *manifest.BackendService, _ string) {
						require.Equal(t, *m.Workload.Type, manifest.BackendServiceType)
						require.Equal(t, m.ImageConfig.HealthCheck, manifest.ContainerHealthCheck{
							Interval:    &testInterval,
							Retries:     &testRetries,
							Timeout:     &testTimeout,
							StartPeriod: &testStartPeriod,
							Command:     []string{"CMD curl -f http://localhost/ || exit 1"}})
					}).Return("/backend/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Workload) {
						require.Equal(t, &config.Workload{
							Name: "backend",
							App:  "app",
							Type: manifest.BackendServiceType,
						}, app)
					}).
					Return(nil)
				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddServiceToApp(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, "backend")
			},
			mockProg: func(m *mocks.MockProg) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddWlToAppStart, "service", "backend"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddWlToAppComplete, "service", "backend"))
			},
		},
		"topic subscriptions enabled": {
			inSvcType:        manifest.WorkerServiceType,
			inAppName:        "app",
			inSvcName:        "worker",
			inDockerfilePath: "worker/Dockerfile",
			inSvcPort:        80,
			inTopics: []manifest.TopicSubscription{
				{
					Name:    aws.String("theTopic"),
					Service: aws.String("publisher"),
				},
			},

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Path().Return("/worker", nil)
				m.EXPECT().WriteServiceManifest(gomock.Any(), "worker").
					Do(func(m *manifest.WorkerService, _ string) {
						require.Equal(t, *m.Workload.Type, manifest.WorkerServiceType)
						require.Empty(t, m.ImageConfig.HealthCheck)
					}).Return("/worker/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Workload) {
						require.Equal(t, &config.Workload{
							Name: "worker",
							App:  "app",
							Type: manifest.WorkerServiceType,
						}, app)
					}).
					Return(nil)

				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddServiceToApp(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, "worker")
			},
			mockProg: func(m *mocks.MockProg) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddWlToAppStart, "service", "worker"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddWlToAppComplete, "service", "worker"))
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWriter := mocks.NewMockWorkspace(ctrl)
			mockstore := mocks.NewMockStore(ctrl)
			mockappDeployer := mocks.NewMockWorkloadAdder(ctrl)
			mockProg := mocks.NewMockProg(ctrl)

			if tc.mockWriter != nil {
				tc.mockWriter(mockWriter)
			}
			if tc.mockstore != nil {
				tc.mockstore(mockstore)
			}
			if tc.mockappDeployer != nil {
				tc.mockappDeployer(mockappDeployer)
			}
			if tc.mockProg != nil {
				tc.mockProg(mockProg)
			}

			initializer := &WorkloadInitializer{
				Store:    mockstore,
				Ws:       mockWriter,
				Prog:     mockProg,
				Deployer: mockappDeployer,
			}

			// WHEN
			_, err := initializer.Service(&ServiceProps{
				WorkloadProps: WorkloadProps{
					App:            tc.inAppName,
					Name:           tc.inSvcName,
					Type:           tc.inSvcType,
					DockerfilePath: tc.inDockerfilePath,
					Image:          tc.inImage,
					Topics:         tc.inTopics,
				},
				Port:        tc.inSvcPort,
				HealthCheck: tc.inHealthCheck,
			})

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
