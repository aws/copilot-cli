// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	sdkcloudformation "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestCloudFormation_renderStackChanges(t *testing.T) {
	t.Run("bubbles up create change set error", func(t *testing.T) {
		// GIVEN
		client := CloudFormation{}

		// WHEN
		in := renderStackChangesInput{
			createChangeSet: func() (string, error) {
				return "", errors.New("createChangeSet error")
			},
		}
		err := client.renderStackChanges(in)

		// THEN
		require.EqualError(t, err, "createChangeSet error")
	})
	t.Run("bubbles up DescribeChangeSet error", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := mocks.NewMockcfnClient(ctrl)
		m.EXPECT().DescribeChangeSet(gomock.Any(), gomock.Any()).Return(nil, errors.New("DescribeChangeSet error"))
		client := CloudFormation{cfnClient: m}

		// WHEN
		in := renderStackChangesInput{
			createChangeSet: func() (string, error) {
				return "", nil
			},
		}
		err := client.renderStackChanges(in)

		// THEN
		require.EqualError(t, err, "DescribeChangeSet error")
	})
	t.Run("bubbles up TemplateBody error", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := mocks.NewMockcfnClient(ctrl)
		m.EXPECT().DescribeChangeSet(gomock.Any(), gomock.Any()).Return(&cloudformation.ChangeSetDescription{}, nil)
		m.EXPECT().TemplateBody(gomock.Any()).Return("", errors.New("TemplateBody error"))
		client := CloudFormation{cfnClient: m}

		// WHEN
		in := renderStackChangesInput{
			createChangeSet: func() (string, error) {
				return "", nil
			},
		}
		err := client.renderStackChanges(in)

		// THEN
		require.EqualError(t, err, "TemplateBody error")
	})
	t.Run("bubbles up waiter error and cancels streamers and renderer on waiter error", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := mocks.NewMockcfnClient(ctrl)
		m.EXPECT().DescribeChangeSet(gomock.Any(), gomock.Any()).Return(&cloudformation.ChangeSetDescription{}, nil)
		m.EXPECT().TemplateBody(gomock.Any()).Return("", nil)
		m.EXPECT().DescribeStackEvents(gomock.Any()).Return(&sdkcloudformation.DescribeStackEventsOutput{}, nil).AnyTimes()
		client := CloudFormation{cfnClient: m}
		buf := new(strings.Builder)

		// WHEN
		in := renderStackChangesInput{
			w: buf,
			createChangeSet: func() (string, error) {
				return "", nil
			},
			waitForStack: func(ctx context.Context, s string) error {
				return errors.New("waiter error")
			},
		}
		err := client.renderStackChanges(in)

		// THEN
		require.EqualError(t, err, "waiter error")
	})
	t.Run("bubbles up streamer error and cancels waiter and renderer on streamer error", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		wantedErr := errors.New("streamer error")
		m := mocks.NewMockcfnClient(ctrl)
		m.EXPECT().DescribeChangeSet(gomock.Any(), gomock.Any()).Return(&cloudformation.ChangeSetDescription{}, nil)
		m.EXPECT().TemplateBody(gomock.Any()).Return("", nil)
		m.EXPECT().DescribeStackEvents(gomock.Any()).Return(nil, wantedErr)
		client := CloudFormation{cfnClient: m}
		buf := new(strings.Builder)

		// WHEN
		in := renderStackChangesInput{
			w: buf,
			createChangeSet: func() (string, error) {
				return "", nil
			},
			waitForStack: func(ctx context.Context, s string) error {
				select {
				case <-time.After(10 * time.Second):
					return errors.New("expected waitForStack to be canceled, instead we waited for a timeout")
				case <-ctx.Done():
					return nil
				}
			},
		}
		err := client.renderStackChanges(in)

		// THEN
		require.True(t, errors.Is(err, wantedErr), "expected streamer error to be wrapped and returned")
	})
	t.Run("renders a resource on success", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := mocks.NewMockcfnClient(ctrl)
		m.EXPECT().DescribeChangeSet("1234", "phonetool-test").Return(&cloudformation.ChangeSetDescription{
			Changes: []*sdkcloudformation.Change{
				{
					ResourceChange: &sdkcloudformation.ResourceChange{
						LogicalResourceId:  aws.String("Cluster"),
						PhysicalResourceId: aws.String("AWS::ECS::Cluster"),
					},
				},
			},
		}, nil)
		m.EXPECT().TemplateBody("phonetool-test").Return(`
Resources:
  Cluster:
    # An ECS cluster
    Type: AWS::ECS::Cluster`, nil)
		m.EXPECT().DescribeStackEvents(&sdkcloudformation.DescribeStackEventsInput{
			StackName: aws.String("phonetool-test"),
		}).Return(&sdkcloudformation.DescribeStackEventsOutput{
			StackEvents: []*sdkcloudformation.StackEvent{
				{
					EventId:            aws.String("some event id"),
					LogicalResourceId:  aws.String("Cluster"),
					PhysicalResourceId: aws.String("AWS::ECS::Cluster"),
					ResourceStatus:     aws.String("CREATE_COMPLETE"),
					Timestamp:          aws.Time(time.Now()),
				},
			},
		}, nil).AnyTimes()
		client := CloudFormation{cfnClient: m}
		buf := new(strings.Builder)

		// WHEN
		in := renderStackChangesInput{
			w:                buf,
			stackName:        "phonetool-test",
			stackDescription: "Creating phonetool-test environment.",
			createChangeSet: func() (string, error) {
				return "1234", nil
			},
			waitForStack: func(ctx context.Context, s string) error {
				return nil
			},
		}
		err := client.renderStackChanges(in)

		// THEN
		require.NoError(t, err)
		require.Contains(t, buf.String(), in.stackDescription)
		require.Contains(t, buf.String(), "An ECS cluster")
	})
}
