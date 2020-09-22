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

func TestStore_ListServices(t *testing.T) {
	frontendService := Workload{Name: "fe", App: "chicken", Type: "LBFargate"}
	frontendServiceString, err := marshal(frontendService)
	frontendServicePath := fmt.Sprintf(fmtSvcParamPath, frontendService.App, frontendService.Name)
	require.NoError(t, err, "Marshal svc should not fail")

	apiService := Workload{Name: "api", App: "chicken", Type: "LBFargate"}
	apiServiceString, err := marshal(apiService)
	apiServicePath := fmt.Sprintf(fmtSvcParamPath, apiService.App, apiService.Name)
	require.NoError(t, err, "Marshal svc should not fail")

	servicePath := fmt.Sprintf(rootSvcParamPath, frontendService.App)

	lastPageInPaginatedResp := false

	testCases := map[string]struct {
		mockGetParametersByPath func(t *testing.T, param *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error)

		wantedSvcs []Workload
		wantedErr  error
	}{
		"with multiple existing svcs": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, servicePath, *param.Path)
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String(frontendServicePath),
							Value: aws.String(frontendServiceString),
						},
						{
							Name:  aws.String(apiServicePath),
							Value: aws.String(apiServiceString),
						},
					},
				}, nil
			},

			wantedSvcs: []Workload{apiService, frontendService},
			wantedErr:  nil,
		},
		"with malformed json": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, servicePath, *param.Path)
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String(apiServicePath),
							Value: aws.String("oops"),
						},
					},
				}, nil
			},
			wantedErr: fmt.Errorf("read service configuration for application chicken: invalid character 'o' looking for beginning of value"),
		},
		"with SSM error": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, servicePath, *param.Path)
				return nil, fmt.Errorf("broken")
			},
			wantedErr: fmt.Errorf("list services for application chicken: broken"),
		},
		"with paginated response": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, servicePath, *param.Path)

				if !lastPageInPaginatedResp {
					lastPageInPaginatedResp = true
					return &ssm.GetParametersByPathOutput{
						Parameters: []*ssm.Parameter{
							{
								Name:  aws.String(frontendServicePath),
								Value: aws.String(frontendServiceString),
							},
						},
						NextToken: aws.String("more"),
					}, nil
				}

				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String(apiServicePath),
							Value: aws.String(apiServiceString),
						},
					},
				}, nil
			},

			wantedSvcs: []Workload{apiService, frontendService},
			wantedErr:  nil,
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
			svcPointers, err := store.ListServices("chicken")
			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				var services []Workload
				for _, s := range svcPointers {
					services = append(services, *s)
				}
				require.ElementsMatch(t, tc.wantedSvcs, services)

			}
		})
	}
}

func TestStore_GetService(t *testing.T) {
	testService := Workload{Name: "api", App: "chicken", Type: "LBFargate"}
	testServiceString, err := marshal(testService)
	testServicePath := fmt.Sprintf(fmtSvcParamPath, testService.App, testService.Name)
	require.NoError(t, err, "Marshal svc should not fail")

	testCases := map[string]struct {
		mockGetParameter func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
		wantedSvc        Workload
		wantedErr        error
	}{
		"with existing service": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testServicePath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testServicePath),
						Value: aws.String(testServiceString),
					},
				}, nil
			},
			wantedSvc: testService,
			wantedErr: nil,
		},
		"with no existing svc": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testServicePath, *param.Name)
				return nil, awserr.New(ssm.ErrCodeParameterNotFound, "bloop", nil)
			},
			wantedErr: &ErrNoSuchService{
				ApplicationName: testService.App,
				ServiceName:     testService.Name,
			},
		},
		"with malformed json": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testServicePath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testServicePath),
						Value: aws.String("oops"),
					},
				}, nil
			},
			wantedErr: fmt.Errorf("read configuration for service api in application chicken: invalid character 'o' looking for beginning of value"),
		},
		"with SSM error": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				return nil, fmt.Errorf("broken")
			},
			wantedErr: fmt.Errorf("get service api in application chicken: broken"),
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
			svc, err := store.GetService("chicken", "api")

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedSvc, *svc)
			}
		})
	}
}

func TestStore_CreateService(t *testing.T) {
	testApplication := Application{Name: "chicken", Version: "1.0"}
	testApplicationString, err := marshal(testApplication)
	testApplicationPath := fmt.Sprintf(fmtApplicationPath, testApplication.Name)
	require.NoError(t, err, "Marshal app should not fail")

	testService := Workload{Name: "api", App: testApplication.Name, Type: "LBFargate"}
	testServiceString, err := marshal(testService)
	testServicePath := fmt.Sprintf(fmtSvcParamPath, testService.App, testService.Name)
	require.NoError(t, err, "Marshal svc should not fail")

	testCases := map[string]struct {
		mockGetParameter func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
		mockPutParameter func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error)
		wantedErr        error
	}{
		"with no existing svc": {
			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				require.Equal(t, testServicePath, *param.Name)
				require.Equal(t, testServiceString, *param.Value)
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
		"with existing svc": {
			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				require.Equal(t, testServicePath, *param.Name)
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
			wantedErr: fmt.Errorf("create service api in application chicken: broken"),
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
			err := store.CreateService(&Workload{
				Name: testService.Name,
				App:  testService.App,
				Type: testService.Type})

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}

func TestDeleteService(t *testing.T) {
	mockApplicationName := "mockApplicationName"
	mockSvcName := "mockSvcName"
	mockError := errors.New("mockError")

	tests := map[string]struct {
		mockDeleteParam func(t *testing.T, in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error)

		want error
	}{
		"parameter is already deleted": {
			mockDeleteParam: func(t *testing.T, in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error) {
				return nil, awserr.New(ssm.ErrCodeParameterNotFound, "Not found", nil)
			},
		},
		"unexpected error": {
			mockDeleteParam: func(t *testing.T, in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error) {
				return nil, mockError
			},
			want: fmt.Errorf("delete service %s from application %s: %w", mockSvcName, mockApplicationName, mockError),
		},
		"successfully deleted param": {
			mockDeleteParam: func(t *testing.T, in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error) {
				wantedPath := fmt.Sprintf(fmtSvcParamPath, mockApplicationName, mockSvcName)

				require.Equal(t, wantedPath, *in.Name)

				return nil, nil
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s := &Store{
				ssmClient: &mockSSM{
					t: t,

					mockDeleteParameter: test.mockDeleteParam,
				},
			}

			got := s.DeleteService(mockApplicationName, mockSvcName)

			require.Equal(t, test.want, got)
		})
	}
}
