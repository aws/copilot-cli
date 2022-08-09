// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/addon/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestTemplate(t *testing.T) {
	const (
		testSvcName = "mysvc"
		testJobName = "resizer"
	)
	testErr := errors.New("some error")
	testCases := map[string]struct {
		workloadName string
		setupMocks   func(m addonMocks)

		wantedTemplate string
		wantedErr      error
	}{
		"return ErrAddonsNotFound if addons doesn't exist in a service": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir(testSvcName).Return(nil, testErr)
			},
			wantedErr: &ErrAddonsNotFound{
				WlName:    testSvcName,
				ParentErr: testErr,
			},
		},
		"return ErrAddonsNotFound if addons doesn't exist in a job": {
			workloadName: testJobName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir(testJobName).Return(nil, testErr)
			},
			wantedErr: &ErrAddonsNotFound{
				WlName:    testJobName,
				ParentErr: testErr,
			},
		},
		"return ErrAddonsNotFound if addons directory is empty in a service": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir(testSvcName).Return([]string{}, nil)
			},
			wantedErr: &ErrAddonsNotFound{
				WlName:    testSvcName,
				ParentErr: nil,
			},
		},
		"return ErrAddonsNotFound if addons directory does not contain yaml files in a service": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir(testSvcName).Return([]string{".gitkeep"}, nil)
			},
			wantedErr: &ErrAddonsNotFound{
				WlName:    testSvcName,
				ParentErr: nil,
			},
		},
		"ignore addons.parameters.yml files": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir(testSvcName).Return([]string{"addons.parameters.yml", "addons.parameters.yaml"}, nil)
			},
			wantedErr: &ErrAddonsNotFound{
				WlName:    testSvcName,
				ParentErr: nil,
			},
		},
		"print correct error message for ErrAddonsNotFound": {
			workloadName: testJobName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir(testJobName).Return(nil, testErr)
			},
			wantedErr: errors.New("read addons directory for resizer: some error"),
		},
		"return err on invalid Metadata fields": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir(testSvcName).Return([]string{"first.yaml", "invalid-metadata.yaml"}, nil)

				first, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().ReadAddon(testSvcName, "first.yaml").Return(first, nil)

				second, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "invalid-metadata.yaml"))
				m.ws.EXPECT().ReadAddon(testSvcName, "invalid-metadata.yaml").Return(second, nil)
			},
			wantedErr: errors.New(`metadata key "Services" defined in "first.yaml" at Ln 4, Col 7 is different than in "invalid-metadata.yaml" at Ln 3, Col 5`),
		},
		"returns err on invalid Parameters fields": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir(testSvcName).Return([]string{"first.yaml", "invalid-parameters.yaml"}, nil)

				first, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().ReadAddon(testSvcName, "first.yaml").Return(first, nil)

				second, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "invalid-parameters.yaml"))
				m.ws.EXPECT().ReadAddon(testSvcName, "invalid-parameters.yaml").Return(second, nil)
			},
			wantedErr: errors.New(`parameter logical ID "Name" defined in "first.yaml" at Ln 15, Col 9 is different than in "invalid-parameters.yaml" at Ln 3, Col 7`),
		},
		"returns err on invalid Mappings fields": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir(testSvcName).Return([]string{"first.yaml", "invalid-mappings.yaml"}, nil)

				first, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().ReadAddon(testSvcName, "first.yaml").Return(first, nil)

				second, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "invalid-mappings.yaml"))
				m.ws.EXPECT().ReadAddon(testSvcName, "invalid-mappings.yaml").Return(second, nil)
			},
			wantedErr: errors.New(`mapping "MyTableDynamoDBSettings.test" defined in "first.yaml" at Ln 21, Col 13 is different than in "invalid-mappings.yaml" at Ln 4, Col 7`),
		},
		"returns err on invalid Conditions fields": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir(testSvcName).Return([]string{"first.yaml", "invalid-conditions.yaml"}, nil)

				first, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().ReadAddon(testSvcName, "first.yaml").Return(first, nil)

				second, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "invalid-conditions.yaml"))
				m.ws.EXPECT().ReadAddon(testSvcName, "invalid-conditions.yaml").Return(second, nil)
			},
			wantedErr: errors.New(`condition "IsProd" defined in "first.yaml" at Ln 28, Col 13 is different than in "invalid-conditions.yaml" at Ln 2, Col 13`),
		},
		"returns err on invalid Resources fields": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir(testSvcName).Return([]string{"first.yaml", "invalid-resources.yaml"}, nil)

				first, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().ReadAddon(testSvcName, "first.yaml").Return(first, nil)

				second, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "invalid-resources.yaml"))
				m.ws.EXPECT().ReadAddon(testSvcName, "invalid-resources.yaml").Return(second, nil)
			},
			wantedErr: errors.New(`resource "MyTable" defined in "first.yaml" at Ln 34, Col 9 is different than in "invalid-resources.yaml" at Ln 3, Col 5`),
		},
		"returns err on invalid Outputs fields": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir(testSvcName).Return([]string{"first.yaml", "invalid-outputs.yaml"}, nil)

				first, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().ReadAddon(testSvcName, "first.yaml").Return(first, nil)

				second, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "invalid-outputs.yaml"))
				m.ws.EXPECT().ReadAddon(testSvcName, "invalid-outputs.yaml").Return(second, nil)
			},
			wantedErr: errors.New(`output "MyTableAccessPolicy" defined in "first.yaml" at Ln 85, Col 9 is different than in "invalid-outputs.yaml" at Ln 3, Col 5`),
		},
		"merge fields successfully": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir(testSvcName).Return([]string{"first.yaml", "second.yaml"}, nil)

				first, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().ReadAddon(testSvcName, "first.yaml").Return(first, nil)

				second, _ := ioutil.ReadFile(filepath.Join("testdata", "merge", "second.yaml"))
				m.ws.EXPECT().ReadAddon(testSvcName, "second.yaml").Return(second, nil)
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

			mocks := addonMocks{
				ws: mocks.NewMockworkspaceReader(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(mocks)
			}

			// WHEN
			stack, err := Parse(tc.workloadName, mocks.ws)
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
				return
			}
			require.NoError(t, err)

			template, err := stack.Template()
			require.NoError(t, err)
			require.Equal(t, tc.wantedTemplate, template)
		})
	}
}

