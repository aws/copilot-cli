// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/addon/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestWorkload_Template(t *testing.T) {
	const (
		testSvcName = "mysvc"
		testJobName = "resizer"
	)
	testErr := errors.New("some error")
	testCases := map[string]struct {
		workloadName string
		setupMocks   func(m addonMocks)

		wantedTemplate            string
		wantedErr                 error
		wantedAddonsNotFoundError bool
	}{
		"return ErrAddonsNotFound if addons doesn't exist in a service": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath(testSvcName).Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return(nil, testErr)
			},
			wantedErr: fmt.Errorf("list addons under path mockPath: %w", &ErrAddonsNotFound{
				ParentErr: testErr,
			}),
		},
		"return ErrAddonsNotFound if addons doesn't exist in a job": {
			workloadName: testJobName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath(testJobName).Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return(nil, testErr)
			},
			wantedErr: fmt.Errorf("list addons under path mockPath: %w", &ErrAddonsNotFound{
				ParentErr: testErr,
			}),
		},
		"return ErrAddonsNotFound if addons directory is empty in a service": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath(testSvcName).Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{}, nil)
			},
			wantedErr: &ErrAddonsNotFound{
				ParentErr: nil,
			},
		},
		"return ErrAddonsNotFound if addons directory does not contain yaml files in a service": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath(testSvcName).Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"gitkeep"}, nil)
			},
			wantedErr: &ErrAddonsNotFound{
				ParentErr: nil,
			},
		},
		"ignore addons.parameters.yml files": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath(testSvcName).Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"addons.parameters.yml", "addons.parameters.yaml"}, nil)
			},
			wantedErr: &ErrAddonsNotFound{
				ParentErr: nil,
			},
		},
		"return err on invalid Metadata fields": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath(testSvcName).Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"first.yaml", "invalid-metadata.yaml"}, nil)

				first, _ := os.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().WorkloadAddonFileAbsPath(testSvcName, "first.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(first, nil)

				second, _ := os.ReadFile(filepath.Join("testdata", "merge", "invalid-metadata.yaml"))
				m.ws.EXPECT().WorkloadAddonFileAbsPath(testSvcName, "invalid-metadata.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(second, nil)
			},
			wantedErr: errors.New(`metadata key "Services" defined in "first.yaml" at Ln 4, Col 7 is different than in "invalid-metadata.yaml" at Ln 3, Col 5`),
		},
		"returns err on invalid Parameters fields": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath(testSvcName).Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"first.yaml", "invalid-parameters.yaml"}, nil)

				first, _ := os.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().WorkloadAddonFileAbsPath(testSvcName, "first.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(first, nil)

				second, _ := os.ReadFile(filepath.Join("testdata", "merge", "invalid-parameters.yaml"))
				m.ws.EXPECT().WorkloadAddonFileAbsPath(testSvcName, "invalid-parameters.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(second, nil)
			},
			wantedErr: errors.New(`parameter logical ID "Name" defined in "first.yaml" at Ln 15, Col 9 is different than in "invalid-parameters.yaml" at Ln 3, Col 7`),
		},
		"returns err on invalid Mappings fields": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath(testSvcName).Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"first.yaml", "invalid-mappings.yaml"}, nil)

				first, _ := os.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().WorkloadAddonFileAbsPath(testSvcName, "first.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(first, nil)

				second, _ := os.ReadFile(filepath.Join("testdata", "merge", "invalid-mappings.yaml"))
				m.ws.EXPECT().WorkloadAddonFileAbsPath(testSvcName, "invalid-mappings.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(second, nil)
			},
			wantedErr: errors.New(`mapping "MyTableDynamoDBSettings.test" defined in "first.yaml" at Ln 21, Col 13 is different than in "invalid-mappings.yaml" at Ln 4, Col 7`),
		},
		"returns err on invalid Conditions fields": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath(testSvcName).Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"first.yaml", "invalid-conditions.yaml"}, nil)

				first, _ := os.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().WorkloadAddonFileAbsPath(testSvcName, "first.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(first, nil)

				second, _ := os.ReadFile(filepath.Join("testdata", "merge", "invalid-conditions.yaml"))
				m.ws.EXPECT().WorkloadAddonFileAbsPath(testSvcName, "invalid-conditions.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(second, nil)
			},
			wantedErr: errors.New(`condition "IsProd" defined in "first.yaml" at Ln 28, Col 13 is different than in "invalid-conditions.yaml" at Ln 2, Col 13`),
		},
		"returns err on invalid Resources fields": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath(testSvcName).Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"first.yaml", "invalid-resources.yaml"}, nil)

				first, _ := os.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().WorkloadAddonFileAbsPath(testSvcName, "first.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(first, nil)

				second, _ := os.ReadFile(filepath.Join("testdata", "merge", "invalid-resources.yaml"))
				m.ws.EXPECT().WorkloadAddonFileAbsPath(testSvcName, "invalid-resources.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(second, nil)
			},
			wantedErr: errors.New(`resource "MyTable" defined in "first.yaml" at Ln 34, Col 9 is different than in "invalid-resources.yaml" at Ln 3, Col 5`),
		},
		"returns err on invalid Outputs fields": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath(testSvcName).Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"first.yaml", "invalid-outputs.yaml"}, nil)

				first, _ := os.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().WorkloadAddonFileAbsPath(testSvcName, "first.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(first, nil)

				second, _ := os.ReadFile(filepath.Join("testdata", "merge", "invalid-outputs.yaml"))
				m.ws.EXPECT().WorkloadAddonFileAbsPath(testSvcName, "invalid-outputs.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(second, nil)
			},
			wantedErr: errors.New(`output "MyTableAccessPolicy" defined in "first.yaml" at Ln 85, Col 9 is different than in "invalid-outputs.yaml" at Ln 3, Col 5`),
		},
		"merge fields successfully": {
			workloadName: testSvcName,
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath(testSvcName).Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"first.yaml", "second.yaml"}, nil)

				first, _ := os.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().WorkloadAddonFileAbsPath(testSvcName, "first.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(first, nil)

				second, _ := os.ReadFile(filepath.Join("testdata", "merge", "second.yaml"))
				m.ws.EXPECT().WorkloadAddonFileAbsPath(testSvcName, "second.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(second, nil)
			},
			wantedTemplate: func() string {
				wanted, _ := os.ReadFile(filepath.Join("testdata", "merge", "wanted.yaml"))
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
				ws: mocks.NewMockWorkspaceAddonsReader(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(mocks)
			}

			// WHEN
			stack, err := ParseFromWorkload(tc.workloadName, mocks.ws)
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.workloadName, stack.workloadName)

			template, err := stack.Template()
			require.NoError(t, err)
			require.Equal(t, tc.wantedTemplate, template)
		})
	}
}

