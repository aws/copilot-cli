package initworkload

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"
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
		mockWriter       func(m *mocks.MockjobDirManifestWriter)
		mockstore        func(m *mocks.Mockstore)
		mockappDeployer  func(m *mocks.MockappDeployer)
		mockProg         func(m *mocks.Mockprogress)
		mockDf           func(m *mocks.MockdockerfileParser)

		wantedErr error
	}{
		"writes Scheduled Job manifest, and creates repositories successfully": {
			inJobType:        manifest.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			mockWriter: func(m *mocks.MockjobDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/resizer", nil)
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Return("/resizer/manifest.yml", nil)
			},
			mockstore: func(m *mocks.Mockstore) {
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
			mockappDeployer: func(m *mocks.MockappDeployer) {
				m.EXPECT().AddJobToApp(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, "resizer")
			},
			mockProg: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddJobToAppStart, "resizer"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddJobToAppComplete, "resizer"))
			},
		},
		"using existing image": {
			inJobType: manifest.ScheduledJobType,
			inAppName: "app",
			inJobName: "resizer",
			inImage:   "mockImage",

			mockWriter: func(m *mocks.MockjobDirManifestWriter) {
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Do(func(m *manifest.ScheduledJob, _ string) {
					require.Equal(t, *m.Workload.Type, manifest.ScheduledJobType)
					require.Equal(t, *m.ImageConfig.Location, "mockImage")
				}).Return("/resizer/manifest.yml", nil)
			},
			mockstore: func(m *mocks.Mockstore) {
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
			mockappDeployer: func(m *mocks.MockappDeployer) {
				m.EXPECT().AddJobToApp(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, "resizer")
			},
			mockProg: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddJobToAppStart, "resizer"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddJobToAppComplete, "resizer"))
			},
		},
		"write manifest error": {
			inJobType:        manifest.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			mockWriter: func(m *mocks.MockjobDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/resizer", nil)
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Return("/resizer/manifest.yml", errors.New("some error"))
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			wantedErr: errors.New("some error"),
		},
		"app error": {
			inJobType:        manifest.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New("get application app: some error"),
		},
		"add job to app fails": {
			inJobType:        manifest.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "frontend/Dockerfile",

			mockWriter: func(m *mocks.MockjobDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/resizer", nil)
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Return("/resizer/manifest.yml", nil)
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockProg: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddJobToAppStart, "resizer"))
				m.EXPECT().Stop(log.Serrorf(fmtAddJobToAppFailed, "resizer"))
			},
			mockappDeployer: func(m *mocks.MockappDeployer) {
				m.EXPECT().AddJobToApp(gomock.Any(), gomock.Any()).Return(errors.New("some error"))
			},
			wantedErr: errors.New("add job resizer to application app: some error"),
		},
		"error saving app": {
			inJobType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			mockWriter: func(m *mocks.MockjobDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/resizer", nil)
				m.EXPECT().WriteJobManifest(gomock.Any(), "resizer").Return("/resizer/manifest.yml", nil)
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().CreateJob(gomock.Any()).
					Return(fmt.Errorf("oops"))
				m.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{}, nil)
			},
			mockappDeployer: func(m *mocks.MockappDeployer) {
				m.EXPECT().AddJobToApp(gomock.Any(), gomock.Any()).Return(nil)
			},
			mockProg: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(gomock.Any())
				m.EXPECT().Stop(gomock.Any())
			},
			wantedErr: fmt.Errorf("saving job resizer: oops"),
		},
	}

	for name, tc := range testCases {
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
		opts := initJobOpts{
			initWkldVars: initWkldVars{
				appName:        tc.inAppName,
				name:           tc.inJobName,
				dockerfilePath: tc.inDockerfilePath,
				image:          tc.inImage,
				wkldType:       tc.inJobType,
			},
			ws:          mockWriter,
			store:       mockstore,
			appDeployer: mockappDeployer,
			prog:        mockProg,
		}
	}
}
