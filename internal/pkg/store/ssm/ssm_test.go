// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ssm

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/stretchr/testify/require"
)

func TestStore_ListProjects(t *testing.T) {

	testProject := archer.Project{Name: "chicken", Version: "1.0"}
	testProjectString, err := marshal(testProject)
	require.NoError(t, err, "Marshal project should not fail")

	cowProject := archer.Project{Name: "cow", Version: "1.0"}
	cowProjectString, err := marshal(cowProject)
	require.NoError(t, err, "Marshal project should not fail")

	lastPageInPaginatedResp := false

	testCases := map[string]struct {
		mockGetParametersByPath func(t *testing.T, param *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error)

		wantedProjectNames []string
		wantedErr          error
	}{
		"with multiple existing projects": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, rootProjectPath, *param.Path)
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String("/archer/chicken"),
							Value: aws.String(testProjectString),
						},
						{
							Name:  aws.String("/archer/cow"),
							Value: aws.String(cowProjectString),
						},
					},
				}, nil
			},

			wantedProjectNames: []string{"chicken", "cow"},
			wantedErr:          nil,
		},
		"with malfored json": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, rootProjectPath, *param.Path)
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String("/archer/chicken"),
							Value: aws.String("oops"),
						},
					},
				}, nil
			},
			wantedErr: fmt.Errorf("invalid character 'o' looking for beginning of value"),
		},
		"with SSM error": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, rootProjectPath, *param.Path)
				return nil, fmt.Errorf("broken")
			},

			wantedProjectNames: nil,
			wantedErr:          fmt.Errorf("broken"),
		},
		"with paginated response": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, rootProjectPath, *param.Path)

				// this closure references the `lastPageInPaginatedResp` variable
				// in order to determine the content of the response.
				if !lastPageInPaginatedResp {
					lastPageInPaginatedResp = true
					return &ssm.GetParametersByPathOutput{
						Parameters: []*ssm.Parameter{
							{
								Name:  aws.String("/archer/chicken"),
								Value: aws.String(testProjectString),
							},
						},
						NextToken: aws.String("more"),
					}, nil
				}
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String("/archer/cow"),
							Value: aws.String(cowProjectString),
						},
					},
				}, nil
			},

			wantedProjectNames: []string{"chicken", "cow"},
			wantedErr:          nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			lastPageInPaginatedResp = false
			store := &SSM{
				systemManager: &mockSSM{
					t:                       t,
					mockGetParametersByPath: tc.mockGetParametersByPath,
				},
			}

			// WHEN
			projects, err := store.ListProjects()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				var names []string
				for _, p := range projects {
					names = append(names, p.Name)
				}
				require.Equal(t, tc.wantedProjectNames, names)

			}
		})
	}
}

func TestStore_GetProject(t *testing.T) {

	testProject := archer.Project{Name: "chicken", Version: "1.0"}
	testProjectString, err := marshal(testProject)
	testProjectPath := fmt.Sprintf(fmtProjectPath, testProject.Name)
	require.NoError(t, err, "Marshal project should not fail")

	testCases := map[string]struct {
		mockGetParameter      func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
		mockGetCallerIdentity func(t *testing.T, param *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error)
		wantedProject         archer.Project
		wantedErr             error
	}{
		"with existing project": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testProjectPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testProjectPath),
						Value: aws.String(testProjectString),
					},
				}, nil
			},

			wantedProject: testProject,
			wantedErr:     nil,
		},
		"with no existing project": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testProjectPath, *param.Name)
				return nil, awserr.New(ssm.ErrCodeParameterNotFound, "No Parameter", fmt.Errorf("No Parameter"))
			},
			mockGetCallerIdentity: func(t *testing.T, input *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
				return &sts.GetCallerIdentityOutput{
					Account: aws.String("12345"),
				}, nil
			},
			wantedErr: &store.ErrNoSuchProject{
				ProjectName: "chicken",
				AccountID:   "12345",
				Region:      "us-west-2",
			},
		},
		"with no existing project and failed STS call": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testProjectPath, *param.Name)
				return nil, awserr.New(ssm.ErrCodeParameterNotFound, "No Parameter", fmt.Errorf("No Parameter"))
			},
			mockGetCallerIdentity: func(t *testing.T, input *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
				return nil, fmt.Errorf("Error")
			},
			wantedErr: &store.ErrNoSuchProject{
				ProjectName: "chicken",
				AccountID:   "unknown",
				Region:      "us-west-2",
			},
		},
		"with malfored json": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testProjectPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testProjectPath),
						Value: aws.String("oops"),
					},
				}, nil
			},

			wantedErr: fmt.Errorf("invalid character 'o' looking for beginning of value"),
		},
		"with SSM error": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testProjectPath, *param.Name)
				return nil, fmt.Errorf("broken")
			},
			wantedErr: fmt.Errorf("broken"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			store := &SSM{
				systemManager: &mockSSM{
					t:                t,
					mockGetParameter: tc.mockGetParameter,
				},
				tokenService: &mockSTS{
					t:                     t,
					mockGetCallerIdentity: tc.mockGetCallerIdentity,
				},
				sessionRegion: "us-west-2",
			}

			// WHEN
			project, err := store.GetProject("chicken")

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedProject, *project)
			}
		})
	}
}