func TestWorkload_Parameters(t *testing.T) {
	mockTemplate := `Parameters:
  App:
    Type: String
  Env:
    Type: String
  Name:
    Type: String
`
	testCases := map[string]struct {
		setupMocks func(m addonMocks)

		wantedParams string
		wantedErr    error
	}{
		"returns ErrAddonsNotFound if there is no addons/ directory defined": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath("api").Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return(nil, errors.New("some error"))
			},
			wantedErr: fmt.Errorf(`list addons under path mockPath: %w`, &ErrAddonsNotFound{
				ParentErr: errors.New("some error"),
			}),
		},
		"returns empty string and nil if there are no parameter files under addons/": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath("api").Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"database.yaml"}, nil)
				m.ws.EXPECT().WorkloadAddonFileAbsPath("api", "database.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return([]byte(mockTemplate), nil)
			},
		},
		"returns an error if there are multiple parameter files defined under addons/": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath("api").Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"database.yml", "addons.parameters.yml", "addons.parameters.yaml"}, nil)
				m.ws.EXPECT().WorkloadAddonFileAbsPath("api", "database.yml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(nil, nil)
			},
			wantedErr: errors.New("defining addons.parameters.yaml and addons.parameters.yml is not allowed under addons/"),
		},
		"returns an error if cannot read parameter file under addons/": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath("api").Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"template.yml", "addons.parameters.yml"}, nil)
				m.ws.EXPECT().WorkloadAddonFileAbsPath("api", "template.yml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(nil, nil)
				m.ws.EXPECT().WorkloadAddonFileAbsPath("api", "addons.parameters.yml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New("read parameter file addons.parameters.yml under path mockPath: some error"),
		},
		"returns an error if there are no 'Parameters' field defined in a parameters file": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath("api").Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"template.yaml", "addons.parameters.yml"}, nil)
				m.ws.EXPECT().WorkloadAddonFileAbsPath("api", "template.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(nil, nil)
				m.ws.EXPECT().WorkloadAddonFileAbsPath("api", "addons.parameters.yml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return([]byte(""), nil)
			},
			wantedErr: errors.New("must define field 'Parameters' in file addons.parameters.yml under path mockPath"),
		},
		"returns an error if reserved parameter fields is redefined in a parameters file": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath("api").Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"template.yaml", "addons.parameters.yml"}, nil)
				m.ws.EXPECT().WorkloadAddonFileAbsPath("api", "template.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return([]byte(mockTemplate), nil)
				m.ws.EXPECT().WorkloadAddonFileAbsPath("api", "addons.parameters.yml").Return("mockParametersPath")
				m.ws.EXPECT().ReadFile("mockParametersPath").Return([]byte(`
Parameters:
  App: !Ref AppName
  Env: !Ref EnvName
  Name: !Ref WorkloadName
  EventsQueue: 
    !Ref EventsQueue
  DiscoveryServiceArn: !GetAtt DiscoveryService.Arn
`), nil)
			},
			wantedErr: errors.New(`reserved parameters "App", "Env" and "Name" cannot be declared`),
		},
		"returns the content of Parameters on success": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().WorkloadAddonsAbsPath("api").Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"template.yaml", "addons.parameters.yaml"}, nil)
				m.ws.EXPECT().WorkloadAddonFileAbsPath("api", "template.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return([]byte(`Parameters:
  App:
    Type: String
  Env:
    Type: String
  Name:
    Type: String
  EventsQueue:
    Type: String
  ServiceName:
    Type: String
  SecurityGroupId:
    Type: String
  DiscoveryServiceArn:
    Type: String
`), nil)
				m.ws.EXPECT().WorkloadAddonFileAbsPath("api", "addons.parameters.yaml").Return("mockParametersPath")
				m.ws.EXPECT().ReadFile("mockParametersPath").Return([]byte(`
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
				ws: mocks.NewMockWorkspaceAddonsReader(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(mocks)
			}

			// WHEN
			stack, err := ParseFromWorkload("api", mocks.ws)
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
				return
			}
			require.NoError(t, err)
			require.Equal(t, "api", stack.workloadName)

			params, err := stack.Parameters()
			require.NoError(t, err)
			require.Equal(t, tc.wantedParams, params)
		})
	}
}

