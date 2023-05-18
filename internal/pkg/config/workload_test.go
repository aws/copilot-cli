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
	frontendService := Workload{Name: "fe", App: "chicken", Type: "Load Balanced Web Service"}
	frontendServiceString, err := marshal(frontendService)
	frontendServicePath := fmt.Sprintf(fmtWkldParamPath, frontendService.App, frontendService.Name)
	require.NoError(t, err, "Marshal svc should not fail")

	apiService := Workload{Name: "api", App: "chicken", Type: "Load Balanced Web Service"}
	apiServiceString, err := marshal(apiService)
	apiServicePath := fmt.Sprintf(fmtWkldParamPath, apiService.App, apiService.Name)
	require.NoError(t, err, "Marshal svc should not fail")

	servicePath := fmt.Sprintf(rootWkldParamPath, frontendService.App)

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
			wantedErr: fmt.Errorf("read service configuration for application chicken: broken"),
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
				ssm: &mockSSM{
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

func TestStore_ListWorkloads(t *testing.T) {
	frontendService := Workload{Name: "fe", App: "chicken", Type: "Load Balanced Fargate Service"}
	frontendServiceString, err := marshal(frontendService)
	frontendServicePath := fmt.Sprintf(fmtWkldParamPath, frontendService.App, frontendService.Name)
	require.NoError(t, err, "Marshal svc should not fail")

	mailerJob := Workload{Name: "mailer", App: "chicken", Type: "Scheduled Job"}
	mailerJobString, err := marshal(mailerJob)
	mailerJobPath := fmt.Sprintf(fmtWkldParamPath, mailerJob.App, mailerJob.Name)
	require.NoError(t, err, "Marshal job should not fail")
	workloadPath := fmt.Sprintf(rootWkldParamPath, mailerJob.App)

	testCases := map[string]struct {
		mockGetParametersByPath func(t *testing.T, param *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error)

		wantedWls []Workload
		wantedErr error
	}{
		"with existing workloads": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, workloadPath, *param.Path)
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String(mailerJobPath),
							Value: aws.String(mailerJobString),
						},
						{
							Name:  aws.String(frontendServicePath),
							Value: aws.String(frontendServiceString),
						},
					},
				}, nil
			},
			wantedWls: []Workload{mailerJob, frontendService},
			wantedErr: nil,
		},
		"with job": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, workloadPath, *param.Path)
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String(mailerJobPath),
							Value: aws.String(mailerJobString),
						},
					},
				}, nil
			},
			wantedWls: []Workload{mailerJob},
			wantedErr: nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			//GIVEN
			store := &Store{
				ssm: &mockSSM{
					t:                       t,
					mockGetParametersByPath: tc.mockGetParametersByPath,
				},
			}

			// WHEN
			wlPointers, err := store.ListWorkloads("chicken")
			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				var wls []Workload
				for _, w := range wlPointers {
					wls = append(wls, *w)
				}
				require.ElementsMatch(t, tc.wantedWls, wls)
			}
		})
	}
}

func TestStore_ListJobs(t *testing.T) {
	frontendService := Workload{Name: "fe", App: "chicken", Type: "Load Balanced Fargate Service"}
	frontendServiceString, err := marshal(frontendService)
	frontendServicePath := fmt.Sprintf(fmtWkldParamPath, frontendService.App, frontendService.Name)
	require.NoError(t, err, "Marshal svc should not fail")

	mailerJob := Workload{Name: "mailer", App: "chicken", Type: "Scheduled Job"}
	mailerJobString, err := marshal(mailerJob)
	mailerJobPath := fmt.Sprintf(fmtWkldParamPath, mailerJob.App, mailerJob.Name)
	require.NoError(t, err, "Marshal job should not fail")

	analyticsJob := Workload{Name: "analyzer", App: "chicken", Type: "Scheduled Job"}
	analyticsJobString, err := marshal(analyticsJob)
	analyticsJobPath := fmt.Sprintf(fmtWkldParamPath, analyticsJob.App, analyticsJob.Name)
	require.NoError(t, err, "Marshal job should not fail")

	workloadPath := fmt.Sprintf(rootWkldParamPath, mailerJob.App)

	testCases := map[string]struct {
		mockGetParametersByPath func(t *testing.T, param *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error)

		wantedJobs []Workload
		wantedErr  error
	}{
		"with existing jobs": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, workloadPath, *param.Path)
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String(mailerJobPath),
							Value: aws.String(mailerJobString),
						},
						{
							Name:  aws.String(analyticsJobPath),
							Value: aws.String(analyticsJobString),
						},
					},
				}, nil
			},
			wantedJobs: []Workload{mailerJob, analyticsJob},
			wantedErr:  nil,
		},
		"with service and job": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, workloadPath, *param.Path)
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String(mailerJobPath),
							Value: aws.String(mailerJobString),
						},
						{
							Name:  aws.String(frontendServicePath),
							Value: aws.String(frontendServiceString),
						},
					},
				}, nil
			},
			wantedJobs: []Workload{mailerJob},
			wantedErr:  nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			//GIVEN
			store := &Store{
				ssm: &mockSSM{
					t:                       t,
					mockGetParametersByPath: tc.mockGetParametersByPath,
				},
			}

			// WHEN
			jobPointers, err := store.ListJobs("chicken")
			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				var jobs []Workload
				for _, j := range jobPointers {
					jobs = append(jobs, *j)
				}
				require.ElementsMatch(t, tc.wantedJobs, jobs)
			}
		})
	}
}

