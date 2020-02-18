// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addons/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/gobuffalo/packd"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestTemplate(t *testing.T) {
	testCases := map[string]struct {
		appName string

		mockWorkspace func(m *mocks.MockworkspaceService)
		mockBox       func(box *packd.MemoryBox)

		wantTemplate string
		wantErr      error
	}{
		"should return addon template": {
			appName: "my-app",

			mockBox: func(box *packd.MemoryBox) {
				box.AddString(addonsTemplatePath, `Description: Additional resources for application '{{.AppName}}'
Parameters:
{{.AddonContent.Parameters}}
Resources:
{{.AddonContent.Resources}}
Outputs:
{{.AddonContent.Outputs}}`)
			},

			mockWorkspace: func(m *mocks.MockworkspaceService) {
				m.EXPECT().ReadAddonFiles("my-app").Return(&workspace.ReadAddonFilesOutput{
					Outputs:    []string{"outputs"},
					Parameters: []string{"params"},
					Resources:  []string{"resources"},
				}, nil)
			},

			wantTemplate: `Description: Additional resources for application 'my-app'
Parameters:
[params]
Resources:
[resources]
Outputs:
[outputs]`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWorkspace := mocks.NewMockworkspaceService(ctrl)
			tc.mockWorkspace(mockWorkspace)
			box := packd.NewMemoryBox()
			tc.mockBox(box)

			service := Addons{
				appName: tc.appName,
				ws:      mockWorkspace,
				box:     box,
			}

			gotTemplate, gotErr := service.Template()

			if gotErr != nil {
				require.Equal(t, tc.wantErr, gotErr)
			} else {
				require.Equal(t, tc.wantTemplate, gotTemplate)
			}
		})
	}
}
