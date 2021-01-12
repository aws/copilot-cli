// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"strings"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/stream"
	"github.com/stretchr/testify/require"
)

var (
	testDate = time.Date(2021, 1, 6, 0, 0, 0, 0, time.UTC)
)

type fakeClock struct {
	index        int
	wantedValues []time.Time
}

func (c *fakeClock) now() time.Time {
	t := c.wantedValues[c.index%len(c.wantedValues)]
	c.index += 1
	return t
}

func TestRegularResourceComponent_Listen(t *testing.T) {
	t.Run("should not add status if no events are received for the logical ID", func(t *testing.T) {
		// GIVEN
		ch := make(chan stream.StackEvent)
		done := make(chan bool)
		comp := &regularResourceComponent{
			logicalID: "EnvironmentManagerRole",
			statuses:  []stackStatus{notStartedStackStatus},
			stopWatch: &stopWatch{
				clock: &fakeClock{
					wantedValues: []time.Time{testDate},
				},
			},
			stream: ch,
		}

		// WHEN
		go func() {
			comp.Listen()
			done <- true
		}()
		go func() {
			ch <- stream.StackEvent{
				LogicalResourceID: "ServiceDiscoveryNamespace",
				ResourceStatus:    "CREATE_COMPLETE",
			}
			close(ch) // Close to notify that no more events will be sent.
		}()

		// THEN
		<-done // Wait for listen to exit.
		require.ElementsMatch(t, []stackStatus{notStartedStackStatus}, comp.statuses)
		_, hasStarted := comp.stopWatch.elapsed()
		require.False(t, hasStarted, "the stopwatch should not have started")
	})
	t.Run("should add status when an event is received for the resource", func(t *testing.T) {
		// GIVEN
		ch := make(chan stream.StackEvent)
		done := make(chan bool)
		comp := &regularResourceComponent{
			logicalID: "EnvironmentManagerRole",
			statuses:  []stackStatus{notStartedStackStatus},
			stopWatch: &stopWatch{
				clock: &fakeClock{
					wantedValues: []time.Time{testDate},
				},
			},
			stream: ch,
		}

		// WHEN
		go func() {
			comp.Listen()
			done <- true
		}()
		go func() {
			ch <- stream.StackEvent{
				LogicalResourceID:    "EnvironmentManagerRole",
				ResourceStatus:       "CREATE_FAILED",
				ResourceStatusReason: "This IAM role already exists.",
			}
			ch <- stream.StackEvent{
				LogicalResourceID: "phonetool-test",
				ResourceStatus:    "ROLLBACK_COMPLETE",
			}
			close(ch) // Close to notify that no more events will be sent.
		}()

		// THEN
		<-done // Wait for listen to exit.
		require.ElementsMatch(t, []stackStatus{
			notStartedStackStatus,
			{
				value:  "CREATE_FAILED",
				reason: "This IAM role already exists.",
			},
		}, comp.statuses)
		elapsed, hasStarted := comp.stopWatch.elapsed()
		require.True(t, hasStarted, "the stopwatch should have started when an event was received")
		require.Equal(t, time.Duration(0), elapsed)
	})
}

