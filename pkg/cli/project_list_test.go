package cli

import (
	"fmt"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"
	"github.com/aws/PRIVATE-amazon-ecs-archer/mocks"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer"
	"github.com/golang/mock/gomock"
	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/require"
)

func TestProjectList_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockProjectStore := mocks.NewMockProjectStore(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		listOpts    ListProjectOpts
		output      func(c *expect.Console) bool
		mocking     func()
		expectedErr error
	}{
		"with projects": {
			output: func(c *expect.Console) bool {
				c.ExpectString("project1")
				c.ExpectString("project2")
				return true
			},
			listOpts: ListProjectOpts{
				manager: mockProjectStore,
			},
			mocking: func() {
				mockProjectStore.
					EXPECT().
					ListProjects().
					Return([]*archer.Project{
						&archer.Project{Name: "project1"},
						&archer.Project{Name: "project2"},
					}, nil)

			},
		},
		"with an error": {
			output: func(c *expect.Console) bool {
				return true
			},
			listOpts: ListProjectOpts{
				manager: mockProjectStore,
			},
			mocking: func() {
				mockProjectStore.
					EXPECT().
					ListProjects().
					Return(nil, fmt.Errorf("error fetching projects"))

			},
			expectedErr: fmt.Errorf("error fetching projects"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			mockTerminal, _, _ := vt10x.NewVT10XConsole()
			// Prepare mocks
			tc.mocking()

			// Set up fake terminal
			tc.listOpts.prompt = terminal.Stdio{
				In:  mockTerminal.Tty(),
				Out: mockTerminal.Tty(),
				Err: mockTerminal.Tty(),
			}

			// WHEN
			if tc.expectedErr != nil {
				err := tc.listOpts.Execute()
				require.Equal(t, tc.expectedErr, err)
			} else {
				err := tc.listOpts.Execute()
				// Write inputs to the terminal
				done := make(chan bool)
				go func() { done <- tc.output(mockTerminal) }()
				require.NoError(t, err)
				require.True(t, <-done, "We should print to the terminal")
			}

			// Cleanup our terminals
			mockTerminal.Tty().Close()
			mockTerminal.Close()
		})
	}
}
