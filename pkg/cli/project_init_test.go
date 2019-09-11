package cli

import (
	"testing"

	"github.com/aws/PRIVATE-amazon-ecs-archer/mocks"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestProjectInit_Init(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockProjectStore := mocks.NewMockProjectStore(ctrl)
	var capturedArgument *archer.Project
	defer ctrl.Finish()

	testCases := map[string]struct {
		initProjectOpts InitProjectOpts
		expectedProject archer.Project
		mocking         func()
	}{
		"with a succesful call to add env": {
			initProjectOpts: InitProjectOpts{
				ProjectName: "project",
				manager:     mockProjectStore,
			},
			expectedProject: archer.Project{
				Name:    "project",
				Version: "1.0",
			},
			mocking: func() {
				mockProjectStore.
					EXPECT().
					CreateProject(gomock.Any()).
					Do(func(project *archer.Project) {
						capturedArgument = project
					})

			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup mocks
			tc.mocking()

			tc.initProjectOpts.CreateProject()

			require.Equal(t, tc.expectedProject, *capturedArgument)
		})
	}
}
