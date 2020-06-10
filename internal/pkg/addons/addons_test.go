// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"bytes"
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addons/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	templatemocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/template/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestAddons_Template(t *testing.T) {
	svcName := "my-svc"
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, a *Addons)

		wantedTemplate string
		wantedErr      error
	}{
		"return ErrDirNotExist if ReadAddonsDir fails": {
			mockDependencies: func(ctrl *gomock.Controller, a *Addons) {
				ws := mocks.NewMockworkspaceReader(ctrl)
				ws.EXPECT().ReadAddonsDir(svcName).
					Return(nil, errors.New("some error"))
				a.ws = ws
			},
			wantedErr: &ErrDirNotExist{
				SvcName:   svcName,
				ParentErr: errors.New("some error"),
			},
		},
		"return error if missing required files": {
			mockDependencies: func(ctrl *gomock.Controller, a *Addons) {
				ws := mocks.NewMockworkspaceReader(ctrl)
				ws.EXPECT().ReadAddonsDir(svcName).
					Return([]string{
						"README.md",
					}, nil)
				ws.EXPECT().ReadAddon(svcName, "params.yaml").Times(0)

				a.ws = ws
			},

			wantedErr: errors.New(`addons directory has missing file(s): params.yaml, outputs.yaml, at least one resource YAML file such as "s3-bucket.yaml"`),
		},
		"return addon template": {
			mockDependencies: func(ctrl *gomock.Controller, a *Addons) {
				ws := mocks.NewMockworkspaceReader(ctrl)
				ws.EXPECT().ReadAddonsDir(svcName).
					Return([]string{
						"params.yaml",
						"outputs.yml",
						"policy.yaml",
						"README.md",
					}, nil)
				ws.EXPECT().ReadAddon(svcName, "params.yaml").
					Return([]byte("hello"), nil)
				ws.EXPECT().ReadAddon(svcName, "outputs.yml").
					Return([]byte("hello"), nil)
				ws.EXPECT().ReadAddon(svcName, "policy.yaml").
					Return([]byte("hello"), nil)

				parser := templatemocks.NewMockParser(ctrl)
				parser.EXPECT().Parse(addonsTemplatePath, struct {
					SvcName    string
					Parameters []string
					Resources  []string
					Outputs    []string
				}{
					SvcName:    a.svcName,
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
				svcName: svcName,
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

func TestAddons_template(t *testing.T) {
	const testSvcName = "mysvc"
	testErr := errors.New("some error")
	testCases := map[string]struct {
		mockAddons func(ctrl *gomock.Controller) *Addons

		wantedTemplate string
		wantedErr      error
	}{
		"return ErrDirNotExist if addons doesn't exist in a service": {
			mockAddons: func(ctrl *gomock.Controller) *Addons {
				ws := mocks.NewMockworkspaceReader(ctrl)
				ws.EXPECT().ReadAddonsDir(testSvcName).
					Return(nil, testErr)
				return &Addons{
					svcName: testSvcName,
					ws:      ws,
				}
			},
			wantedErr: &ErrDirNotExist{
				SvcName:   testSvcName,
				ParentErr: testErr,
			},
		},
		"return err on invalid Metadata fields": {
			mockAddons: func(ctrl *gomock.Controller) *Addons {
				ws := mocks.NewMockworkspaceReader(ctrl)
				ws.EXPECT().ReadAddonsDir(testSvcName).Return([]string{"first.yaml", "invalid-metadata.yaml"}, nil)

				first, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				ws.EXPECT().ReadAddon(testSvcName, "first.yaml").Return(first, nil)

				second, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "invalid-metadata.yaml"))
				ws.EXPECT().ReadAddon(testSvcName, "invalid-metadata.yaml").Return(second, nil)
				return &Addons{
					svcName: testSvcName,
					ws:      ws,
				}
			},
			wantedErr: errors.New(`merge addon invalid-metadata.yaml under service mysvc: metadata key "Services" already exists with a different definition`),
		},
		"returns err on invalid Parameters fields": {
			mockAddons: func(ctrl *gomock.Controller) *Addons {
				ws := mocks.NewMockworkspaceReader(ctrl)
				ws.EXPECT().ReadAddonsDir(testSvcName).Return([]string{"first.yaml", "invalid-parameters.yaml"}, nil)

				first, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				ws.EXPECT().ReadAddon(testSvcName, "first.yaml").Return(first, nil)

				second, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "invalid-parameters.yaml"))
				ws.EXPECT().ReadAddon(testSvcName, "invalid-parameters.yaml").Return(second, nil)
				return &Addons{
					svcName: testSvcName,
					ws:      ws,
				}
			},
			wantedErr: errors.New(`merge addon invalid-parameters.yaml under service mysvc: parameter logical ID "Name" already exists with a different definition`),
		},
		"returns err on invalid Mappings fields": {
			mockAddons: func(ctrl *gomock.Controller) *Addons {
				ws := mocks.NewMockworkspaceReader(ctrl)
				ws.EXPECT().ReadAddonsDir(testSvcName).Return([]string{"first.yaml", "invalid-mappings.yaml"}, nil)

				first, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				ws.EXPECT().ReadAddon(testSvcName, "first.yaml").Return(first, nil)

				second, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "invalid-mappings.yaml"))
				ws.EXPECT().ReadAddon(testSvcName, "invalid-mappings.yaml").Return(second, nil)
				return &Addons{
					svcName: testSvcName,
					ws:      ws,
				}
			},
			wantedErr: errors.New(`merge addon invalid-mappings.yaml under service mysvc: mapping "MyTableDynamoDBSettings.test" already exists with a different definition`),
		},
		"merge fields successfully": {
			mockAddons: func(ctrl *gomock.Controller) *Addons {
				ws := mocks.NewMockworkspaceReader(ctrl)
				ws.EXPECT().ReadAddonsDir(testSvcName).Return([]string{"first.yaml", "second.yaml"}, nil)

				first, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				ws.EXPECT().ReadAddon(testSvcName, "first.yaml").Return(first, nil)

				second, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "second.yaml"))
				ws.EXPECT().ReadAddon(testSvcName, "second.yaml").Return(second, nil)
				return &Addons{
					svcName: testSvcName,
					ws:      ws,
				}
			},
			wantedTemplate: func() string {
				wanted, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "wanted.yaml"))
				return string(wanted)
			}(),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			addons := tc.mockAddons(ctrl)

			// WHEN
			actualTemplate, actualErr := addons.template()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, actualErr, tc.wantedErr.Error())
			} else {
				require.NoError(t, actualErr)
				require.Equal(t, tc.wantedTemplate, actualTemplate)
			}
		})
	}
}