func TestRegularResourceComponent_Render(t *testing.T) {
	t.Run("renders a resource that was created succesfully immediately", func(t *testing.T) {
		// GIVEN
		comp := &regularResourceComponent{
			description: "An ECS cluster to hold your services",
			statuses: []stackStatus{
				notStartedStackStatus,
				{
					value: "CREATE_COMPLETE",
				},
			},
			stopWatch: &stopWatch{
				startTime: testDate,
				stopTime:  testDate.Add(1*time.Minute + 10*time.Second + 100*time.Millisecond),
				started:   true,
				stopped:   true,
			},
			separator: '\t',
		}
		buf := new(strings.Builder)

		// WHEN
		nl, err := comp.Render(buf)

		// THEN
		require.NoError(t, err)
		require.Equal(t, 1, nl, "expected to be rendered as a single line component")
		require.Equal(t, "- An ECS cluster to hold your services\t[create complete]\t[70.1s]\n", buf.String())
	})
	t.Run("renders a resource that is in progress", func(t *testing.T) {
		// GIVEN
		comp := &regularResourceComponent{
			description: "An ECS cluster to hold your services",
			statuses: []stackStatus{
				notStartedStackStatus,
				{
					value: "CREATE_IN_PROGRESS",
				},
			},
			stopWatch: &stopWatch{
				startTime: testDate,
				started:   true,
				clock: &fakeClock{
					wantedValues: []time.Time{testDate.Add(10 * time.Second)},
				},
			},
			separator: '\t',
		}
		buf := new(strings.Builder)

		// WHEN
		nl, err := comp.Render(buf)

		// THEN
		require.NoError(t, err)
		require.Equal(t, 1, nl, "expected to be rendered as a single line component")
		require.Equal(t, "- An ECS cluster to hold your services\t[create in progress]\t[10.0s]\n", buf.String())
	})
	t.Run("splits long failure reason into multiple lines", func(t *testing.T) {
		// GIVEN
		comp := &regularResourceComponent{
			description: `The environment stack "phonetool-test" contains your shared resources between services`,
			statuses: []stackStatus{
				notStartedStackStatus,
				{
					value: "CREATE_IN_PROGRESS",
				},
				{
					value: "CREATE_FAILED",
					reason: "The following resource(s) failed to create: [PublicSubnet2, CloudformationExecutionRole, " +
						"PrivateSubnet1, InternetGatewayAttachment, PublicSubnet1, ServiceDiscoveryNamespace," +
						" PrivateSubnet2], EnvironmentSecurityGroup, PublicRouteTable]. Rollback requested by user.",
				},
				{
					value: "DELETE_COMPLETE",
				},
			},
			stopWatch: &stopWatch{
				startTime: testDate,
				stopTime:  testDate,
				started:   true,
				stopped:   true,
			},
			separator: '\t',
		}
		buf := new(strings.Builder)

		// WHEN
		nl, err := comp.Render(buf)

		// THEN
		require.NoError(t, err)
		require.Equal(t, 5, nl, "expected 3 entries to be printed to the terminal")
		require.Equal(t, "- The environment stack \"phonetool-test\" contains your shared resources between services\t[delete complete]\t[0.0s]\n"+
			"  The following resource(s) failed to create: [PublicSubnet2, Cloudforma\t\t\n"+
			"  tionExecutionRole, PrivateSubnet1, InternetGatewayAttachment, PublicSu\t\t\n"+
			"  bnet1, ServiceDiscoveryNamespace, PrivateSubnet2], EnvironmentSecurity\t\t\n"+
			"  Group, PublicRouteTable]. Rollback requested by user.\t\t\n", buf.String())
	})
	t.Run("renders multiple failure reasons", func(t *testing.T) {
		// GIVEN
		comp := &regularResourceComponent{
			description: `The environment stack "phonetool-test" contains your shared resources between services`,
			statuses: []stackStatus{
				notStartedStackStatus,
				{
					value: "CREATE_IN_PROGRESS",
				},
				{
					value:  "CREATE_FAILED",
					reason: "Resource creation cancelled",
				},
				{
					value:  "DELETE_FAILED",
					reason: "Resource cannot be deleted",
				},
			},
			stopWatch: &stopWatch{
				startTime: testDate,
				stopTime:  testDate,
				started:   true,
				stopped:   true,
			},
			separator: '\t',
		}
		buf := new(strings.Builder)

		// WHEN
		nl, err := comp.Render(buf)

		// THEN
		require.NoError(t, err)
		require.Equal(t, 3, nl, "expected 3 entries to be printed to the terminal")
		require.Equal(t, "- The environment stack \"phonetool-test\" contains your shared resources between services\t[delete failed]\t[0.0s]\n"+
			"  Resource creation cancelled\t\t\n"+
			"  Resource cannot be deleted\t\t\n", buf.String())
	})
}
