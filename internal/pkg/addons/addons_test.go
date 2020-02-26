// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"bytes"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addons/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	templatemocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/template/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestAddons_Template(t *testing.T) {
	testCases := map[string]struct {
		appName          string
		mockDependencies func(ctrl *gomock.Controller, a *Addons)

		wantedTemplate string
		wantedErr      error
	}{
		"should return addon template": {
			appName: "my-app",

			mockDependencies: func(ctrl *gomock.Controller, a *Addons) {
				ws := mocks.NewMockworkspaceService(ctrl)
				out := &workspace.AddonFiles{
					Outputs:    []string{"outputs"},
					Parameters: []string{"params"},
					Resources:  []string{"resources"},
				}
				ws.EXPECT().ReadAddonFiles("my-app").Return(out, nil)

				parser := templatemocks.NewMockParser(ctrl)
				parser.EXPECT().Parse(addonsTemplatePath, struct {
					AppName      string
					AddonContent *workspace.AddonFiles
				}{
					AppName:      a.appName,
					AddonContent: out,
				}).Return(&template.Content{Buffer: bytes.NewBufferString("hello")}, nil)

				a.ws = ws
				a.parser = parser
			},

			wantedTemplate: "hello",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			addons := &Addons{
				appName: tc.appName,
			}
			tc.mockDependencies(ctrl, addons)

			// WHEN
			gotTemplate, gotErr := addons.Template()

			// THEN
			require.Equal(t, tc.wantedErr, gotErr)
			require.Equal(t, tc.wantedTemplate, gotTemplate)
		})
	}
}
