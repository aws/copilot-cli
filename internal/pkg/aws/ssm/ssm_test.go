// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ssm

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/ssm/mocks"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestSSM_PutSecret(t *testing.T) {
	const (
		mockApp = "myapp"
		mockEnv = "myenv"
	)

	testCases := map[string]struct {
		inPutSecretInput PutSecretInput

		mockClient func(*mocks.Mockapi)

		wantedOut   *PutSecretOutput
		wantedError error
	}{
		"attempt to create a new secret": {
			inPutSecretInput: PutSecretInput{
				Name:  fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv),
				Value: "super secure password",
				Tags: map[string]string{
					deploy.AppTagKey: mockApp,
					deploy.EnvTagKey: mockEnv,
				},
			},
			mockClient: func(m *mocks.Mockapi) {
				m.EXPECT().PutParameter(&ssm.PutParameterInput{
					DataType: aws.String("text"),
					Type:     aws.String("SecureString"),
					Name:     aws.String(fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv)),
					Value:    aws.String("super secure password"),
					Tags: []*ssm.Tag{
						{
							Key:   aws.String(deploy.AppTagKey),
							Value: aws.String(mockApp),
						},
						{
							Key:   aws.String(deploy.EnvTagKey),
							Value: aws.String(mockEnv),
						},
					},
				}).Return(&ssm.PutParameterOutput{
					Tier:    aws.String("Standard"),
					Version: aws.Int64(1),
				}, nil)
			},
			wantedOut: &PutSecretOutput{
				Tier:    aws.String("Standard"),
				Version: aws.Int64(1),
			},
		},
		"attempt to create a new secret even if overwrite is true": {
			inPutSecretInput: PutSecretInput{
				Name:  fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv),
				Value: "super secure password",
				Tags: map[string]string{
					deploy.AppTagKey: mockApp,
					deploy.EnvTagKey: mockEnv,
				},
				Overwrite: true,
			},
			mockClient: func(m *mocks.Mockapi) {
				m.EXPECT().PutParameter(&ssm.PutParameterInput{
					DataType: aws.String("text"),
					Type:     aws.String("SecureString"),
					Name:     aws.String(fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv)),
					Value:    aws.String("super secure password"),
					Tags: []*ssm.Tag{
						{
							Key:   aws.String(deploy.AppTagKey),
							Value: aws.String(mockApp),
						},
						{
							Key:   aws.String(deploy.EnvTagKey),
							Value: aws.String(mockEnv),
						},
					},
				}).Return(&ssm.PutParameterOutput{
					Tier:    aws.String("Standard"),
					Version: aws.Int64(1),
				}, nil)
			},
			wantedOut: &PutSecretOutput{
				Tier:    aws.String("Standard"),
				Version: aws.Int64(1),
			},
		},
		"no overwrite attempt when overwrite is false and creation fails because the secret exists": {
			inPutSecretInput: PutSecretInput{
				Name:  fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv),
				Value: "super secure password",
				Tags: map[string]string{
					deploy.AppTagKey: mockApp,
					deploy.EnvTagKey: mockEnv,
				},
			},
			mockClient: func(m *mocks.Mockapi) {
				m.EXPECT().PutParameter(&ssm.PutParameterInput{
					DataType: aws.String("text"),
					Type:     aws.String("SecureString"),
					Name:     aws.String(fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv)),
					Value:    aws.String("super secure password"),
					Tags: []*ssm.Tag{
						{
							Key:   aws.String(deploy.AppTagKey),
							Value: aws.String(mockApp),
						},
						{
							Key:   aws.String(deploy.EnvTagKey),
							Value: aws.String(mockEnv),
						},
					},
				}).Return(nil, awserr.New(ssm.ErrCodeParameterAlreadyExists, "parameter already exists", fmt.Errorf("parameter already exists")))
			},
			wantedError: &ErrParameterAlreadyExists{"/copilot/myapp/myenv/secrets/db-password"},
		},
		"no overwrite attempt when overwrite is false and creation fails because of other errors": {
			inPutSecretInput: PutSecretInput{
				Name:  fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv),
				Value: "super secure password",
				Tags: map[string]string{
					deploy.AppTagKey: mockApp,
					deploy.EnvTagKey: mockEnv,
				},
			},
			mockClient: func(m *mocks.Mockapi) {
				m.EXPECT().PutParameter(&ssm.PutParameterInput{
					DataType: aws.String("text"),
					Type:     aws.String("SecureString"),
					Name:     aws.String(fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv)),
					Value:    aws.String("super secure password"),
					Tags: []*ssm.Tag{
						{
							Key:   aws.String(deploy.AppTagKey),
							Value: aws.String(mockApp),
						},
						{
							Key:   aws.String(deploy.EnvTagKey),
							Value: aws.String(mockEnv),
						},
					},
				}).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("create parameter /copilot/myapp/myenv/secrets/db-password: some error"),
		},
		"no overwrite attempt when overwrite is true and creation fails because of other errors": {
			inPutSecretInput: PutSecretInput{
				Name:  fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv),
				Value: "super secure password",
				Tags: map[string]string{
					deploy.AppTagKey: mockApp,
					deploy.EnvTagKey: mockEnv,
				},
				Overwrite: true,
			},
			mockClient: func(m *mocks.Mockapi) {
				m.EXPECT().PutParameter(&ssm.PutParameterInput{
					DataType: aws.String("text"),
					Type:     aws.String("SecureString"),
					Name:     aws.String(fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv)),
					Value:    aws.String("super secure password"),
					Tags: []*ssm.Tag{
						{
							Key:   aws.String(deploy.AppTagKey),
							Value: aws.String(mockApp),
						},
						{
							Key:   aws.String(deploy.EnvTagKey),
							Value: aws.String(mockEnv),
						},
					},
				}).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("create parameter /copilot/myapp/myenv/secrets/db-password: some error"),
		},
		"attempt to overwrite only when overwrite is true and creation fails because the secret exists": {
			inPutSecretInput: PutSecretInput{
				Name:  fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv),
				Value: "super secure password",
				Tags: map[string]string{
					deploy.AppTagKey: mockApp,
					deploy.EnvTagKey: mockEnv,
				},
				Overwrite: true,
			},
			mockClient: func(m *mocks.Mockapi) {
				gomock.InOrder(
					m.EXPECT().PutParameter(&ssm.PutParameterInput{
						DataType: aws.String("text"),
						Type:     aws.String("SecureString"),
						Name:     aws.String(fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv)),
						Value:    aws.String("super secure password"),
						Tags: []*ssm.Tag{
							{
								Key:   aws.String(deploy.AppTagKey),
								Value: aws.String(mockApp),
							},
							{
								Key:   aws.String(deploy.EnvTagKey),
								Value: aws.String(mockEnv),
							},
						},
					}).Return(nil, awserr.New(ssm.ErrCodeParameterAlreadyExists, "parameter already exists", fmt.Errorf("parameter already exists"))),
					m.EXPECT().PutParameter(&ssm.PutParameterInput{
						DataType:  aws.String("text"),
						Type:      aws.String("SecureString"),
						Name:      aws.String(fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv)),
						Value:     aws.String("super secure password"),
						Overwrite: aws.Bool(true),
					}).Return(&ssm.PutParameterOutput{
						Tier:    aws.String("Standard"),
						Version: aws.Int64(3),
					}, nil),
					m.EXPECT().AddTagsToResource(&ssm.AddTagsToResourceInput{
						ResourceType: aws.String(ssm.ResourceTypeForTaggingParameter),
						ResourceId:   aws.String(fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv)),
						Tags: convertTags(map[string]string{
							deploy.AppTagKey: mockApp,
							deploy.EnvTagKey: mockEnv,
						}),
					}).Return(nil, nil),
				)
			},
			wantedOut: &PutSecretOutput{
				Tier:    aws.String("Standard"),
				Version: aws.Int64(3),
			},
		},
		"failed to add tags during an overwrite operation": {
			inPutSecretInput: PutSecretInput{
				Name:  fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv),
				Value: "super secure password",
				Tags: map[string]string{
					deploy.AppTagKey: mockApp,
					deploy.EnvTagKey: mockEnv,
				},
				Overwrite: true,
			},
			mockClient: func(m *mocks.Mockapi) {
				gomock.InOrder(
					m.EXPECT().PutParameter(&ssm.PutParameterInput{
						DataType: aws.String("text"),
						Type:     aws.String("SecureString"),
						Name:     aws.String(fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv)),
						Value:    aws.String("super secure password"),
						Tags: []*ssm.Tag{
							{
								Key:   aws.String(deploy.AppTagKey),
								Value: aws.String(mockApp),
							},
							{
								Key:   aws.String(deploy.EnvTagKey),
								Value: aws.String(mockEnv),
							},
						},
					}).Return(nil, awserr.New(ssm.ErrCodeParameterAlreadyExists, "parameter already exists", fmt.Errorf("parameter already exists"))),
					m.EXPECT().PutParameter(&ssm.PutParameterInput{
						DataType:  aws.String("text"),
						Type:      aws.String("SecureString"),
						Name:      aws.String(fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv)),
						Value:     aws.String("super secure password"),
						Overwrite: aws.Bool(true),
					}).Return(&ssm.PutParameterOutput{
						Tier:    aws.String("Standard"),
						Version: aws.Int64(3),
					}, nil),
					m.EXPECT().AddTagsToResource(&ssm.AddTagsToResourceInput{
						ResourceType: aws.String(ssm.ResourceTypeForTaggingParameter),
						ResourceId:   aws.String(fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv)),
						Tags: convertTags(map[string]string{
							deploy.AppTagKey: mockApp,
							deploy.EnvTagKey: mockEnv,
						}),
					}).Return(nil, errors.New("some error")),
				)
			},
			wantedError: errors.New("add tags to resource /copilot/myapp/myenv/secrets/db-password: some error"),
		},
		"fail to overwrite": {
			inPutSecretInput: PutSecretInput{
				Name:  fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv),
				Value: "super secure password",
				Tags: map[string]string{
					deploy.AppTagKey: mockApp,
					deploy.EnvTagKey: mockEnv,
				},
				Overwrite: true,
			},
			mockClient: func(m *mocks.Mockapi) {
				gomock.InOrder(
					m.EXPECT().PutParameter(&ssm.PutParameterInput{
						DataType: aws.String("text"),
						Type:     aws.String("SecureString"),
						Name:     aws.String(fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv)),
						Value:    aws.String("super secure password"),
						Tags: []*ssm.Tag{
							{
								Key:   aws.String(deploy.AppTagKey),
								Value: aws.String(mockApp),
							},
							{
								Key:   aws.String(deploy.EnvTagKey),
								Value: aws.String(mockEnv),
							},
						},
					}).Return(nil, awserr.New(ssm.ErrCodeParameterAlreadyExists, "parameter already exists", fmt.Errorf("parameter already exists"))),
					m.EXPECT().PutParameter(&ssm.PutParameterInput{
						DataType:  aws.String("text"),
						Type:      aws.String("SecureString"),
						Name:      aws.String(fmt.Sprintf("/copilot/%s/%s/secrets/db-password", mockApp, mockEnv)),
						Value:     aws.String("super secure password"),
						Overwrite: aws.Bool(true),
					}).Return(nil, errors.New("some error")),
				)
			},
			wantedError: errors.New("update parameter /copilot/myapp/myenv/secrets/db-password: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSSMClient := mocks.NewMockapi(ctrl)
			client := SSM{
				client: mockSSMClient,
			}
			tc.mockClient(mockSSMClient)

			got, err := client.PutSecret(tc.inPutSecretInput)

			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedOut, got)
			}

		})
	}
}

