// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/route53domains"
	"github.com/aws/aws-sdk-go/service/route53domains/route53domainsiface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/stretchr/testify/require"
)

// APP TEST

func TestStore_ListApplications(t *testing.T) {
	frontendApplication := archer.Application{Name: "fe", Project: "chicken", Type: "LBFargate"}
	frontendApplicationString, err := marshal(frontendApplication)
	frontendApplicationPath := fmt.Sprintf(fmtAppParamPath, frontendApplication.Project, frontendApplication.Name)
	require.NoError(t, err, "Marshal app should not fail")

	apiApplication := archer.Application{Name: "api", Project: "chicken", Type: "LBFargate"}
	apiApplicationString, err := marshal(apiApplication)
	apiApplicationPath := fmt.Sprintf(fmtAppParamPath, apiApplication.Project, apiApplication.Name)
	require.NoError(t, err, "Marshal app should not fail")

	applicationPath := fmt.Sprintf(rootAppParamPath, frontendApplication.Project)

	lastPageInPaginatedResp := false

	testCases := map[string]struct {
		mockGetParametersByPath func(t *testing.T, param *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error)

		wantedApps []archer.Application
		wantedErr  error
	}{
		"with multiple existing apps": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, applicationPath, *param.Path)
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String(frontendApplicationPath),
							Value: aws.String(frontendApplicationString),
						},
						{
							Name:  aws.String(apiApplicationPath),
							Value: aws.String(apiApplicationString),
						},
					},
				}, nil
			},

			wantedApps: []archer.Application{apiApplication, frontendApplication},
			wantedErr:  nil,
		},
		"with malformed json": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, applicationPath, *param.Path)
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String(apiApplicationPath),
							Value: aws.String("oops"),
						},
					},
				}, nil
			},
			wantedErr: fmt.Errorf("read application details for project chicken: invalid character 'o' looking for beginning of value"),
		},
		"with SSM error": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, applicationPath, *param.Path)
				return nil, fmt.Errorf("broken")
			},
			wantedErr: fmt.Errorf("list applications for project chicken: broken"),
		},
		"with paginated response": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, applicationPath, *param.Path)

				if !lastPageInPaginatedResp {
					lastPageInPaginatedResp = true
					return &ssm.GetParametersByPathOutput{
						Parameters: []*ssm.Parameter{
							{
								Name:  aws.String(frontendApplicationPath),
								Value: aws.String(frontendApplicationString),
							},
						},
						NextToken: aws.String("more"),
					}, nil
				}

				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name:  aws.String(apiApplicationPath),
							Value: aws.String(apiApplicationString),
						},
					},
				}, nil
			},

			wantedApps: []archer.Application{apiApplication, frontendApplication},
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
			appPointers, err := store.ListApplications("chicken")
			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				var applications []archer.Application
				for _, a := range appPointers {
					applications = append(applications, *a)
				}
				require.ElementsMatch(t, tc.wantedApps, applications)

			}
		})
	}
}

func TestStore_GetApp(t *testing.T) {
	testApplication := archer.Application{Name: "api", Project: "chicken", Type: "LBFargate"}
	testApplicationString, err := marshal(testApplication)
	testApplicationPath := fmt.Sprintf(fmtAppParamPath, testApplication.Project, testApplication.Name)
	require.NoError(t, err, "Marshal app should not fail")

	testCases := map[string]struct {
		mockGetParameter func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
		wantedApp        archer.Application
		wantedErr        error
	}{
		"with existing application": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testApplicationPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testApplicationPath),
						Value: aws.String(testApplicationString),
					},
				}, nil
			},
			wantedApp: testApplication,
			wantedErr: nil,
		},
		"with no existing app": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testApplicationPath, *param.Name)
				return nil, awserr.New(ssm.ErrCodeParameterNotFound, "bloop", nil)
			},
			wantedErr: &ErrNoSuchApplication{
				ProjectName:     testApplication.Project,
				ApplicationName: testApplication.Name,
			},
		},
		"with malformed json": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				require.Equal(t, testApplicationPath, *param.Name)
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String(testApplicationPath),
						Value: aws.String("oops"),
					},
				}, nil
			},
			wantedErr: fmt.Errorf("read details for application api in project chicken: invalid character 'o' looking for beginning of value"),
		},
		"with SSM error": {
			mockGetParameter: func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
				return nil, fmt.Errorf("broken")
			},
			wantedErr: fmt.Errorf("get application api in project chicken: broken"),
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
			app, err := store.GetApplication("chicken", "api")

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedApp, *app)
			}
		})
	}
}

func TestStore_CreateApplication(t *testing.T) {
	testProject := archer.Project{Name: "chicken", Version: "1.0"}
	testProjectString, err := marshal(testProject)
	testProjectPath := fmt.Sprintf(fmtProjectPath, testProject.Name)
	require.NoError(t, err, "Marshal project should not fail")

	testApplication := archer.Application{Name: "api", Project: testProject.Name, Type: "LBFargate"}
	testApplicationString, err := marshal(testApplication)
	testApplicationPath := fmt.Sprintf(fmtAppParamPath, testApplication.Project, testApplication.Name)
	require.NoError(t, err, "Marshal app should not fail")

	testCases := map[string]struct {
		mockGetParameter func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
		mockPutParameter func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error)
		wantedErr        error
	}{
		"with no existing app": {
			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				require.Equal(t, testApplicationPath, *param.Name)
				require.Equal(t, testApplicationString, *param.Value)
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
		"with existing app": {
			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				require.Equal(t, testApplicationPath, *param.Name)
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
			wantedErr: &ErrApplicationAlreadyExists{
				ApplicationName: testApplication.Name,
				ProjectName:     testApplication.Project,
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
			wantedErr: fmt.Errorf("create application api in project chicken: broken"),
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
			err := store.CreateApplication(&archer.Application{
				Name:    testApplication.Name,
				Project: testApplication.Project,
				Type:    testApplication.Type})

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

type mockIdentityService struct {
	mockIdentityServiceGet func() (identity.Caller, error)
}

func (m mockIdentityService) Get() (identity.Caller, error) {
	return m.mockIdentityServiceGet()
}

type mockRoute53Domains struct {
	route53domainsiface.Route53DomainsAPI
	t                    *testing.T
	mockGetDomainDetails func(t *testing.T, in *route53domains.GetDomainDetailInput) (*route53domains.GetDomainDetailOutput, error)
}

func (m *mockRoute53Domains) GetDomainDetail(in *route53domains.GetDomainDetailInput) (*route53domains.GetDomainDetailOutput, error) {
	return m.mockGetDomainDetails(m.t, in)
}
