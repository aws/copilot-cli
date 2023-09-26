// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/route53"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type initAppMocks struct {
	mockRoute53Svc   *mocks.MockdomainHostedZoneGetter
	mockStore        *mocks.Mockstore
	mockPolicyLister *mocks.MockpolicyLister
	mockRoleManager  *mocks.MockroleManager
	mockProg         *mocks.Mockprogress
}

func TestInitAppOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppName      string
		inDomainName   string
		inPBPolicyName string

		mock func(m *initAppMocks)

		wantedError error
	}{
		"skip everything": {
			mock: func(m *initAppMocks) {},
		},
		"valid app name without application in SSM and without IAM adminrole": {
			inAppName: "metrics",
			mock: func(m *initAppMocks) {
				m.mockStore.EXPECT().GetApplication("metrics").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "metrics",
				})
				m.mockRoleManager.EXPECT().ListRoleTags(gomock.Eq("metrics-adminrole")).Return(nil, errors.New("role not found"))
			},
		},
		"valid app name without application in SSM and with IAM adminrole with copliot tag": {
			inAppName: "metrics",
			mock: func(m *initAppMocks) {
				m.mockStore.EXPECT().GetApplication("metrics").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "metrics",
				})
				m.mockRoleManager.EXPECT().ListRoleTags(gomock.Eq("metrics-adminrole")).Return(
					map[string]string{
						"copilot-application": "metrics",
					}, nil)
			},
			wantedError: errors.New("application named \"metrics\" already exists in another region"),
		},
		"valid app name without application in SSM and with IAM adminrole without copilot tag": {
			inAppName: "metrics",
			mock: func(m *initAppMocks) {
				m.mockStore.EXPECT().GetApplication("metrics").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "metrics",
				})
				m.mockRoleManager.EXPECT().ListRoleTags(gomock.Eq("metrics-adminrole")).Return(
					map[string]string{
						"mock-application": "metrics",
					}, nil)
			},
			wantedError: errors.New("IAM admin role \"metrics-adminrole\" already exists in this account"),
		},
		"valid app name without application in SSM and with IAM adminrole without any tag": {
			inAppName: "metrics",
			mock: func(m *initAppMocks) {
				m.mockStore.EXPECT().GetApplication("metrics").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "metrics",
				})
				m.mockRoleManager.EXPECT().ListRoleTags(gomock.Eq("metrics-adminrole")).Return(nil, nil)
			},
			wantedError: errors.New("IAM admin role \"metrics-adminrole\" already exists in this account"),
		},
		"invalid app name": {
			inAppName: "123chicken",
			mock:      func(m *initAppMocks) {},

			wantedError: fmt.Errorf("application name 123chicken is invalid: %w", errBasicNameRegexNotMatched),
		},
		"errors if application with different domain already exists": {
			inAppName:    "metrics",
			inDomainName: "badDomain.com",
			mock: func(m *initAppMocks) {
				m.mockStore.EXPECT().GetApplication("metrics").Return(&config.Application{
					Name:   "metrics",
					Domain: "domain.com",
				}, nil)
			},

			wantedError: errors.New("application named metrics already exists with a different domain name domain.com"),
		},
		"skip checking if domain name is not set": {
			inAppName:    "metrics",
			inDomainName: "",

			mock: func(m *initAppMocks) {
				m.mockStore.EXPECT().GetApplication("metrics").Return(nil, nil)
			},
		},
		"errors if failed to get application": {
			inAppName: "metrics",
			mock: func(m *initAppMocks) {
				m.mockStore.EXPECT().GetApplication("metrics").Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("get application metrics: some error"),
		},
		"invalid domain name not containing a dot": {
			inDomainName: "hello_website",
			mock: func(m *initAppMocks) {
				m.mockProg.EXPECT().Start(gomock.Any())
				m.mockProg.EXPECT().Stop(gomock.Any())
			},

			wantedError: fmt.Errorf("domain name hello_website is invalid: %w", errDomainInvalid),
		},
		"ignore unexpected errors from checking domain ownership": {
			inDomainName: "something.com",
			mock: func(m *initAppMocks) {
				m.mockProg.EXPECT().Start(gomock.Any())
				m.mockProg.EXPECT().Stop(gomock.Any()).AnyTimes()
				m.mockRoute53Svc.EXPECT().ValidateDomainOwnership("something.com").Return(errors.New("some error"))
				m.mockRoute53Svc.EXPECT().PublicDomainHostedZoneID("something.com").Return("mockHostedZoneID", nil)
			},
		},
		"wrap error from ListPolicies": {
			inPBPolicyName: "nonexistentPolicyName",
			mock: func(m *initAppMocks) {
				m.mockPolicyLister.EXPECT().ListPolicyNames().Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("list permissions boundary policies: some error"),
		},
		"invalid permissions boundary policy name": {
			inPBPolicyName: "nonexistentPolicyName",
			mock: func(m *initAppMocks) {
				m.mockPolicyLister.EXPECT().ListPolicyNames().Return(
					[]string{"existentPolicyName"}, nil)
			},
			wantedError: errors.New("IAM policy \"nonexistentPolicyName\" not found in this account"),
		},
		"invalid domain name that doesn't have a hosted zone": {
			inDomainName: "badMockDomain.com",
			mock: func(m *initAppMocks) {
				m.mockProg.EXPECT().Start(gomock.Any())
				m.mockProg.EXPECT().Stop(gomock.Any()).AnyTimes()
				m.mockRoute53Svc.EXPECT().ValidateDomainOwnership("badMockDomain.com").Return(nil)
				m.mockRoute53Svc.EXPECT().PublicDomainHostedZoneID("badMockDomain.com").Return("", &route53.ErrDomainHostedZoneNotFound{})
			},
			wantedError: fmt.Errorf("get public hosted zone ID for domain badMockDomain.com: %w", &route53.ErrDomainHostedZoneNotFound{}),
		},
		"errors if failed to validate that domain has a hosted zone": {
			inDomainName: "mockDomain.com",
			mock: func(m *initAppMocks) {
				m.mockProg.EXPECT().Start(gomock.Any())
				m.mockProg.EXPECT().Stop(gomock.Any()).AnyTimes()
				m.mockRoute53Svc.EXPECT().ValidateDomainOwnership("mockDomain.com").Return(&route53.ErrUnmatchedNSRecords{})
				m.mockRoute53Svc.EXPECT().PublicDomainHostedZoneID("mockDomain.com").Return("", errors.New("some error"))
			},
			wantedError: errors.New("get public hosted zone ID for domain mockDomain.com: some error"),
		},
		"valid": {
			inPBPolicyName: "arn:aws:iam::1234567890:policy/myPermissionsBoundaryPolicy",
			inDomainName:   "mockDomain.com",
			mock: func(m *initAppMocks) {
				m.mockProg.EXPECT().Start(`Validating ownership of "mockDomain.com"`)
				m.mockProg.EXPECT().Stop("")
				m.mockRoute53Svc.EXPECT().ValidateDomainOwnership("mockDomain.com").Return(nil)
				m.mockPolicyLister.EXPECT().ListPolicyNames().Return(
					[]string{"myPermissionsBoundaryPolicy"}, nil)
				m.mockRoute53Svc.EXPECT().PublicDomainHostedZoneID("mockDomain.com").Return("mockHostedZoneID", nil)
			},
		},
		"valid domain name containing multiple dots": {
			inDomainName: "hello.dog.com",
			mock: func(m *initAppMocks) {
				m.mockProg.EXPECT().Start(gomock.Any())
				m.mockProg.EXPECT().Stop(gomock.Any()).AnyTimes()
				m.mockRoute53Svc.EXPECT().ValidateDomainOwnership("hello.dog.com").Return(nil)
				m.mockRoute53Svc.EXPECT().PublicDomainHostedZoneID("hello.dog.com").Return("mockHostedZoneID", nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &initAppMocks{
				mockStore:        mocks.NewMockstore(ctrl),
				mockRoute53Svc:   mocks.NewMockdomainHostedZoneGetter(ctrl),
				mockPolicyLister: mocks.NewMockpolicyLister(ctrl),
				mockRoleManager:  mocks.NewMockroleManager(ctrl),
				mockProg:         mocks.NewMockprogress(ctrl),
			}
			tc.mock(m)

			opts := &initAppOpts{
				route53:        m.mockRoute53Svc,
				store:          m.mockStore,
				iam:            m.mockPolicyLister,
				iamRoleManager: m.mockRoleManager,
				prog:           m.mockProg,
				initAppVars: initAppVars{
					name:                tc.inAppName,
					domainName:          tc.inDomainName,
					permissionsBoundary: tc.inPBPolicyName,
				},
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type initAppAskMocks struct {
	store             *mocks.Mockstore
	ws                *mocks.MockwsAppManager
	existingWorkspace func() (wsAppManager, error)
	prompt            *mocks.Mockprompter
	iamRoleManager    *mocks.MockroleManager
}

func TestInitAppOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inAppName  string
		setupMocks func(m *initAppAskMocks)

		wantedAppName string
		wantedErr     string
	}{
		"if summary exists and without application in SSM and without IAM adminrole": {
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					m.ws.EXPECT().Summary().Return(&workspace.Summary{Application: "metrics", Path: "/test"}, nil)
					return m.ws, nil
				}
				m.store.EXPECT().GetApplication("metrics").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "metrics",
				})
				m.iamRoleManager.EXPECT().ListRoleTags(gomock.Eq("metrics-adminrole")).Return(nil, errors.New("role not found"))
			},
			wantedAppName: "metrics",
		},
		"if summary exists and with application in SSM and with IAM admin with copilot tag": {
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					m.ws.EXPECT().Summary().Return(&workspace.Summary{Application: "metrics", Path: "/test"}, nil)
					return m.ws, nil
				}
				m.store.EXPECT().GetApplication("metrics").Return(&config.Application{Name: "metrics"}, nil)
			},
			wantedAppName: "metrics",
		},
		"errors if summary exists and without application in SSM and with IAM adminrole with copliot tag": {
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					m.ws.EXPECT().Summary().Return(&workspace.Summary{Application: "metrics", Path: "/test"}, nil)
					return m.ws, nil
				}
				m.store.EXPECT().GetApplication("metrics").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "metrics",
				})
				m.iamRoleManager.EXPECT().ListRoleTags(gomock.Eq("metrics-adminrole")).Return(
					map[string]string{
						"copilot-application": "metrics",
					}, nil)
			},
			wantedErr: "application named \"metrics\" already exists in another region",
		},
		"errors if summary exists and without application in SSM and with IAM adminrole without copliot tag": {
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					m.ws.EXPECT().Summary().Return(&workspace.Summary{Application: "metrics", Path: "/test"}, nil)
					return m.ws, nil
				}
				m.store.EXPECT().GetApplication("metrics").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "metrics",
				})
				m.iamRoleManager.EXPECT().ListRoleTags(gomock.Eq("metrics-adminrole")).Return(
					map[string]string{
						"mock-application": "metrics",
					}, nil)
			},
			wantedErr: "IAM admin role \"metrics-adminrole\" already exists in this account",
		},
		"getting an unknown error when trying to use an existing workspace": {
			inAppName: "metrics",
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					return nil, errors.New("some error")
				}
				m.store.EXPECT().ListApplications().Times(0)
			},
			wantedErr: "some error",
		},
		"use argument if there is no workspace": {
			inAppName: "metrics",
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					return nil, &workspace.ErrWorkspaceNotFound{}
				}
				m.store.EXPECT().ListApplications().Times(0)
			},
			wantedAppName: "metrics",
		},
		"use argument if there is a workspace but no summary": {
			inAppName: "metrics",
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					m.ws.EXPECT().Summary().Return(nil, &workspace.ErrNoAssociatedApplication{})
					return m.ws, nil
				}
				m.store.EXPECT().ListApplications().Times(0)
			},
			wantedAppName: "metrics",
		},
		"getting an unknown error when trying to read the summary": {
			inAppName: "testname",
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					m.ws.EXPECT().Summary().Return(nil, errors.New("some error"))
					return m.ws, nil
				}
				m.store.EXPECT().ListApplications().Times(0)
			},
			wantedErr: "some error",
		},
		"errors if summary exists and differs from app argument": {
			inAppName: "testname",
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					m.ws.EXPECT().Summary().Return(&workspace.Summary{Application: "metrics", Path: "/test"}, nil)
					return m.ws, nil
				}
				m.store.EXPECT().ListApplications().Times(0)
			},
			wantedErr: "workspace already registered with metrics",
		},
		"return error from new app name": {
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					return nil, &workspace.ErrWorkspaceNotFound{}
				}
				m.store.EXPECT().ListApplications().Return([]*config.Application{}, nil)
				m.prompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("my error"))
				m.prompt.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "prompt get application name: my error",
		},

		"enter new app name if no existing apps and without application in SSM and without IAM adminrole": {
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					return nil, &workspace.ErrWorkspaceNotFound{}
				}
				m.store.EXPECT().ListApplications().Return([]*config.Application{}, nil)
				m.prompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
				m.store.EXPECT().GetApplication("metrics").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "metrics",
				})
				m.iamRoleManager.EXPECT().ListRoleTags(gomock.Eq("metrics-adminrole")).Return(nil, errors.New("role not found"))
				m.prompt.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedAppName: "metrics",
		},
		"enter new app name if no existing apps and without application in SSM and with IAM adminrole with copilot tag": {
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					return nil, &workspace.ErrWorkspaceNotFound{}
				}
				m.store.EXPECT().ListApplications().Return([]*config.Application{}, nil)
				m.prompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
				m.store.EXPECT().GetApplication("metrics").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "metrics",
				})
				m.iamRoleManager.EXPECT().ListRoleTags(gomock.Eq("metrics-adminrole")).Return(
					map[string]string{
						"copilot-application": "metrics",
					}, nil)
				m.prompt.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "application named \"metrics\" already exists in another region",
		},
		"enter new app name if no existing apps and without application in SSM and with IAM adminrole without copilot tag": {
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					return nil, &workspace.ErrWorkspaceNotFound{}
				}
				m.store.EXPECT().ListApplications().Return([]*config.Application{}, nil)
				m.prompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
				m.store.EXPECT().GetApplication("metrics").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "metrics",
				})
				m.iamRoleManager.EXPECT().ListRoleTags(gomock.Eq("metrics-adminrole")).Return(
					map[string]string{
						"mock-application": "metrics",
					}, nil)
				m.prompt.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "IAM admin role \"metrics-adminrole\" already exists in this account",
		},
		"enter new app name if no existing apps and without application in SSM and with IAM adminrole without any tag": {
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					return nil, &workspace.ErrWorkspaceNotFound{}
				}
				m.store.EXPECT().ListApplications().Return([]*config.Application{}, nil)
				m.prompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
				m.store.EXPECT().GetApplication("metrics").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "metrics",
				})
				m.iamRoleManager.EXPECT().ListRoleTags(gomock.Eq("metrics-adminrole")).Return(nil, nil)
				m.prompt.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "IAM admin role \"metrics-adminrole\" already exists in this account",
		},

		"return error from app selection": {
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					return nil, &workspace.ErrWorkspaceNotFound{}
				}
				m.store.EXPECT().ListApplications().Return([]*config.Application{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				m.prompt.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("my error"))
			},
			wantedErr: "prompt select application name: my error",
		},
		"use from existing apps": {
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					return nil, &workspace.ErrWorkspaceNotFound{}
				}
				m.store.EXPECT().ListApplications().Return([]*config.Application{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				m.prompt.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
			},
			wantedAppName: "metrics",
		},
		"enter new app name if user opts out of selection and application does exists in SSM and IAM admin role does not exist": {
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					return nil, &workspace.ErrWorkspaceNotFound{}
				}
				m.store.EXPECT().ListApplications().Return([]*config.Application{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				m.prompt.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				m.prompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("mock-app", nil)
				m.store.EXPECT().GetApplication("mock-app").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "mock-app",
				})
				m.iamRoleManager.EXPECT().ListRoleTags(gomock.Eq("mock-app-adminrole")).Return(nil, errors.New("role not found"))
			},
			wantedAppName: "mock-app",
		},
		"enter new app name if user opts out of selection and application does exists in SSM and IAM admin role does exist with copilot tag": {
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					return nil, &workspace.ErrWorkspaceNotFound{}
				}
				m.store.EXPECT().ListApplications().Return([]*config.Application{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				m.prompt.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				m.prompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("mock-app", nil)
				m.store.EXPECT().GetApplication("mock-app").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "mock-app",
				})
				m.iamRoleManager.EXPECT().ListRoleTags(gomock.Eq("mock-app-adminrole")).Return(
					map[string]string{
						"copilot-application": "mock-app",
					}, nil)
			},
			wantedErr: "application named \"mock-app\" already exists in another region",
		},
		"enter new app name if user opts out of selection and application does exists in SSM and IAM admin role does exist without copilot tag": {
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					return nil, &workspace.ErrWorkspaceNotFound{}
				}
				m.store.EXPECT().ListApplications().Return([]*config.Application{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				m.prompt.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				m.prompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("mock-app", nil)
				m.store.EXPECT().GetApplication("mock-app").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "mock-app",
				})
				m.iamRoleManager.EXPECT().ListRoleTags(gomock.Eq("mock-app-adminrole")).Return(
					map[string]string{
						"mock-application": "mock-app",
					}, nil)
			},
			wantedErr: "IAM admin role \"mock-app-adminrole\" already exists in this account",
		},
		"enter new app name if user opts out of selection and application does exists in SSM and IAM admin role does exist without any tag": {
			setupMocks: func(m *initAppAskMocks) {
				m.existingWorkspace = func() (wsAppManager, error) {
					return nil, &workspace.ErrWorkspaceNotFound{}
				}
				m.store.EXPECT().ListApplications().Return([]*config.Application{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				m.prompt.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				m.prompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("mock-app", nil)
				m.store.EXPECT().GetApplication("mock-app").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "mock-app",
				})
				m.iamRoleManager.EXPECT().ListRoleTags(gomock.Eq("mock-app-adminrole")).Return(nil, nil)
			},
			wantedErr: "IAM admin role \"mock-app-adminrole\" already exists in this account",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &initAppAskMocks{
				store:          mocks.NewMockstore(ctrl),
				ws:             mocks.NewMockwsAppManager(ctrl),
				prompt:         mocks.NewMockprompter(ctrl),
				iamRoleManager: mocks.NewMockroleManager(ctrl),
			}
			tc.setupMocks(m)

			opts := &initAppOpts{
				initAppVars: initAppVars{
					name: tc.inAppName,
				},
				store:          m.store,
				prompt:         m.prompt,
				iamRoleManager: m.iamRoleManager,
				isSessionFromEnvVars: func() (bool, error) {
					return false, nil
				},
				existingWorkspace: m.existingWorkspace,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedAppName, opts.name)
			}
		})
	}
}