func TestStore_CreateProject(t *testing.T) {
	testProject := archer.Project{Name: "chicken"}
	testProjectString := fmt.Sprintf("{\"name\":\"chicken\",\"version\":\"%s\"}", schemaVersion)
	marshal(testProject)
	testProjectPath := fmt.Sprintf(fmtProjectPath, testProject.Name)

	testCases := map[string]struct {
		mockPutParameter func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error)
		wantedErr        error
	}{
		"with no existing project": {
			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				require.Equal(t, testProjectPath, *param.Name)
				require.Equal(t, testProjectString, *param.Value)

				return &ssm.PutParameterOutput{
					Version: aws.Int64(1),
				}, nil
			},
			wantedErr: nil,
		},
		"with existing project": {
			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				require.Equal(t, testProjectPath, *param.Name)
				return nil, awserr.New(ssm.ErrCodeParameterAlreadyExists, "Already exists", fmt.Errorf("Already Exists"))
			},
			wantedErr: &store.ErrProjectAlreadyExists{
				ProjectName: "chicken",
			},
		},
		"with SSM error": {
			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				require.Equal(t, testProjectPath, *param.Name)
				return nil, fmt.Errorf("broken")
			},
			wantedErr: fmt.Errorf("broken"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			store := &SSM{
				systemManager: &mockSSM{
					t:                t,
					mockPutParameter: tc.mockPutParameter,
				},
			}

			// WHEN
			err := store.CreateProject(&archer.Project{Name: "chicken", Version: "1.0"})

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}

func TestStore_ListEnvironments(t *testing.T) {

	testEnvironment := archer.Environment{Name: "test", AccountID: "12345", Project: "chicken", Region: "us-west-2s"}
	testEnvironmentString, err := marshal(testEnvironment)
	testEnvironmentPath := fmt.Sprintf(fmtEnvParamPath, testEnvironment.Project, testEnvironment.Name)
	require.NoError(t, err, "Marshal environment should not fail")

	prodEnvironment := archer.Environment{Name: "prod", AccountID: "12345", Project: "chicken", Region: "us-west-2s"}
	prodEnvironmentString, err := marshal(prodEnvironment)
	prodEnvironmentPath := fmt.Sprintf(fmtEnvParamPath, prodEnvironment.Project, prodEnvironment.Name)
	require.NoError(t, err, "Marshal environment should not fail")

	environmentPath := fmt.Sprintf(rootEnvParamPath, testEnvironment.Project)

	lastPageInPaginatedResp := false

	testCases := map[string]struct {
		mockGetParametersByPath func(t *testing.T, param *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error)

		wantedEnvironments []archer.Environment
		wantedErr          error
	}{
		"with multiple existing environments": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, environmentPath, *param.Path)
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String(testEnvironmentPath),
							Value: aws.String(testEnvironmentString),
						},
						{
							Name:  aws.String(prodEnvironmentPath),
							Value: aws.String(prodEnvironmentString),
						},
					},
				}, nil
			},

			wantedEnvironments: []archer.Environment{testEnvironment, prodEnvironment},
			wantedErr:          nil,
		},
		"with malfored json": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, environmentPath, *param.Path)
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String(testEnvironmentPath),
							Value: aws.String("oops"),
						},
					},
				}, nil
			},
			wantedErr: fmt.Errorf("invalid character 'o' looking for beginning of value"),
		},
		"with SSM error": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, environmentPath, *param.Path)
				return nil, fmt.Errorf("broken")
			},
			wantedErr: fmt.Errorf("broken"),
		},
		"with paginated response": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, environmentPath, *param.Path)

				if !lastPageInPaginatedResp {
					lastPageInPaginatedResp = true
					return &ssm.GetParametersByPathOutput{
						Parameters: []*ssm.Parameter{
							{
								Name:  aws.String(testEnvironmentPath),
								Value: aws.String(testEnvironmentString),
							},
						},
						NextToken: aws.String("more"),
					}, nil
				}

				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String(prodEnvironmentPath),
							Value: aws.String(prodEnvironmentString),
						},
					},
				}, nil
			},

			wantedEnvironments: []archer.Environment{testEnvironment, prodEnvironment},
			wantedErr:          nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			lastPageInPaginatedResp = false
			store := &SSM{
				systemManager: &mockSSM{
					t:                       t,
					mockGetParametersByPath: tc.mockGetParametersByPath,
				},
			}

			// WHEN
			envPointers, err := store.ListEnvironments("chicken")
			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				var environments []archer.Environment
				for _, e := range envPointers {
					environments = append(environments, *e)
				}
				require.Equal(t, tc.wantedEnvironments, environments)

			}
		})
	}
}
func TestStore_GetEnvironment(t *testing.T) {
	testEnvironment := archer.Environment{Name: "test", AccountID: "12345", Project: "chicken", Region: "us-west-2s"}
	testEnvironmentString, err := marshal(testEnvironment)
	testEnvironmentPath := fmt.Sprintf(fmtEnvParamPath, testEnvironment.Project, testEnvironment.Name)
	require.NoError(t, err, "Marshal environment should not fail")

	testCases := map[string]struct {
		mockGetParameter  func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
		wantedEnvironment archer.Environment
		wantedErr         error
	}{
		"with existing environment": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testEnvironmentPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testEnvironmentPath),
						Value: aws.String(testEnvironmentString),
					},
				}, nil
			},
			wantedEnvironment: testEnvironment,
			wantedErr:         nil,
		},
		"with no existing environment": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testEnvironmentPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{},
				}, nil
			},
			wantedErr: &store.ErrNoSuchEnvironment{
				ProjectName:     testEnvironment.Project,
				EnvironmentName: testEnvironment.Name,
			},
		},
		"with malfored json": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testEnvironmentPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testEnvironmentPath),
						Value: aws.String("oops"),
					},
				}, nil
			},
			wantedErr: fmt.Errorf("invalid character 'o' looking for beginning of value"),
		},
		"with SSM error": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				return nil, fmt.Errorf("broken")
			},
			wantedErr: fmt.Errorf("broken"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			store := &SSM{
				systemManager: &mockSSM{
					t:                t,
					mockGetParameter: tc.mockGetParameter,
				},
			}

			// WHEN
			env, err := store.GetEnvironment("chicken", "test")

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedEnvironment, *env)
			}
		})
	}
}

