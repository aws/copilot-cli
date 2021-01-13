// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"io"
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

type mockFileWriter struct {
	io.Writer
}

func (m mockFileWriter) Fd() uintptr { return 0 }

func TestCloudFormation_renderStackChanges(t *testing.T) {
	t.Run("bubbles up create change set error", func(t *testing.T) {
		// GIVEN
		client := CloudFormation{}

		// WHEN
		in := renderStackChangesInput{
			w: mockFileWriter{Writer: new(strings.Builder)},
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
			w: mockFileWriter{Writer: new(strings.Builder)},
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
			w: mockFileWriter{Writer: new(strings.Builder)},
			createChangeSet: func() (string, error) {
				return "", nil
			},
		}
		err := client.renderStackChanges(in)

		// THEN
		require.EqualError(t, err, "TemplateBody error")
	})
	t.Run("bubbles up streamer error and cancels renderer", func(t *testing.T) {
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
			w: mockFileWriter{Writer: buf},
			createChangeSet: func() (string, error) {
				return "", nil
			},
		}
		err := client.renderStackChanges(in)

		// THEN
		require.True(t, errors.Is(err, wantedErr), "expected streamer error to be wrapped and returned")
	})
	t.Run("renders the stack and its resources until stack fails and return an error", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := mocks.NewMockcfnClient(ctrl)
		m.EXPECT().DescribeChangeSet(gomock.Any(), gomock.Any()).Return(&cloudformation.ChangeSetDescription{}, nil)
		m.EXPECT().TemplateBody(gomock.Any()).Return("", nil)
		m.EXPECT().DescribeStackEvents(gomock.Any()).Return(&sdkcloudformation.DescribeStackEventsOutput{
			StackEvents: []*sdkcloudformation.StackEvent{
				{
					EventId:            aws.String("2"),
					LogicalResourceId:  aws.String("phonetool-test"),
					PhysicalResourceId: aws.String("AWS::CloudFormation::Stack"),
					ResourceStatus:     aws.String("CREATE_FAILED"), // Send failure event for stack.
					Timestamp:          aws.Time(time.Now()),
				},
			},
		}, nil).AnyTimes()
		m.EXPECT().Describe("phonetool-test").Return(&cloudformation.StackDescription{
			StackStatus: aws.String("CREATE_FAILED"),
		}, nil)
		client := CloudFormation{cfnClient: m}
		buf := new(strings.Builder)

		// WHEN
		in := renderStackChangesInput{
			w:                mockFileWriter{Writer: buf},
			stackName:        "phonetool-test",
			stackDescription: "Creating phonetool-test environment.",
			createChangeSet: func() (string, error) {
				return "1234", nil
			},
		}
		err := client.renderStackChanges(in)

		// THEN
		require.EqualError(t, err, "stack phonetool-test did not complete successfully and exited with status CREATE_FAILED")
	})
	t.Run("renders the stack and its resource on success", func(t *testing.T) {
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
					EventId:            aws.String("1"),
					LogicalResourceId:  aws.String("Cluster"),
					PhysicalResourceId: aws.String("AWS::ECS::Cluster"),
					ResourceStatus:     aws.String("CREATE_COMPLETE"),
					Timestamp:          aws.Time(time.Now()),
				},
				{
					EventId:            aws.String("2"),
					LogicalResourceId:  aws.String("phonetool-test"),
					PhysicalResourceId: aws.String("AWS::CloudFormation::Stack"),
					ResourceStatus:     aws.String("CREATE_COMPLETE"),
					Timestamp:          aws.Time(time.Now()),
				},
			},
		}, nil).AnyTimes()
		m.EXPECT().Describe("phonetool-test").Return(&cloudformation.StackDescription{
			StackStatus: aws.String("CREATE_COMPLETE"),
		}, nil)
		client := CloudFormation{cfnClient: m}
		buf := new(strings.Builder)

		// WHEN
		in := renderStackChangesInput{
			w:                mockFileWriter{Writer: buf},
			stackName:        "phonetool-test",
			stackDescription: "Creating phonetool-test environment.",
			createChangeSet: func() (string, error) {
				return "1234", nil
			},
		}
		err := client.renderStackChanges(in)

		// THEN
		require.NoError(t, err)
		require.Contains(t, buf.String(), in.stackDescription)
		require.Contains(t, buf.String(), "An ECS cluster")
	})
}
