// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/require"
)

func TestStore_ListEnvironments(t *testing.T) {
	testEnvironment := Environment{Name: "test", AccountID: "12345", App: "chicken", Region: "us-west-2s", Prod: false}
	testEnvironmentString, err := marshal(testEnvironment)
	testEnvironmentPath := fmt.Sprintf(fmtEnvParamPath, testEnvironment.App, testEnvironment.Name)
	require.NoError(t, err, "Marshal environment should not fail")

	prodEnvironment := Environment{Name: "prod", AccountID: "12345", App: "chicken", Region: "us-west-2s", Prod: true}
	prodEnvironmentString, err := marshal(prodEnvironment)
	prodEnvironmentPath := fmt.Sprintf(fmtEnvParamPath, prodEnvironment.App, prodEnvironment.Name)
	require.NoError(t, err, "Marshal environment should not fail")

	environmentPath := fmt.Sprintf(rootEnvParamPath, testEnvironment.App)

	lastPageInPaginatedResp := false

	testCases := map[string]struct {
		mockGetParametersByPath func(t *testing.T, param *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error)

		wantedEnvironments []Environment
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

			wantedEnvironments: []Environment{testEnvironment, prodEnvironment},
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
			wantedErr: fmt.Errorf("read environment configuration for application chicken: invalid character 'o' looking for beginning of value"),
		},
		"with SSM error": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, environmentPath, *param.Path)
				return nil, fmt.Errorf("broken")
			},
			wantedErr: fmt.Errorf("list environments for application chicken: broken"),
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

			wantedEnvironments: []Environment{testEnvironment, prodEnvironment},
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
				var environments []Environment
				for _, e := range envPointers {
					environments = append(environments, *e)
				}
				require.Equal(t, tc.wantedEnvironments, environments)

			}
		})
	}
}

func TestStore_GetEnvironment(t *testing.T) {
	testEnvironment := Environment{Name: "test", AccountID: "12345", App: "chicken", Region: "us-west-2s"}
	testEnvironmentString, err := marshal(testEnvironment)
	testEnvironmentPath := fmt.Sprintf(fmtEnvParamPath, testEnvironment.App, testEnvironment.Name)
	require.NoError(t, err, "Marshal environment should not fail")

	testCases := map[string]struct {
		mockGetParameter  func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
		wantedEnvironment Environment
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
				ApplicationName: testEnvironment.App,
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
			wantedErr: fmt.Errorf("read configuration for environment test in application chicken: invalid character 'o' looking for beginning of value"),
		},
		"with SSM error": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				return nil, fmt.Errorf("broken")
			},
			wantedErr: fmt.Errorf("get environment test in application chicken: broken"),
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
	testApplication := Application{Name: "chicken", Version: "1.0"}
	testApplicationString, err := marshal(testApplication)
	testApplicationPath := fmt.Sprintf(fmtApplicationPath, testApplication.Name)
	require.NoError(t, err, "Marshal application should not fail")

	testEnvironment := Environment{Name: "test", App: testApplication.Name, AccountID: "1234", Region: "us-west-2"}
	testEnvironmentString, err := marshal(testEnvironment)
	testEnvironmentPath := fmt.Sprintf(fmtEnvParamPath, testEnvironment.App, testEnvironment.Name)
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
				require.Equal(t, testApplicationPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testApplicationPath),
						Value: aws.String(testApplicationString),
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
				require.Equal(t, testApplicationPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testApplicationPath),
						Value: aws.String(testApplicationString),
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
				require.Equal(t, testApplicationPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testApplicationPath),
						Value: aws.String(testApplicationString),
					},
				}, nil
			},
			wantedErr: fmt.Errorf("create environment test in application chicken: broken"),
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
			err := store.CreateEnvironment(&Environment{
				Name:      testEnvironment.Name,
				App:       testEnvironment.App,
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
		inApplicationName string
		inEnvName         string
		mockDeleteParam   func(t *testing.T, in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error)

		wantedError error
	}{
		"parameter is already deleted": {
			inApplicationName: "phonetool",
			inEnvName:         "test",
			mockDeleteParam: func(t *testing.T, in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error) {
				return nil, awserr.New(ssm.ErrCodeParameterNotFound, "Not found", nil)
			},
		},
		"unexpected error": {
			inApplicationName: "phonetool",
			inEnvName:         "test",
			mockDeleteParam: func(t *testing.T, in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error) {
				return nil, errors.New("some error")
			},
			wantedError: errors.New("delete environment test from application phonetool: some error"),
		},
		"successfully deleted param": {
			inApplicationName: "phonetool",
			inEnvName:         "test",
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
			err := store.DeleteEnvironment(tc.inApplicationName, tc.inEnvName)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}
