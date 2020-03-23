// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
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
							Name:  aws.String("/ecs-cli-v2/chicken"),
							Value: aws.String(testProjectString),
						},
						{
							Name:  aws.String("/ecs-cli-v2/cow"),
							Value: aws.String(cowProjectString),
						},
					},
				}, nil
			},

			wantedProjectNames: []string{"chicken", "cow"},
			wantedErr:          nil,
		},
		"with malformed json": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, rootProjectPath, *param.Path)
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String("/ecs-cli-v2/chicken"),
							Value: aws.String("oops"),
						},
					},
				}, nil
			},
			wantedErr: fmt.Errorf("read project details: invalid character 'o' looking for beginning of value"),
		},
		"with SSM error": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, rootProjectPath, *param.Path)
				return nil, fmt.Errorf("broken")
			},

			wantedProjectNames: nil,
			wantedErr:          fmt.Errorf("list projects: broken"),
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
								Name:  aws.String("/ecs-cli-v2/chicken"),
								Value: aws.String(testProjectString),
							},
						},
						NextToken: aws.String("more"),
					}, nil
				}
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String("/ecs-cli-v2/cow"),
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
			store := &Store{
				ssmClient: &mockSSM{
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
				require.ElementsMatch(t, tc.wantedProjectNames, names)

			}
		})
	}
}

func TestStore_GetProject(t *testing.T) {
	testProject := archer.Project{Name: "chicken", AccountID: "1234", Version: "1.0"}
	testProjectString, err := marshal(testProject)
	testProjectPath := fmt.Sprintf(fmtProjectPath, testProject.Name)
	require.NoError(t, err, "Marshal project should not fail")

	testCases := map[string]struct {
		mockGetParameter       func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
		mockIdentityServiceGet func() (identity.Caller, error)
		wantedProject          archer.Project
		wantedErr              error
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
			mockIdentityServiceGet: func() (identity.Caller, error) {
				return identity.Caller{
					Account: "12345",
				}, nil
			},
			wantedErr: &ErrNoSuchProject{
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
			mockIdentityServiceGet: func() (identity.Caller, error) {
				return identity.Caller{}, fmt.Errorf("Error")
			},
			wantedErr: &ErrNoSuchProject{
				ProjectName: "chicken",
				AccountID:   "unknown",
				Region:      "us-west-2",
			},
		},
		"with malformed json": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testProjectPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testProjectPath),
						Value: aws.String("oops"),
					},
				}, nil
			},

			wantedErr: fmt.Errorf("read details for project chicken: invalid character 'o' looking for beginning of value"),
		},
		"with SSM error": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testProjectPath, *param.Name)
				return nil, fmt.Errorf("broken")
			},
			wantedErr: fmt.Errorf("get project chicken: broken"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			store := &Store{
				ssmClient: &mockSSM{
					t:                t,
					mockGetParameter: tc.mockGetParameter,
				},
				idClient: mockIdentityService{
					mockIdentityServiceGet: tc.mockIdentityServiceGet,
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
	testCases := map[string]struct {
		inProject *archer.Project

		mockPutParameter func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error)
		wantedErr        error
	}{
		"with no existing project": {
			inProject: &archer.Project{Name: "phonetool", AccountID: "1234", Domain: "phonetool.com"},
			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				require.Equal(t, fmt.Sprintf(fmtProjectPath, "phonetool"), *param.Name)
				require.Equal(t, fmt.Sprintf(`{"name":"phonetool","account":"1234","domain":"phonetool.com","version":"%s"}`, schemaVersion), *param.Value)

				return &ssm.PutParameterOutput{
					Version: aws.Int64(1),
				}, nil
			},
			wantedErr: nil,
		},
		"with existing project": {
			inProject: &archer.Project{Name: "phonetool", AccountID: "1234"},
			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				return nil, awserr.New(ssm.ErrCodeParameterAlreadyExists, "Already exists", fmt.Errorf("Already Exists"))
			},
			wantedErr: nil,
		},
		"with SSM error": {
			inProject: &archer.Project{Name: "phonetool", AccountID: "1234"},
			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				return nil, fmt.Errorf("broken")
			},
			wantedErr: fmt.Errorf("create project phonetool: broken"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			store := &Store{
				ssmClient: &mockSSM{
					t:                t,
					mockPutParameter: tc.mockPutParameter,
				},
			}

			// WHEN
			err := store.CreateProject(tc.inProject)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestDeleteProject(t *testing.T) {
	mockProjectName := "mockProjectName"
	mockError := errors.New("mockError")

	tests := map[string]struct {
		mockDeleteParameter func(t *testing.T, in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error)

		want error
	}{
		"should return nil given success": {
			mockDeleteParameter: func(t *testing.T, in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error) {
				require.Equal(t, fmt.Sprintf(fmtProjectPath, mockProjectName), *in.Name)

				return &ssm.DeleteParameterOutput{}, nil
			},
			want: nil,
		},
		"should return nil given paramter not found error code": {
			mockDeleteParameter: func(t *testing.T, in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error) {
				require.Equal(t, fmt.Sprintf(fmtProjectPath, mockProjectName), *in.Name)

				return nil, awserr.New(ssm.ErrCodeParameterNotFound, "whatevs", mockError)
			},
			want: nil,
		},
		"should return unhandled non-awserr": {
			mockDeleteParameter: func(t *testing.T, in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error) {
				require.Equal(t, fmt.Sprintf(fmtProjectPath, mockProjectName), *in.Name)

				return nil, mockError
			},
			want: mockError,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			store := &Store{
				ssmClient: &mockSSM{
					t:                   t,
					mockDeleteParameter: test.mockDeleteParameter,
				},
			}

			got := store.DeleteProject(mockProjectName)

			require.Equal(t, test.want, got)
		})
	}
}
