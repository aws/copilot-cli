// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package codestar

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/codestarconnections"

	"github.com/aws/copilot-cli/internal/pkg/aws/codestar/mocks"

	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/require"
)

func TestCodestar_WaitUntilStatusAvailable(t *testing.T) {
	t.Run("times out if connection status not changed to available in allotted time", func(t *testing.T) {
		// GIVEN
		ctx, cancel := context.WithDeadline(context.Background(), time.Now())
		defer cancel()
		connection := &CodeStar{}
		connectionARN := "mockConnectionARN"

		// WHEN
		err := connection.WaitUntilConnectionStatusAvailable(ctx, connectionARN)

		// THEN
		require.EqualError(t, err, "timed out waiting for connection mockConnectionARN status to change from PENDING to AVAILABLE")
	})

	t.Run("returns a wrapped error on GetConnection call failure", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := mocks.NewMockapi(ctrl)
		m.EXPECT().GetConnection(gomock.Any()).Return(nil, errors.New("some error"))

		connection := &CodeStar{
			client: m,
		}
		connectionARN := "mockConnectionARN"

		// WHEN
		err := connection.WaitUntilConnectionStatusAvailable(context.Background(), connectionARN)

		// THEN
		require.EqualError(t, err, "get connection details for mockConnectionARN: some error")
	})

	t.Run("waits until connection status is returned as 'available' and exits gracefully", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := mocks.NewMockapi(ctrl)
		connection := &CodeStar{
			client: m,
		}
		connectionARN := "mockConnectionARN"
		m.EXPECT().GetConnection(&codestarconnections.GetConnectionInput{
			ConnectionArn: aws.String(connectionARN),
		}).Return(
			&codestarconnections.GetConnectionOutput{Connection: &codestarconnections.Connection{
				ConnectionStatus: aws.String(codestarconnections.ConnectionStatusAvailable),
			},
			}, nil)

		// WHEN
		err := connection.WaitUntilConnectionStatusAvailable(context.Background(), connectionARN)

		// THEN
		require.NoError(t, err)
	})
}
