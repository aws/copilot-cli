package cloudformation

import (
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/mocks"
	"github.com/golang/mock/gomock"
)

func TestCloudFormation_DeployTask(t *testing.T) {
	const (
		stackName     = "copilot-my-task"
		stackTemplate = "my-task template"
	)
	mockTask := &deploy.CreateTaskResourcesInput{
		Name: "my-task",
	}

	testCases := map[string]struct {
		mockCfnClient func(m *mocks.MockcfnClient)
	}{
		"create a new stack": {
			mockCfnClient: func(m *mocks.MockcfnClient) {
				m.EXPECT().CreateAndWait(gomock.Any()).Return(nil)
				m.EXPECT().UpdateAndWait(gomock.Any()).Times(0)
			},
		},
		"update the stack": {
			mockCfnClient: func(m *mocks.MockcfnClient) {
				m.EXPECT().CreateAndWait(gomock.Any()).Return(&cloudformation.ErrStackAlreadyExists{
					Name: "my-task",
				})
				m.EXPECT().UpdateAndWait(gomock.Any()).Times(1).Return(nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCfnClient := mocks.NewMockcfnClient(ctrl)
			if tc.mockCfnClient != nil {
				tc.mockCfnClient(mockCfnClient)
			}

			cf := CloudFormation{
				cfnClient: mockCfnClient,
			}

			err := cf.DeployTask(mockTask)
			require.NoError(t, err)
		})
	}
}
