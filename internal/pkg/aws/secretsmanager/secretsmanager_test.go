// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package secretsmanager wraps AWS SecretsManager API functionality.
package secretsmanager

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/copilot-cli/internal/pkg/aws/secretsmanager/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestSecretsManager_CreateSecret(t *testing.T) {
	mockSecretName := "github-token-backend-badgoose"
	mockSecretString := "H0NKH0NKH0NK"
	mockError := errors.New("mockError")
	mockOutput := &secretsmanager.CreateSecretOutput{
		ARN: aws.String("arn-goose"),
	}
	mockAwsErr := awserr.New(secretsmanager.ErrCodeResourceExistsException, "", nil)

	tests := map[string]struct {
		inSecretName   string
		inSecretString string
		callMock       func(m *mocks.Mockapi)

		expectedError error
	}{
		"should wrap error returned by CreateSecret": {
			inSecretName:   mockSecretName,
			inSecretString: mockSecretString,
			callMock: func(m *mocks.Mockapi) {
				m.EXPECT().CreateSecret(&secretsmanager.CreateSecretInput{
					Name:         aws.String(mockSecretName),
					SecretString: aws.String(mockSecretString),
					Tags:         []*secretsmanager.Tag{},
				}).Return(nil, mockError)
			},
			expectedError: fmt.Errorf("create secret %s: %w", mockSecretName, mockError),
		},

		"should return no error if secret already exists": {
			inSecretName:   mockSecretName,
			inSecretString: mockSecretString,
			callMock: func(m *mocks.Mockapi) {
				m.EXPECT().CreateSecret(&secretsmanager.CreateSecretInput{
					Name:         aws.String(mockSecretName),
					SecretString: aws.String(mockSecretString),
					Tags:         []*secretsmanager.Tag{},
				}).Return(nil, mockAwsErr)
			},
			expectedError: &ErrSecretAlreadyExists{
				secretName: mockSecretName,
				parentErr:  mockAwsErr,
			},
		},

		"should return no error if successful": {
			inSecretName:   mockSecretName,
			inSecretString: mockSecretString,
			callMock: func(m *mocks.Mockapi) {
				m.EXPECT().CreateSecret(&secretsmanager.CreateSecretInput{
					Name:         aws.String(mockSecretName),
					SecretString: aws.String(mockSecretString),
					Tags:         []*secretsmanager.Tag{},
				}).Return(mockOutput, nil)
			},
			expectedError: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSecretsManager := mocks.NewMockapi(ctrl)

			sm := SecretsManager{
				secretsManager: mockSecretsManager,
			}

			tc.callMock(mockSecretsManager)

			// WHEN
			oldSecretTags := secretTags
			defer func() { secretTags = oldSecretTags }()
			secretTags = func() []*secretsmanager.Tag {
				return []*secretsmanager.Tag{}
			}

			_, err := sm.CreateSecret(tc.inSecretName, tc.inSecretString)

			// THEN
			require.Equal(t, tc.expectedError, err)
		})
	}
}

func TestSecretsManager_DeleteSecret(t *testing.T) {
	mockSecretName := "github-token-backend-badgoose"
	mockError := errors.New("mockError")

	tests := map[string]struct {
		inSecretName string
		callMock     func(m *mocks.Mockapi)

		expectedError error
	}{
		"should wrap error returned by DeleteSecret": {
			inSecretName: mockSecretName,
			callMock: func(m *mocks.Mockapi) {
				m.EXPECT().DeleteSecret(&secretsmanager.DeleteSecretInput{
					SecretId:                   aws.String(mockSecretName),
					ForceDeleteWithoutRecovery: aws.Bool(true),
				}).Return(nil, mockError)
			},
			expectedError: fmt.Errorf("delete secret %s from secrets manager: %w", mockSecretName, mockError),
		},
		"should return no error if successful": {
			inSecretName: mockSecretName,
			callMock: func(m *mocks.Mockapi) {
				m.EXPECT().DeleteSecret(&secretsmanager.DeleteSecretInput{
					SecretId:                   aws.String(mockSecretName),
					ForceDeleteWithoutRecovery: aws.Bool(true),
				}).Return(nil, nil)
			},
			expectedError: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSecretsManager := mocks.NewMockapi(ctrl)
			sm := SecretsManager{
				secretsManager: mockSecretsManager,
			}
			tc.callMock(mockSecretsManager)

			// WHEN
			err := sm.DeleteSecret(tc.inSecretName)

			// THEN
			require.Equal(t, tc.expectedError, err)
		})
	}
}