type initAppExecuteMocks struct {
	store           *mocks.Mockstore
	ws              *mocks.MockwsAppManager
	identityService *mocks.MockidentityService
	deployer        *mocks.MockappDeployer
	progress        *mocks.Mockprogress
	newWorkspace    func(appName string) (wsAppManager, error)
}

func TestInitAppOpts_Execute(t *testing.T) {
	mockError := fmt.Errorf("error")
	testCases := map[string]struct {
		inDomainName                string
		inDomainHostedZoneID        string
		inPermissionsBoundaryPolicy string

		expectedError error
		mocking       func(m *initAppExecuteMocks)
	}{
		"with a successful call to add app": {
			inDomainName:                "amazon.com",
			inDomainHostedZoneID:        "mockID",
			inPermissionsBoundaryPolicy: "mockPolicy",

			mocking: func(m *initAppExecuteMocks) {
				m.identityService.EXPECT().Get().Return(identity.Caller{
					Account: "12345",
				}, nil)
				m.store.EXPECT().CreateApplication(&config.Application{
					AccountID:           "12345",
					Name:                "myapp",
					Domain:              "amazon.com",
					DomainHostedZoneID:  "mockID",
					PermissionsBoundary: "mockPolicy",
					Tags: map[string]string{
						"owner": "boss",
					},
				})
				m.newWorkspace = func(appName string) (wsAppManager, error) {
					return m.ws, nil
				}
				m.deployer.EXPECT().DeployApp(&deploy.CreateAppInput{
					Name:               "myapp",
					AccountID:          "12345",
					DomainName:         "amazon.com",
					DomainHostedZoneID: "mockID",
					AdditionalTags: map[string]string{
						"owner": "boss",
					},
					Version:             version.LatestTemplateVersion(),
					PermissionsBoundary: "mockPolicy",
				}).Return(nil)
			},
		},
		"should return error from workspace.Create": {
			expectedError: mockError,
			mocking: func(m *initAppExecuteMocks) {
				m.identityService.EXPECT().Get().Return(identity.Caller{
					Account: "12345",
				}, nil)
				m.newWorkspace = func(appName string) (wsAppManager, error) {
					return nil, mockError
				}
			},
		},
		"with an error while deploying myapp": {
			expectedError: mockError,
			mocking: func(m *initAppExecuteMocks) {
				m.identityService.EXPECT().Get().Return(identity.Caller{
					Account: "12345",
				}, nil)
				m.newWorkspace = func(appName string) (wsAppManager, error) {
					return m.ws, nil
				}
				m.deployer.EXPECT().DeployApp(gomock.Any()).Return(mockError)
			},
		},
		"should return error from CreateApplication": {
			expectedError: mockError,
			mocking: func(m *initAppExecuteMocks) {
				m.identityService.EXPECT().Get().Return(identity.Caller{
					Account: "12345",
				}, nil)
				m.store.EXPECT().CreateApplication(gomock.Any()).Return(mockError)
				m.newWorkspace = func(appName string) (wsAppManager, error) {
					return m.ws, nil
				}
				m.deployer.EXPECT().DeployApp(gomock.Any()).Return(nil)
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &initAppExecuteMocks{
				store:           mocks.NewMockstore(ctrl),
				ws:              mocks.NewMockwsAppManager(ctrl),
				identityService: mocks.NewMockidentityService(ctrl),
				deployer:        mocks.NewMockappDeployer(ctrl),
				progress:        mocks.NewMockprogress(ctrl),
			}
			tc.mocking(m)

			opts := &initAppOpts{
				initAppVars: initAppVars{
					name:                "myapp",
					domainName:          tc.inDomainName,
					permissionsBoundary: tc.inPermissionsBoundaryPolicy,
					resourceTags: map[string]string{
						"owner": "boss",
					},
				},
				store:    m.store,
				identity: m.identityService,
				cfn:      m.deployer,
				// ws:                 m.ws,
				prog:               m.progress,
				cachedHostedZoneID: tc.inDomainHostedZoneID,
				newWorkspace:       m.newWorkspace,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.expectedError == nil {
				require.NoError(t, err)
			} else {
				require.True(t, errors.Is(err, tc.expectedError))
			}
		})
	}
}