func TestEnv_Template(t *testing.T) {
	testErr := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(m addonMocks)

		wantedTemplate string
		wantedErr      error
	}{
		"return ErrAddonsNotFound if addons doesn't exist in an environment": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return(nil, testErr)
			},
			wantedErr: fmt.Errorf("list addons under path mockPath: %w", &ErrAddonsNotFound{
				ParentErr: testErr,
			}),
		},
		"return ErrAddonsNotFound if addons directory is empty in an environment": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{}, nil)
			},
			wantedErr: &ErrAddonsNotFound{},
		},
		"return ErrAddonsNotFound if addons directory does not contain yaml files in an environment": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"gitkeep"}, nil)
			},
			wantedErr: &ErrAddonsNotFound{},
		},
		"ignore addons.parameters.yml files": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"addons.parameters.yml", "addons.parameters.yaml"}, nil)
			},
			wantedErr: &ErrAddonsNotFound{},
		},
		"return err on invalid Metadata fields": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"first.yaml", "invalid-metadata.yaml"}, nil)

				first, _ := os.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().EnvAddonFileAbsPath("first.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(first, nil)

				second, _ := os.ReadFile(filepath.Join("testdata", "merge", "invalid-metadata.yaml"))
				m.ws.EXPECT().EnvAddonFileAbsPath("invalid-metadata.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(second, nil)
			},
			wantedErr: errors.New(`metadata key "Services" defined in "first.yaml" at Ln 4, Col 7 is different than in "invalid-metadata.yaml" at Ln 3, Col 5`),
		},
		"returns err on invalid Parameters fields": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"first.yaml", "invalid-parameters.yaml"}, nil)

				first, _ := os.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().EnvAddonFileAbsPath("first.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(first, nil)

				second, _ := os.ReadFile(filepath.Join("testdata", "merge", "invalid-parameters.yaml"))
				m.ws.EXPECT().EnvAddonFileAbsPath("invalid-parameters.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(second, nil)
			},
			wantedErr: errors.New(`parameter logical ID "Name" defined in "first.yaml" at Ln 15, Col 9 is different than in "invalid-parameters.yaml" at Ln 3, Col 7`),
		},
		"returns err on invalid Mappings fields": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"first.yaml", "invalid-mappings.yaml"}, nil)

				first, _ := os.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().EnvAddonFileAbsPath("first.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(first, nil)

				second, _ := os.ReadFile(filepath.Join("testdata", "merge", "invalid-mappings.yaml"))
				m.ws.EXPECT().EnvAddonFileAbsPath("invalid-mappings.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(second, nil)
			},
			wantedErr: errors.New(`mapping "MyTableDynamoDBSettings.test" defined in "first.yaml" at Ln 21, Col 13 is different than in "invalid-mappings.yaml" at Ln 4, Col 7`),
		},
		"returns err on invalid Conditions fields": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"first.yaml", "invalid-conditions.yaml"}, nil)

				first, _ := os.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().EnvAddonFileAbsPath("first.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(first, nil)

				second, _ := os.ReadFile(filepath.Join("testdata", "merge", "invalid-conditions.yaml"))
				m.ws.EXPECT().EnvAddonFileAbsPath("invalid-conditions.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(second, nil)
			},
			wantedErr: errors.New(`condition "IsProd" defined in "first.yaml" at Ln 28, Col 13 is different than in "invalid-conditions.yaml" at Ln 2, Col 13`),
		},
		"returns err on invalid Resources fields": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"first.yaml", "invalid-resources.yaml"}, nil)

				first, _ := os.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().EnvAddonFileAbsPath("first.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(first, nil)

				second, _ := os.ReadFile(filepath.Join("testdata", "merge", "invalid-resources.yaml"))
				m.ws.EXPECT().EnvAddonFileAbsPath("invalid-resources.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(second, nil)
			},
			wantedErr: errors.New(`resource "MyTable" defined in "first.yaml" at Ln 34, Col 9 is different than in "invalid-resources.yaml" at Ln 3, Col 5`),
		},
		"returns err on invalid Outputs fields": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"first.yaml", "invalid-outputs.yaml"}, nil)

				first, _ := os.ReadFile(filepath.Join("testdata", "merge", "first.yaml"))
				m.ws.EXPECT().EnvAddonFileAbsPath("first.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(first, nil)

				second, _ := os.ReadFile(filepath.Join("testdata", "merge", "invalid-outputs.yaml"))
				m.ws.EXPECT().EnvAddonFileAbsPath("invalid-outputs.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(second, nil)
			},
			wantedErr: errors.New(`output "MyTableAccessPolicy" defined in "first.yaml" at Ln 85, Col 9 is different than in "invalid-outputs.yaml" at Ln 3, Col 5`),
		},
		"merge fields successfully": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"first.yaml", "second.yaml"}, nil)

				first, _ := os.ReadFile(filepath.Join("testdata", "merge", "env", "first.yaml"))
				m.ws.EXPECT().EnvAddonFileAbsPath("first.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(first, nil)

				second, _ := os.ReadFile(filepath.Join("testdata", "merge", "env", "second.yaml"))
				m.ws.EXPECT().EnvAddonFileAbsPath("second.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(second, nil)
			},
			wantedTemplate: func() string {
				wanted, _ := os.ReadFile(filepath.Join("testdata", "merge", "env", "wanted.yaml"))
				return string(wanted)
			}(),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := addonMocks{
				ws: mocks.NewMockWorkspaceAddonsReader(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(m)
			}

			// WHEN
			stack, err := ParseFromEnv(m.ws)
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

func TestEnv_Parameters(t *testing.T) {
	mockTemplate := `Parameters:
  App:
    Type: String
  Env:
    Type: String`
	testCases := map[string]struct {
		setupMocks func(m addonMocks)

		wantedParams string
		wantedErr    error
	}{
		"returns ErrAddonsNotFound if there is no addons/ directory defined": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return(nil, errors.New("some error"))
			},
			wantedErr: fmt.Errorf("list addons under path mockPath: %w", &ErrAddonsNotFound{
				ParentErr: errors.New("some error"),
			}),
		},
		"returns empty string and nil if there are no parameter files under addons/": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"database.yaml"}, nil)
				m.ws.EXPECT().EnvAddonFileAbsPath("database.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return([]byte(mockTemplate), nil)
			},
		},
		"returns an error if there are multiple parameter files defined under addons/": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"database.yml", "addons.parameters.yml", "addons.parameters.yaml"}, nil)
				m.ws.EXPECT().EnvAddonFileAbsPath("database.yml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(nil, nil)
			},
			wantedErr: errors.New("defining addons.parameters.yaml and addons.parameters.yml is not allowed under addons/"),
		},
		"returns an error if cannot read parameter file under addons/": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"template.yml", "addons.parameters.yml"}, nil)
				m.ws.EXPECT().EnvAddonFileAbsPath("template.yml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(nil, nil)
				m.ws.EXPECT().EnvAddonFileAbsPath("addons.parameters.yml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New("read parameter file addons.parameters.yml under path mockPath: some error"),
		},
		"returns an error if there are no 'Parameters' field defined in a parameters file": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"template.yaml", "addons.parameters.yml"}, nil)
				m.ws.EXPECT().EnvAddonFileAbsPath("template.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return(nil, nil)
				m.ws.EXPECT().EnvAddonFileAbsPath("addons.parameters.yml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return([]byte(""), nil)
			},
			wantedErr: errors.New("must define field 'Parameters' in file addons.parameters.yml under path mockPath"),
		},
		"returns an error if reserved parameter fields is redefined in a parameters file": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"template.yaml", "addons.parameters.yml"}, nil)
				m.ws.EXPECT().EnvAddonFileAbsPath("template.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return([]byte(mockTemplate), nil)
				m.ws.EXPECT().EnvAddonFileAbsPath("addons.parameters.yml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return([]byte(`
Parameters:
  App: !Ref AppName
  Env: !Ref EnvName
  EventsQueue: 
    !Ref EventsQueue
  DiscoveryServiceArn: !GetAtt DiscoveryService.Arn
`), nil)
			},
			wantedErr: errors.New(`reserved parameters "App" and "Env" cannot be declared`),
		},
		"returns the content of Parameters on success": {
			setupMocks: func(m addonMocks) {
				m.ws.EXPECT().EnvAddonsAbsPath().Return("mockPath")
				m.ws.EXPECT().ListFiles("mockPath").Return([]string{"template.yaml", "addons.parameters.yaml"}, nil)
				m.ws.EXPECT().EnvAddonFileAbsPath("template.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return([]byte(`Parameters:
  App:
    Type: String
  Env:
    Type: String
  EventsQueue: 
    Type: String
  ServiceName:
    Type: String
  SecurityGroupId: 
    Type: String
  DiscoveryServiceArn:
    Type: String
`), nil)
				m.ws.EXPECT().EnvAddonFileAbsPath("addons.parameters.yaml").Return("mockPath")
				m.ws.EXPECT().ReadFile("mockPath").Return([]byte(`
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
				ws: mocks.NewMockWorkspaceAddonsReader(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(mocks)
			}

			// WHEN
			stack, err := ParseFromEnv(mocks.ws)
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
				return
			}
			require.NoError(t, err)

			params, err := stack.Parameters()
			require.NoError(t, err)
			require.Equal(t, tc.wantedParams, params)
		})
	}
}

