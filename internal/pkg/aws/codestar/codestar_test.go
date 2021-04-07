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

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := mocks.NewMockapi(ctrl)
		m.EXPECT().GetConnection(gomock.Any()).Return(
			&codestarconnections.GetConnectionOutput{Connection: &codestarconnections.Connection{
				ConnectionStatus: aws.String(codestarconnections.ConnectionStatusPending),
			},
			}, nil).AnyTimes()

		connection := &CodeStar{
			client: m,
		}
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

func TestCodeStar_GetConnectionARN(t *testing.T) {
	t.Run("returns wrapped error if ListConnections is unsuccessful", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		m := mocks.NewMockapi(ctrl)
		m.EXPECT().ListConnections(gomock.Any()).Return(nil, errors.New("some error"))

		connection := &CodeStar{
			client: m,
		}

		// WHEN
		ARN, err := connection.GetConnectionARN("someConnectionName")

		// THEN
		require.EqualError(t, err, "get list of connections in AWS account: some error")
		require.Equal(t, "", ARN)
	})

	t.Run("returns an error if no connections in the account match the one in the pipeline manifest", func(t *testing.T) {
		// GIVEN
		connectionName := "string cheese"
		ctrl := gomock.NewController(t)
		m := mocks.NewMockapi(ctrl)
		m.EXPECT().ListConnections(gomock.Any()).Return(
			&codestarconnections.ListConnectionsOutput{
				Connections: []*codestarconnections.Connection{
					{ConnectionName: aws.String("gouda")},
					{ConnectionName: aws.String("fontina")},
					{ConnectionName: aws.String("brie")},
				},
			}, nil)

		connection := &CodeStar{
			client: m,
		}

		// WHEN
		ARN, err := connection.GetConnectionARN(connectionName)

		// THEN
		require.Equal(t, "", ARN)
		require.EqualError(t, err, "cannot find a connectionARN associated with string cheese")
	})

	t.Run("returns a match", func(t *testing.T) {
		// GIVEN
		connectionName := "string cheese"
		ctrl := gomock.NewController(t)
		m := mocks.NewMockapi(ctrl)
		m.EXPECT().ListConnections(gomock.Any()).Return(
			&codestarconnections.ListConnectionsOutput{
				Connections: []*codestarconnections.Connection{
					{
						ConnectionName: aws.String("gouda"),
						ConnectionArn:  aws.String("notThisOne"),
					},
					{
						ConnectionName: aws.String("string cheese"),
						ConnectionArn:  aws.String("thisCheesyFakeARN"),
					},
					{
						ConnectionName: aws.String("fontina"),
						ConnectionArn:  aws.String("norThisOne"),
					},
				},
			}, nil)

		connection := &CodeStar{
			client: m,
		}

		// WHEN
		ARN, err := connection.GetConnectionARN(connectionName)

		// THEN
		require.Equal(t, "thisCheesyFakeARN", ARN)
		require.NoError(t, err)
	})

	t.Run("checks all connections and returns a match when paginated", func(t *testing.T) {
		// GIVEN
		connectionName := "string cheese"
		mockNextToken := "next"
		ctrl := gomock.NewController(t)
		m := mocks.NewMockapi(ctrl)
		m.EXPECT().ListConnections(gomock.Any()).Return(
			&codestarconnections.ListConnectionsOutput{
				Connections: []*codestarconnections.Connection{
					{
						ConnectionName: aws.String("gouda"),
						ConnectionArn:  aws.String("notThisOne"),
					},
					{
						ConnectionName: aws.String("fontina"),
						ConnectionArn:  aws.String("thisCheesyFakeARN"),
					},
				},
				NextToken: &mockNextToken,
			}, nil)
		m.EXPECT().ListConnections(&codestarconnections.ListConnectionsInput{
			NextToken: &mockNextToken,
		}).Return(
			&codestarconnections.ListConnectionsOutput{
				Connections: []*codestarconnections.Connection{
					{
						ConnectionName: aws.String("string cheese"),
						ConnectionArn:  aws.String("thisOne"),
					},
				},
			}, nil)

		connection := &CodeStar{
			client: m,
		}

		// WHEN
		ARN, err := connection.GetConnectionARN(connectionName)

		// THEN
		require.Equal(t, "thisOne", ARN)
		require.NoError(t, err)
	})
}
