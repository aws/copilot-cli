// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"bytes"
	"errors"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addons/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	templatemocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/template/mocks"
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
		"return ErrDirNotExist if ReadAddonsDir fails": {
			appName: "my-app",
			mockDependencies: func(ctrl *gomock.Controller, a *Addons) {
				ws := mocks.NewMockworkspaceService(ctrl)
				ws.EXPECT().ReadAddonsDir("my-app").
					Return(nil, errors.New("some error"))
				a.ws = ws
			},
			wantedErr: &ErrDirNotExist{
				AppName:   "my-app",
				ParentErr: errors.New("some error"),
			},
		},
		"return error if missing required files": {
			appName: "my-app",
			mockDependencies: func(ctrl *gomock.Controller, a *Addons) {
				ws := mocks.NewMockworkspaceService(ctrl)
				ws.EXPECT().ReadAddonsDir("my-app").
					Return([]string{
						"README.md",
					}, nil)
				ws.EXPECT().ReadAddonsFile("my-app", "params.yaml").Times(0)

				a.ws = ws
			},

			wantedErr: errors.New(`addons directory has missing file(s): params.yaml, outputs.yaml, at least one resource YAML file such as "s3-bucket.yaml"`),
		},
		"return addon template": {
			appName: "my-app",

			mockDependencies: func(ctrl *gomock.Controller, a *Addons) {
				ws := mocks.NewMockworkspaceService(ctrl)
				ws.EXPECT().ReadAddonsDir("my-app").
					Return([]string{
						"params.yaml",
						"outputs.yml",
						"policy.yaml",
						"README.md",
					}, nil)
				ws.EXPECT().ReadAddonsFile("my-app", "params.yaml").
					Return([]byte("hello"), nil)
				ws.EXPECT().ReadAddonsFile("my-app", "outputs.yml").
					Return([]byte("hello"), nil)
				ws.EXPECT().ReadAddonsFile("my-app", "policy.yaml").
					Return([]byte("hello"), nil)

				parser := templatemocks.NewMockParser(ctrl)
				parser.EXPECT().Parse(addonsTemplatePath, struct {
					AppName    string
					Parameters []string
					Resources  []string
					Outputs    []string
				}{
					AppName:    a.appName,
					Parameters: []string{"hello"},
					Resources:  []string{"hello"},
					Outputs:    []string{"hello"},
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
