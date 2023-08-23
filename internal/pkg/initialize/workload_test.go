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
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
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

		wantedErr error
	}{
		"writes Scheduled Job manifest, and creates repositories successfully": {
			inJobType:        manifestinfo.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			inSchedule: "@hourly",

			mockWriter: func(m *mocks.MockWorkspace) {
				// workspace root: "/resizer/copilot"
				gomock.InOrder(
					m.EXPECT().Rel("resizer/Dockerfile").Return("../Dockerfile", nil),
					m.EXPECT().Rel("/resizer/copilot/manifest.yml").Return("manifest.yml", nil))
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Return("/resizer/copilot/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().CreateJob(gomock.Any()).
					Do(func(app *config.Workload) {
						require.Equal(t, &config.Workload{
							Name: "resizer",
							App:  "app",
							Type: manifestinfo.ScheduledJobType,
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
		},
		"using existing image": {
			inJobType: manifestinfo.ScheduledJobType,
			inAppName: "app",
			inJobName: "resizer",
			inImage:   "mockImage",

			inSchedule: "@hourly",

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Rel("/resizer/manifest.yml").Return("manifest.yml", nil)
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Do(func(m *manifest.ScheduledJob, _ string) {
					require.Equal(t, *m.Workload.Type, manifestinfo.ScheduledJobType)
					require.Equal(t, *m.ImageConfig.Image.Location, "mockImage")
				}).Return("/resizer/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().CreateJob(gomock.Any()).
					Do(func(app *config.Workload) {
						require.Equal(t, &config.Workload{
							Name: "resizer",
							App:  "app",
							Type: manifestinfo.ScheduledJobType,
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
		},
		"write manifest error": {
			inJobType:        manifestinfo.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			inSchedule: "@hourly",

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Rel("resizer/Dockerfile").Return("Dockerfile", nil)
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Return("/resizer/manifest.yml", errors.New("some error"))
			},
			mockstore: func(m *mocks.MockStore) {},
			wantedErr: errors.New("write job manifest: some error"),
		},
		"app error": {
			inJobType:        manifestinfo.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			inSchedule: "@hourly",
			mockWriter: func(m *mocks.MockWorkspace) {
				// workspace root: "/copilot"
				m.EXPECT().Rel("resizer/Dockerfile").Return("resizer/Dockerfile", nil)
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Return("/copilot/resizer/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().GetApplication(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New("get application app: some error"),
		},
		"add job to app fails": {
			inJobType:        manifestinfo.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "frontend/Dockerfile",

			inSchedule: "@hourly",

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Rel("frontend/Dockerfile").Return("frontend/Dockerfile", nil)
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Return("/resizer/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddJobToApp(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("some error"))
			},
			wantedErr: errors.New("add job resizer to application app: some error"),
		},
		"error saving app": {
			inJobType:        manifestinfo.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			inSchedule: "@hourly",

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().Rel("resizer/Dockerfile").Return("Dockerfile", nil)
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Return("/resizer/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().CreateJob(gomock.Any()).
					Return(fmt.Errorf("oops"))
				m.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{}, nil)
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddJobToApp(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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

			initializer := &WorkloadInitializer{
				Store:    mockstore,
				Ws:       mockWriter,
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
		inAppDomain      string
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
						Type: manifestinfo.LoadBalancedWebServiceType,
					},
				}, nil)
			},

			wantedPath: "/",
		},
		"creates manifest with / as the path when it's the only LBWebService": {
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
		"creates manifest with {service name} as the path if there's another LBWebService": {
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "/Dockerfile",

			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().ListServices("app").Return([]*config.Workload{
					{
						Name: "admin",
						Type: manifestinfo.LoadBalancedWebServiceType,
					},
				}, nil)
			},

			wantedPath: "frontend",
		},
		"creates manifest with root path if the application is initialized with a domain": {
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "/Dockerfile",
			inAppDomain:      "example.com",

			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().ListServices("app").Return([]*config.Workload{
					{
						Name: "admin",
						Type: manifestinfo.LoadBalancedWebServiceType,
					},
				}, nil)
			},

			wantedPath: "/",
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
				Port:      tc.inSvcPort,
				appDomain: &tc.inAppDomain,
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
				require.Equal(t, tc.wantedPath, aws.StringValue(manifest.HTTPOrBool.Main.Path))
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

			// WHEN
			manifest := newRequestDrivenWebServiceManifest(&props)

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

		wantedErr error
	}{
		"writes Load Balanced Web Service manifest, and creates repositories successfully": {
			inSvcType:        manifestinfo.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",
			inSvcPort:        80,

			mockWriter: func(m *mocks.MockWorkspace) {
				// workspace root: "/frontend"
				gomock.InOrder(
					m.EXPECT().Rel("frontend/Dockerfile").Return("Dockerfile", nil),
					m.EXPECT().Rel("/frontend/manifest.yml").Return("manifest.yml", nil))
				m.EXPECT().WriteServiceManifest(gomock.Any(), "frontend").Return("/frontend/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().ListServices("app").Return([]*config.Workload{}, nil)
				m.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Workload) {
						require.Equal(t, &config.Workload{
							Name: "frontend",
							App:  "app",
							Type: manifestinfo.LoadBalancedWebServiceType,
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
		},
		"writes Static Site manifest": {
			inSvcType: manifestinfo.StaticSiteType,
			inAppName: "app",
			inSvcName: "static",

			mockWriter: func(m *mocks.MockWorkspace) {
				// workspace root: "/static"
				gomock.InOrder(
					m.EXPECT().Rel("/static/manifest.yml").Return("manifest.yml", nil))
				m.EXPECT().WriteServiceManifest(gomock.Any(), "static").Return("/static/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Workload) {
						require.Equal(t, &config.Workload{
							Name: "static",
							App:  "app",
							Type: manifestinfo.StaticSiteType,
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
				}, "static", gomock.Any())
			},
		},
		"app error": {
			inSvcType:        manifestinfo.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "frontend/Dockerfile",

			mockWriter: func(m *mocks.MockWorkspace) {
				// workspace root: "/frontend"
				m.EXPECT().Rel("frontend/Dockerfile").Return("Dockerfile", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().GetApplication("app").Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New("get application app: some error"),
		},
		"write manifest error": {
			inSvcType:        manifestinfo.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",
			inSvcPort:        80,

			mockWriter: func(m *mocks.MockWorkspace) {
				// workspace root: "/frontend"
				m.EXPECT().Rel("frontend/Dockerfile").Return("Dockerfile", nil)
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
			inSvcType:        manifestinfo.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "frontend/Dockerfile",

			mockWriter: func(m *mocks.MockWorkspace) {
				// workspace root: "/frontend"
				m.EXPECT().Rel("frontend/Dockerfile").Return("Dockerfile", nil)
				m.EXPECT().WriteServiceManifest(gomock.Any(), "frontend").Return("/frontend/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().ListServices("app").Return([]*config.Workload{}, nil)
				m.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddServiceToApp(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("some error"))
			},
			wantedErr: errors.New("add service frontend to application app: some error"),
		},
		"error saving app": {
			inSvcType:        manifestinfo.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",

			mockWriter: func(m *mocks.MockWorkspace) {
				// workspace root: "/frontend"
				m.EXPECT().Rel("frontend/Dockerfile").Return("Dockerfile", nil)
				m.EXPECT().WriteServiceManifest(gomock.Any(), "frontend").Return("/frontend/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().ListServices("app").Return([]*config.Workload{}, nil)
				m.EXPECT().CreateService(gomock.Any()).
					Return(fmt.Errorf("oops"))
				m.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{}, nil)
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddServiceToApp(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantedErr: fmt.Errorf("saving service frontend: oops"),
		},
		"using existing image": {
			inSvcType: manifestinfo.BackendServiceType,
			inAppName: "app",
			inSvcName: "backend",
			inImage:   "mockImage",
			inSvcPort: 80,

			mockWriter: func(m *mocks.MockWorkspace) {
				// workspace root: "/backend"
				m.EXPECT().Rel("/backend/manifest.yml").Return("manifest.yml", nil)
				m.EXPECT().WriteServiceManifest(gomock.Any(), "backend").
					Do(func(m *manifest.BackendService, _ string) {
						require.Equal(t, *m.Workload.Type, manifestinfo.BackendServiceType)
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
							Type: manifestinfo.BackendServiceType,
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
		},
		"no healthcheck options": {
			inSvcType:        manifestinfo.BackendServiceType,
			inAppName:        "app",
			inSvcName:        "backend",
			inDockerfilePath: "backend/Dockerfile",
			inSvcPort:        80,

			mockWriter: func(m *mocks.MockWorkspace) {
				// workspace root: "/backend"
				gomock.InOrder(
					m.EXPECT().Rel("backend/Dockerfile").Return("Dockerfile", nil),
					m.EXPECT().Rel("/backend/manifest.yml").Return("manifest.yml", nil))
				m.EXPECT().WriteServiceManifest(gomock.Any(), "backend").
					Do(func(m *manifest.BackendService, _ string) {
						require.Equal(t, *m.Workload.Type, manifestinfo.BackendServiceType)
						require.Empty(t, m.ImageConfig.HealthCheck)
					}).Return("/backend/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Workload) {
						require.Equal(t, &config.Workload{
							Name: "backend",
							App:  "app",
							Type: manifestinfo.BackendServiceType,
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
		},
		"default healthcheck options": {
			inSvcType:        manifestinfo.BackendServiceType,
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
				// workspace root: "/backend"
				gomock.InOrder(
					m.EXPECT().Rel("backend/Dockerfile").Return("Dockerfile", nil),
					m.EXPECT().Rel("/backend/manifest.yml").Return("manifest.yml", nil))
				m.EXPECT().WriteServiceManifest(gomock.Any(), "backend").
					Do(func(m *manifest.BackendService, _ string) {
						require.Equal(t, *m.Workload.Type, manifestinfo.BackendServiceType)
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
							Type: manifestinfo.BackendServiceType,
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
		},
		"topic subscriptions enabled": {
			inSvcType:        manifestinfo.WorkerServiceType,
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
				// workspace root: "/worker"
				gomock.InOrder(
					m.EXPECT().Rel("worker/Dockerfile").Return("Dockerfile", nil),
					m.EXPECT().Rel("/worker/manifest.yml").Return("manifest.yml", nil))
				m.EXPECT().WriteServiceManifest(gomock.Any(), "worker").
					Do(func(m *manifest.WorkerService, _ string) {
						require.Equal(t, *m.Workload.Type, manifestinfo.WorkerServiceType)
						require.Empty(t, m.ImageConfig.HealthCheck)
					}).Return("/worker/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Workload) {
						require.Equal(t, &config.Workload{
							Name: "worker",
							App:  "app",
							Type: manifestinfo.WorkerServiceType,
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
		},
		"topic subscriptions enabled with default fifo queue": {
			inSvcType:        manifestinfo.WorkerServiceType,
			inAppName:        "app",
			inSvcName:        "worker",
			inDockerfilePath: "worker/Dockerfile",
			inSvcPort:        80,
			inTopics: []manifest.TopicSubscription{
				{
					Name:    aws.String("theTopic.fifo"),
					Service: aws.String("publisher"),
				},
			},

			mockWriter: func(m *mocks.MockWorkspace) {
				// workspace root: "/worker"
				gomock.InOrder(
					m.EXPECT().Rel("worker/Dockerfile").Return("Dockerfile", nil),
					m.EXPECT().Rel("/worker/manifest.yml").Return("manifest.yml", nil))
				m.EXPECT().WriteServiceManifest(gomock.Any(), "worker").
					Do(func(m *manifest.WorkerService, _ string) {
						require.Equal(t, *m.Workload.Type, manifestinfo.WorkerServiceType)
						require.Equal(t, *m.Subscribe.Queue.FIFO.Enable, true)
						require.Empty(t, m.ImageConfig.HealthCheck)
					}).Return("/worker/manifest.yml", nil)
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Workload) {
						require.Equal(t, &config.Workload{
							Name: "worker",
							App:  "app",
							Type: manifestinfo.WorkerServiceType,
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

func TestWorkloadInitializer_AddWorkloadToApp(t *testing.T) {
	testCases := map[string]struct {
		inWlType  string
		inWlName  string
		inAppName string

		mockstore       func(m *mocks.MockStore)
		mockappDeployer func(m *mocks.MockWorkloadAdder)

		wantedErr error
	}{
		"adds job to app": {
			inWlType:  manifestinfo.ScheduledJobType,
			inAppName: "app",
			inWlName:  "job",

			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name: "app",
				}, nil)
				m.EXPECT().CreateJob(&config.Workload{
					App:  "app",
					Name: "job",
					Type: manifestinfo.ScheduledJobType,
				})
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddJobToApp(&config.Application{
					Name: "app",
				}, "job").Return(nil)
			},
		},
		"adds service to app": {
			inWlType:  manifestinfo.LoadBalancedWebServiceType,
			inAppName: "app",
			inWlName:  "svc",

			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name: "app",
				}, nil)
				m.EXPECT().CreateService(&config.Workload{
					App:  "app",
					Name: "svc",
					Type: manifestinfo.LoadBalancedWebServiceType,
				})
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddServiceToApp(&config.Application{
					Name: "app",
				}, "svc").Return(nil)
			},
		},
		"adds static site to app": {
			inWlType:  manifestinfo.StaticSiteType,
			inAppName: "app",
			inWlName:  "svc",

			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name: "app",
				}, nil)
				m.EXPECT().CreateService(&config.Workload{
					App:  "app",
					Name: "svc",
					Type: manifestinfo.StaticSiteType,
				})
			},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {
				m.EXPECT().AddServiceToApp(&config.Application{
					Name: "app",
				}, "svc", gomock.Any()).Return(nil)
			},
		},
		"error getting app": {
			inWlType:  manifestinfo.LoadBalancedWebServiceType,
			inAppName: "app",
			inWlName:  "svc",

			wantedErr: errors.New("get application app: some error"),
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name: "app",
				}, errors.New("some error"))
				m.EXPECT().CreateService(gomock.Any()).Times(0)
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockStore(ctrl)
			mockappDeployer := mocks.NewMockWorkloadAdder(ctrl)

			if tc.mockstore != nil {
				tc.mockstore(mockstore)
			}
			if tc.mockappDeployer != nil {
				tc.mockappDeployer(mockappDeployer)
			}

			initializer := &WorkloadInitializer{
				Store:    mockstore,
				Deployer: mockappDeployer,
			}

			// WHEN
			err := initializer.AddWorkloadToApp(tc.inAppName, tc.inWlName, tc.inWlType)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
