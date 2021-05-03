// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ssm provides a client to make API requests to Amazon Systems Manager.
package ssm

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/ssm/mocks"

	"github.com/golang/mock/gomock"
)

func TestSSM_PutSecret(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockSSMClient := mocks.NewMockapi(ctrl)
		mockSSMClient.EXPECT().PutParameter(&ssm.PutParameterInput{
			DataType:  aws.String("text"),
			Type:      aws.String("SecureString"),
			Name:      aws.String("/copilot/myapp/myenv/secrets/db-password"),
			Value:     aws.String("super secure password"),
			Overwrite: aws.Bool(false),
			Tags: []*ssm.Tag{
				{
					Key:   aws.String(deploy.AppTagKey),
					Value: aws.String("myapp"),
				},
				{
					Key:   aws.String(deploy.EnvTagKey),
					Value: aws.String("myenv"),
				},
			},
		}).Return(&ssm.PutParameterOutput{
			Tier:    aws.String("Standard"),
			Version: aws.Int64(1),
		}, nil)

		client := SSM{
			client: mockSSMClient,
		}
		err := client.PutSecret(PutSecretInput{
			Name:  "/copilot/myapp/myenv/secrets/db-password",
			Value: "super secure password",
			Tags: map[string]string{
				deploy.AppTagKey: "myapp",
				deploy.EnvTagKey: "myenv",
			},
		})
		require.NoError(t, err)
	})

	t.Run("fail to put parameter", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockSSMClient := mocks.NewMockapi(ctrl)
		mockSSMClient.EXPECT().PutParameter(gomock.Any()).Return(nil, errors.New("some error"))

		client := SSM{
			client: mockSSMClient,
		}
		err := client.PutSecret(PutSecretInput{
			Name:  "/copilot/myapp/myenv/secrets/db-password",
			Value: "super secure password",
			Tags: map[string]string{
				deploy.AppTagKey: "myapp",
				deploy.EnvTagKey: "myenv",
			},
		})
		require.EqualError(t, errors.New("put parameter /copilot/myapp/myenv/secrets/db-password: some error"), err.Error())
	})
}