func TestSecretsManager_DescribeSecret(t *testing.T) {
	mockTime := time.Now()
	mockSecretName := "github-token-backend-badgoose"
	mockError := errors.New("mockError")
	mockAPIOutput := &secretsmanager.DescribeSecretOutput{
		CreatedDate: aws.Time(mockTime),
		Name:        aws.String(mockSecretName),
		Tags:        []*secretsmanager.Tag{},
	}
	mockOutput := &DescribeSecretOutput{
		CreatedDate: aws.Time(mockTime),
		Name:        aws.String(mockSecretName),
		Tags:        []*secretsmanager.Tag{},
	}
	mockAwsErr := awserr.New(secretsmanager.ErrCodeResourceNotFoundException, "", nil)

	tests := map[string]struct {
		inSecretName string
		callMock     func(m *mocks.Mockapi)

		expectedResp  *DescribeSecretOutput
		expectedError error
	}{
		"should wrap error returned by DescribeSecret": {
			inSecretName: mockSecretName,
			callMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeSecret(&secretsmanager.DescribeSecretInput{
					SecretId: aws.String(mockSecretName),
				}).Return(nil, mockError)
			},
			expectedError: fmt.Errorf("describe secret %s: %w", mockSecretName, mockError),
		},

		"should return no error if secret is not found": {
			inSecretName: mockSecretName,
			callMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeSecret(&secretsmanager.DescribeSecretInput{
					SecretId: aws.String(mockSecretName),
				}).Return(nil, mockAwsErr)
			},
			expectedError: &ErrSecretNotFound{
				secretName: mockSecretName,
				parentErr:  mockAwsErr,
			},
		},

		"should return no error if successful": {
			inSecretName: mockSecretName,
			callMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeSecret(&secretsmanager.DescribeSecretInput{
					SecretId: aws.String(mockSecretName),
				}).Return(mockAPIOutput, nil)
			},
			expectedResp:  mockOutput,
			expectedError: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSecretsManager := mocks.NewMockapi(ctrl)

			sm := SecretsManager{
				secretsManager: mockSecretsManager,
			}

			tc.callMock(mockSecretsManager)

			// WHEN
			oldSecretTags := secretTags
			defer func() { secretTags = oldSecretTags }()
			secretTags = func() []*secretsmanager.Tag {
				return []*secretsmanager.Tag{}
			}

			resp, err := sm.DescribeSecret(tc.inSecretName)

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedResp, resp)
			}
		})
	}
}

func TestSecretsManager_GetSecretValue(t *testing.T) {
	tests := map[string]struct {
		secretName string
		setupMock  func(m *mocks.Mockapi)

		want      string
		wantError string
	}{
		"error": {
			secretName: "asdf",
			setupMock: func(m *mocks.Mockapi) {
				m.EXPECT().GetSecretValueWithContext(gomock.Any(), &secretsmanager.GetSecretValueInput{
					SecretId: aws.String("asdf"),
				}).Return(nil, errors.New("some error"))
			},
			wantError: `get secret "asdf" from secrets manager: some error`,
		},
		"success": {
			secretName: "asdf",
			setupMock: func(m *mocks.Mockapi) {
				m.EXPECT().GetSecretValueWithContext(gomock.Any(), &secretsmanager.GetSecretValueInput{
					SecretId: aws.String("asdf"),
				}).Return(&secretsmanager.GetSecretValueOutput{
					SecretString: aws.String("hi"),
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

			sm := SecretsManager{
				secretsManager: api,
			}

			got, err := sm.GetSecretValue(context.Background(), tc.secretName)
			if tc.wantError != "" {
				require.EqualError(t, err, tc.wantError)
			}
			require.Equal(t, tc.want, got)
		})
	}
}
