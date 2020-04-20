// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/require"
)

func TestStore_ListEnvironments(t *testing.T) {
	testEnvironment := archer.Environment{Name: "test", AccountID: "12345", Project: "chicken", Region: "us-west-2s", Prod: false}
	testEnvironmentString, err := marshal(testEnvironment)
	testEnvironmentPath := fmt.Sprintf(fmtEnvParamPath, testEnvironment.Project, testEnvironment.Name)
	require.NoError(t, err, "Marshal environment should not fail")

	prodEnvironment := archer.Environment{Name: "prod", AccountID: "12345", Project: "chicken", Region: "us-west-2s", Prod: true}
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
							Name:  aws.String(prodEnvironmentPath), // <- return prod before test on purpose to test sorting by Prod
							Value: aws.String(prodEnvironmentString),
						},
						{
							Name:  aws.String(testEnvironmentPath),
							Value: aws.String(testEnvironmentString),
						},
					},
				}, nil
			},

			wantedEnvironments: []archer.Environment{testEnvironment, prodEnvironment},
			wantedErr:          nil,
		},
		"with malformed json": {
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
			wantedErr: fmt.Errorf("read environment details for project chicken: invalid character 'o' looking for beginning of value"),
		},
		"with SSM error": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, environmentPath, *param.Path)
				return nil, fmt.Errorf("broken")
			},
			wantedErr: fmt.Errorf("list environments for project chicken: broken"),
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
			store := &Store{
				ssmClient: &mockSSM{
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
				return nil, awserr.New(ssm.ErrCodeParameterNotFound, "bloop", nil)
			},
			wantedErr: &ErrNoSuchEnvironment{
				ProjectName:     testEnvironment.Project,
				EnvironmentName: testEnvironment.Name,
			},
		},
		"with malformed json": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testEnvironmentPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testEnvironmentPath),
						Value: aws.String("oops"),
					},
				}, nil
			},
			wantedErr: fmt.Errorf("read details for environment test in project chicken: invalid character 'o' looking for beginning of value"),
		},
		"with SSM error": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				return nil, fmt.Errorf("broken")
			},
			wantedErr: fmt.Errorf("get environment test in project chicken: broken"),
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
			wantedErr: nil,
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
			wantedErr: fmt.Errorf("create environment test in project chicken: broken"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			store := &Store{
				ssmClient: &mockSSM{
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

func TestStore_DeleteEnvironment(t *testing.T) {
	testCases := map[string]struct {
		inProjectName   string
		inEnvName       string
		mockDeleteParam func(t *testing.T, in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error)

		wantedError error
	}{
		"parameter is already deleted": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			mockDeleteParam: func(t *testing.T, in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error) {
				return nil, awserr.New(ssm.ErrCodeParameterNotFound, "Not found", nil)
			},
		},
		"unexpected error": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			mockDeleteParam: func(t *testing.T, in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error) {
				return nil, errors.New("some error")
			},
			wantedError: errors.New("delete environment test from project phonetool: some error"),
		},
		"successfully deleted param": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			mockDeleteParam: func(t *testing.T, in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error) {
				wantedPath := fmt.Sprintf(fmtEnvParamPath, "phonetool", "test")
				require.Equal(t, wantedPath, *in.Name)
				return nil, nil
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			store := &Store{
				ssmClient: &mockSSM{
					t:                   t,
					mockDeleteParameter: tc.mockDeleteParam,
				},
			}

			// WHEN
			err := store.DeleteEnvironment(tc.inProjectName, tc.inEnvName)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}