func TestStore_CreateEnvironment(t *testing.T) {

	testProject := archer.Project{Name: "chicken", Version: "1.0"}
	testProjectString, err := marshal(testProject)
	testProjectPath := fmt.Sprintf(fmtProjectPath, testProject.Name)
	require.NoError(t, err, "Marshal project should not fail")

	testEnvironment := archer.Environment{Name: "test", Project: testProject.Name, AccountID: "1234", Region: "us-west-2"}
	testEnvironmentString, err := marshal(testEnvironment)
	testEnvironmentPath := fmt.Sprintf(fmtEnvParamPath, testEnvironment.Project, testEnvironment.Name)
	require.NoError(t, err, "Marshal environment should not fail")

	testCases := map[string]struct {
		mockGetParameter func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
		mockPutParameter func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error)
		wantedErr        error
	}{
		"with no existing environment": {
			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				require.Equal(t, testEnvironmentPath, *param.Name)
				require.Equal(t, testEnvironmentString, *param.Value)
				return &ssm.PutParameterOutput{
					Version: aws.Int64(1),
				}, nil
			},
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testProjectPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testProjectPath),
						Value: aws.String(testProjectString),
					},
				}, nil
			},

			wantedErr: nil,
		},
		"with existing environment": {
			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				require.Equal(t, testEnvironmentPath, *param.Name)
				return nil, awserr.New(ssm.ErrCodeParameterAlreadyExists, "Already exists", fmt.Errorf("Already Exists"))
			},
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testProjectPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testProjectPath),
						Value: aws.String(testProjectString),
					},
				}, nil
			},
			wantedErr: &store.ErrEnvironmentAlreadyExists{
				EnvironmentName: testEnvironment.Name,
				ProjectName:     testEnvironment.Project,
			},
		},
		"with SSM error": {
			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				return nil, fmt.Errorf("broken")
			},
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testProjectPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testProjectPath),
						Value: aws.String(testProjectString),
					},
				}, nil
			},
			wantedErr: fmt.Errorf("broken"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			store := &SSM{
				systemManager: &mockSSM{
					t:                t,
					mockPutParameter: tc.mockPutParameter,
					mockGetParameter: tc.mockGetParameter,
				},
			}

			// WHEN
			err := store.CreateEnvironment(&archer.Environment{
				Name:      testEnvironment.Name,
				Project:   testEnvironment.Project,
				AccountID: testEnvironment.AccountID,
				Region:    testEnvironment.Region})

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}

type mockSSM struct {
	ssmiface.SSMAPI
	t                       *testing.T
	mockPutParameter        func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error)
	mockGetParametersByPath func(t *testing.T, param *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error)
	mockGetParameter        func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
}

func (m *mockSSM) PutParameter(in *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
	return m.mockPutParameter(m.t, in)
}

func (m *mockSSM) GetParametersByPath(in *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error) {
	return m.mockGetParametersByPath(m.t, in)
}

func (m *mockSSM) GetParameter(in *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
	return m.mockGetParameter(m.t, in)
}

type mockSTS struct {
	stsiface.STSAPI
	t                     *testing.T
	mockGetCallerIdentity func(t *testing.T, input *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error)
}

func (m *mockSTS) GetCallerIdentity(input *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
	return m.mockGetCallerIdentity(m.t, input)
}
