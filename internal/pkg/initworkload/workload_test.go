package initworkload

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/initworkload/mocks"
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

		inSchedule string
		inRetries  int
		inTimeout  string

		inPort uint16

		mockWriter      func(m *mocks.MockWorkspace)
		mockstore       func(m *mocks.MockStore)
		mockappDeployer func(m *mocks.MockWorkloadAdder)
		mockProg        func(m *mocks.MockProg)

		wantedErr  error
		wantedPath string
	}{
		"writes Scheduled Job manifest, and creates repositories successfully": {
			inJobType:        manifest.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			inSchedule: "@hourly",

			mockWriter: func(m *mocks.MockWorkspace) {
				m.EXPECT().CopilotDirPath().Return("/resizer/copilot", nil)
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
					require.Equal(t, *m.ImageConfig.Location, "mockImage")
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
				m.EXPECT().CopilotDirPath().Return("/resizer", nil)
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Return("/resizer/manifest.yml", errors.New("some error"))
			},
			mockstore: func(m *mocks.MockStore) {
				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			wantedErr: errors.New("write job manifest: some error"),
		},
		"app error": {
			inJobType:        manifest.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			inSchedule: "@hourly",

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
				m.EXPECT().CopilotDirPath().Return("/resizer", nil)
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
				m.EXPECT().CopilotDirPath().Return("/resizer", nil)
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
		"developer error passing wrong params": {
			mockWriter:      func(m *mocks.MockWorkspace) {},
			mockstore:       func(m *mocks.MockStore) {},
			mockappDeployer: func(m *mocks.MockWorkloadAdder) {},
			mockProg:        func(m *mocks.MockProg) {},

			inJobType:        manifest.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			inPort: 80,

			wantedErr: fmt.Errorf("input properties do not specify a valid job"),
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

			initializer := NewJobInitializer(
				mockstore,
				mockWriter,
				mockProg,
				mockappDeployer,
			)

			initJobProps := &WorkloadProps{
				App:            tc.inAppName,
				Name:           tc.inJobName,
				DockerfilePath: tc.inDockerfilePath,
				Image:          tc.inImage,
				Type:           tc.inJobType,
				Schedule:       tc.inSchedule,
				Retries:        tc.inRetries,
				Timeout:        tc.inTimeout,
				Port:           tc.inPort,
			}

			// WHEN
			_, err := initializer.Job(initJobProps)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
				return
			} else {
				require.NoError(t, err)
			}
		})
	}
}