func TestSSM_GetSecretValue(t *testing.T) {
	tests := map[string]struct {
		secretName string
		setupMock  func(m *mocks.Mockapi)

		want      string
		wantError string
	}{
		"error": {
			secretName: "asdf",
			setupMock: func(m *mocks.Mockapi) {
				m.EXPECT().GetParameterWithContext(gomock.Any(), &ssm.GetParameterInput{
					Name:           aws.String("asdf"),
					WithDecryption: aws.Bool(true),
				}).Return(nil, errors.New("some error"))
			},
			wantError: `get parameter "asdf" from SSM: some error`,
		},
		"success": {
			secretName: "asdf",
			setupMock: func(m *mocks.Mockapi) {
				m.EXPECT().GetParameterWithContext(gomock.Any(), &ssm.GetParameterInput{
					Name:           aws.String("asdf"),
					WithDecryption: aws.Bool(true),
				}).Return(&ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Value: aws.String("hi"),
					},
				}, nil)
			},
			want: "hi",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			api := mocks.NewMockapi(ctrl)
			tc.setupMock(api)

			ssm := SSM{
				client: api,
			}

			got, err := ssm.GetSecretValue(context.Background(), tc.secretName)
			if tc.wantError != "" {
				require.EqualError(t, err, tc.wantError)
			}
			require.Equal(t, tc.want, got)
		})
	}
}