func TestParameters(t *testing.T) {
	testCases := map[string]struct {
		setupMocks func(m addonMocks)

		wantedParams string
		wantedErr    string
	}{
		"returns ErrAddonsNotFound if there is no addons/ directory defined": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir("api").Return(nil, errors.New("some error"))
			},
			wantedErr: (&ErrAddonsNotFound{
				WlName:    "api",
				ParentErr: errors.New("some error"),
			}).Error(),
		},
		"returns empty string and nil if there are no parameter files under addons/": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir("api").Return([]string{"database.yml"}, nil)
				m.ws.EXPECT().ReadAddon("api", "database.yml").Return(nil, nil)
			},
		},
		"returns an error if there are multiple parameter files defined under addons/": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir("api").
					Return([]string{"database.yml", "addons.parameters.yml", "addons.parameters.yaml"}, nil)
				m.ws.EXPECT().ReadAddon("api", "database.yml").Return(nil, nil)
			},
			wantedErr: "defining addons.parameters.yaml and addons.parameters.yml is not allowed under api addons/",
		},
		"returns an error if cannot read parameter file under addons/": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir("api").
					Return([]string{"addons.parameters.yml", "template.yaml"}, nil)
				m.ws.EXPECT().ReadAddon("api", "template.yaml").Return(nil, nil)
				m.ws.EXPECT().ReadAddon("api", "addons.parameters.yml").Return(nil, errors.New("some error"))
			},
			wantedErr: "read parameter file addons.parameters.yml under api addons/: some error",
		},
		"returns an error if there are no 'Parameters' field defined in a parameters file": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir("api").
					Return([]string{"addons.parameters.yml", "template.yaml"}, nil)
				m.ws.EXPECT().ReadAddon("api", "template.yaml").Return(nil, nil)
				m.ws.EXPECT().ReadAddon("api", "addons.parameters.yml").Return([]byte(""), nil)
			},
			wantedErr: "must define field 'Parameters' in file addons.parameters.yml under api addons/",
		},
		"returns an error if reserved parameter fields is redefined in a parameters file": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir("api").
					Return([]string{"addons.parameters.yml", "template.yaml"}, nil)
				m.ws.EXPECT().ReadAddon("api", "template.yaml").Return(nil, nil)
				m.ws.EXPECT().ReadAddon("api", "addons.parameters.yml").Return([]byte(`
Parameters:
  App: !Ref AppName
  Env: !Ref EnvName
  Name: !Ref WorkloadName
  EventsQueue: 
    !Ref EventsQueue
  DiscoveryServiceArn: !GetAtt DiscoveryService.Arn
`), nil)
			},
			wantedErr: "reserved parameters 'App', 'Env', and 'Name' cannot be declared in addons.parameters.yml under api addons/",
		},
		"returns the content of Parameters on success": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().ReadAddonsDir("api").
					Return([]string{"addons.parameters.yml", "template.yaml"}, nil)
				m.ws.EXPECT().ReadAddon("api", "template.yaml").Return(nil, nil)
				m.ws.EXPECT().ReadAddon("api", "addons.parameters.yml").Return([]byte(`
Parameters:
  EventsQueue: 
    !Ref EventsQueue
  ServiceName: !Ref Service
  SecurityGroupId: 
    Fn::GetAtt: [ServiceSecurityGroup, Id]
  DiscoveryServiceArn: !GetAtt DiscoveryService.Arn
`), nil)
			},
			wantedParams: `EventsQueue: !Ref EventsQueue
ServiceName: !Ref Service
SecurityGroupId:
  Fn::GetAtt: [ServiceSecurityGroup, Id]
DiscoveryServiceArn: !GetAtt DiscoveryService.Arn
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := addonMocks{
				ws: mocks.NewMockworkspaceReader(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(mocks)
			}

			// WHEN
			stack, err := Parse("api", mocks.ws)
			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
				return
			}
			require.NoError(t, err)

			params, err := stack.Parameters()
			require.NoError(t, err)
			require.Equal(t, tc.wantedParams, params)
		})
	}
}