func Test_validaTemplateParameters(t *testing.T) {
	type content struct {
		Parameters yaml.Node `yaml:"Parameters"`
	}
	testCases := map[string]struct {
		rawParams   string
		rawTpl      string
		wantedError error
	}{
		"template parameters with default values are not required in parameters file": {
			rawParams: `Parameters:`,
			rawTpl: `Parameters:
  App:
    Type: String
    Description: Your application's name.
  Env:
    Type: String
    Description: The environment name your service, job, or workflow is being deployed to.
  IsProd:
    Type: String
    Default: "false"
`,
		},
		"some template parameters are missing from the parameters file": {
			rawParams: `Parameters:`,
			rawTpl: `Parameters:
  App:
    Type: String
    Description: Your application's name.
  Env:
    Type: String
    Description: The environment name your service, job, or workflow is being deployed to.
  InstanceType:
    Type: 'AWS::SSM::Parameter::Value<String>'
`,
			wantedError: errors.New(`parameter "InstanceType" in template must have a default value or is included in parameters file`),
		},
		"template does not have required parameters": {
			rawParams: `Parameters:`,
			rawTpl: `Parameters:
  App:
    Type: String
    Description: Your application's name.
  IsProd:
    Type: String
    Default: "false"
`,
			wantedError: errors.New(`required parameter "Env" is missing from the template`),
		},
		"parameters file contains reserved keys": {
			rawParams: `Parameters:
  App: !Ref AppName
  Env: !Ref EnvName
  Name: !Ref WorkloadName
  EventsQueue: 
    !Ref EventsQueue
  DiscoveryServiceArn: !GetAtt DiscoveryService.Arn`,
			rawTpl: `Parameters:
  App:
    Type: String
    Description: Your application's name.
  Env:
    Type: String
    Description: The environment name your service, job, or workflow is being deployed to.
  InstanceType:
    Type: 'AWS::SSM::Parameter::Value<String>'`,
			wantedError: errors.New(`reserved parameters "App" and "Env" cannot be declared`),
		},
		"parameters file contains parameters that are not required by the template": {
			rawParams: `Parameters:
  ServiceName: !Ref Service`,
			rawTpl: `Parameters:
  App:
    Type: String
    Description: Your application's name.
  Env:
    Type: String
    Description: The environment name your service, job, or workflow is being deployed to.
  IsProd:
    Type: String
    Default: "false"
`,
			wantedError: errors.New(`template does not require the parameter "ServiceName" in parameters file`),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parameters := content{}
			err := yaml.Unmarshal([]byte(tc.rawParams), &parameters)
			require.NoError(t, err)

			tpl := content{}
			err = yaml.Unmarshal([]byte(tc.rawTpl), &tpl)
			require.NoError(t, err)

			err = validateParameters(tpl.Parameters, parameters.Parameters, envAddonsParameterReservedKeys)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