func TestStore_GetService(t *testing.T) {
	testService := Workload{Name: "api", App: "chicken", Type: "Load Balanced Web Service"}
	testServiceString, err := marshal(testService)
	testServicePath := fmt.Sprintf(fmtWkldParamPath, testService.App, testService.Name)
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
			wantedErr: errors.New("couldn't find service api in the application chicken"),
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
			wantedErr: fmt.Errorf("broken"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			store := &Store{
				ssm: &mockSSM{
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

func TestStore_GetJob(t *testing.T) {
	mailerJob := Workload{Name: "mailer", App: "chicken", Type: "Scheduled Job"}
	mailerJobString, err := marshal(mailerJob)
	mailerJobPath := fmt.Sprintf(fmtWkldParamPath, mailerJob.App, mailerJob.Name)
	require.NoError(t, err, "Marshal job should not fail")

	testService := Workload{Name: "mailer", App: "chicken", Type: "Load Balanced Fargate Service"}
	testServiceString, err := marshal(testService)
	testServicePath := fmt.Sprintf(fmtWkldParamPath, testService.App, testService.Name)
	require.NoError(t, err, "Marshal svc should not fail")

	testCases := map[string]struct {
		mockGetParameter func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
		wantedJob        Workload
		wantedErr        error
	}{
		"with existing job": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, mailerJobPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(mailerJobPath),
						Value: aws.String(mailerJobString),
					},
				}, nil
			},
			wantedJob: mailerJob,
			wantedErr: nil,
		},
		"with no existing job": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, mailerJobPath, *param.Name)
				return nil, awserr.New(ssm.ErrCodeParameterNotFound, "bloop", nil)
			},
			wantedErr: errors.New("couldn't find job mailer in the application chicken"),
		},
		"with existing service": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, mailerJobPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testServicePath),
						Value: aws.String(testServiceString),
					},
				}, nil
			},
			wantedErr: &ErrNoSuchJob{
				App:  mailerJob.App,
				Name: mailerJob.Name,
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			store := &Store{
				ssm: &mockSSM{
					t:                t,
					mockGetParameter: tc.mockGetParameter,
				},
			}

			// WHEN
			job, err := store.GetJob("chicken", "mailer")

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedJob, *job)
			}
		})
	}
}

func TestStore_CreateService(t *testing.T) {
	testApplication := Application{Name: "chicken", Version: "1.0"}
	testApplicationString, err := marshal(testApplication)
	testApplicationPath := fmt.Sprintf(fmtApplicationPath, testApplication.Name)
	require.NoError(t, err, "Marshal app should not fail")

	testService := Workload{Name: "api", App: testApplication.Name, Type: "Load Balanced Fargate Service"}
	testServiceString, err := marshal(testService)
	testServicePath := fmt.Sprintf(fmtWkldParamPath, testService.App, testService.Name)
	require.NoError(t, err, "Marshal svc should not fail")
	tagsForServiceParam := []*ssm.Tag{
		{
			Key:   aws.String("copilot-application"),
			Value: aws.String(testApplication.Name),
		},
		{
			Key:   aws.String("copilot-service"),
			Value: aws.String(testService.Name),
		},
	}
	testCases := map[string]struct {
		mockGetParameter func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
		mockPutParameter func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error)
		wantedErr        error
	}{
		"with no existing svc": {
			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				require.Equal(t, testServicePath, *param.Name)
				require.Equal(t, testServiceString, *param.Value)
				require.Equal(t, tagsForServiceParam, param.Tags)
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
		},
		"with existing svc": {
			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				require.Equal(t, testServicePath, *param.Name)
				require.Equal(t, tagsForServiceParam, param.Tags)
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
				require.Equal(t, tagsForServiceParam, param.Tags)
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
				ssm: &mockSSM{
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
				wantedPath := fmt.Sprintf(fmtWkldParamPath, mockApplicationName, mockSvcName)

				require.Equal(t, wantedPath, *in.Name)

				return nil, nil
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s := &Store{
				ssm: &mockSSM{
					t: t,

					mockDeleteParameter: test.mockDeleteParam,
				},
			}

			got := s.DeleteService(mockApplicationName, mockSvcName)

			require.Equal(t, test.want, got)
		})
	}
}
