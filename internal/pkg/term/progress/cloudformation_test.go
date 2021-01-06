// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"strings"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/stream"
	"github.com/stretchr/testify/require"
)

func TestStackComponent_Listen(t *testing.T) {
	t.Run("should not add status if no events are received for the logical ID", func(t *testing.T) {
		// GIVEN
		ch := make(chan stream.StackEvent)
		done := make(chan bool)
		comp := &stackComponent{
			logicalID: "phonetool-test",
			statuses:  []stackStatus{},
			stream:    ch,
		}

		// WHEN
		go func() {
			comp.Listen()
			done <- true
		}()
		go func() {
			ch <- stream.StackEvent{
				LogicalResourceID: "EnvironmentManagerRole",
				ResourceStatus:    "CREATE_COMPLETE",
			}
			ch <- stream.StackEvent{
				LogicalResourceID: "ServiceDiscoveryNamespace",
				ResourceStatus:    "CREATE_COMPLETE",
			}
			close(ch) // Close to notify that no more events will be sent.
		}()

		// THEN
		<-done // Wait for listen to exit.
		require.Empty(t, comp.statuses)
	})
	t.Run("should add status when an event is received for stack", func(t *testing.T) {
		// GIVEN
		ch := make(chan stream.StackEvent)
		done := make(chan bool)
		comp := &stackComponent{
			logicalID: "phonetool-test",
			statuses:  []stackStatus{},
			stream:    ch,
		}

		// WHEN
		go func() {
			comp.Listen()
			done <- true
		}()
		go func() {
			ch <- stream.StackEvent{
				LogicalResourceID: "EnvironmentManagerRole",
				ResourceStatus:    "CREATE_COMPLETE",
			}
			ch <- stream.StackEvent{
				LogicalResourceID: "phonetool-test",
				ResourceStatus:    "CREATE_COMPLETE",
			}
			close(ch) // Close to notify that no more events will be sent.
		}()

		// THEN
		<-done // Wait for listen to exit.
		require.ElementsMatch(t, []stackStatus{
			{
				value: "CREATE_COMPLETE",
			},
		}, comp.statuses)
	})
}

func TestStackComponent_Render(t *testing.T) {
	t.Run("renders the stack description and children renderers", func(t *testing.T) {
		// GIVEN
		comp := &stackComponent{
			description: `The environment stack "phonetool-test" contains your shared resources between services`,
			statuses: []stackStatus{
				{
					value: "CREATE_COMPLETE",
				},
			},
			children: []Renderer{
				&mockRenderer{
					content: "  - A load balancer to distribute traffic from the internet\n",
				},
				&mockRenderer{
					content: "  - An ECS cluster to hold your services\n",
				},
			},
			separator: '\t',
		}
		buf := new(strings.Builder)

		// WHEN
		nl, err := comp.Render(buf)

		// THEN
		require.NoError(t, err)
		require.Equal(t, 3, nl, "expected 3 entries to be printed to the terminal")
		require.Equal(t, "- The environment stack \"phonetool-test\" contains your shared resources between services\t[create complete]\n"+
			"  - A load balancer to distribute traffic from the internet\n"+
			"  - An ECS cluster to hold your services\n", buf.String())
	})
	t.Run("splits long failure reason into multiple lines", func(t *testing.T) {
		// GIVEN
		comp := &stackComponent{
			description: `The environment stack "phonetool-test" contains your shared resources between services`,
			statuses: []stackStatus{
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
			separator: '\t',
		}
		buf := new(strings.Builder)

		// WHEN
		nl, err := comp.Render(buf)

		// THEN
		require.NoError(t, err)
		require.Equal(t, 5, nl, "expected 3 entries to be printed to the terminal")
		require.Equal(t, "- The environment stack \"phonetool-test\" contains your shared resources between services\t[delete complete]\n"+
			"  The following resource(s) failed to create: [PublicSubnet2, Cloudforma\t\n"+
			"  tionExecutionRole, PrivateSubnet1, InternetGatewayAttachment, PublicSu\t\n"+
			"  bnet1, ServiceDiscoveryNamespace, PrivateSubnet2], EnvironmentSecurity\t\n"+
			"  Group, PublicRouteTable]. Rollback requested by user.\t\n", buf.String())
	})
	t.Run("renders multiple failure reasons", func(t *testing.T) {
		// GIVEN
		comp := &stackComponent{
			description: `The environment stack "phonetool-test" contains your shared resources between services`,
			statuses: []stackStatus{
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
			separator: '\t',
		}
		buf := new(strings.Builder)

		// WHEN
		nl, err := comp.Render(buf)

		// THEN
		require.NoError(t, err)
		require.Equal(t, 3, nl, "expected 3 entries to be printed to the terminal")
		require.Equal(t, "- The environment stack \"phonetool-test\" contains your shared resources between services\t[delete failed]\n"+
			"  Resource creation cancelled\t\n"+
			"  Resource cannot be deleted\t\n", buf.String())
	})
}

func TestRegularResourceComponent_Listen(t *testing.T) {
	t.Run("should not add status if no events are received for the logical ID", func(t *testing.T) {
		// GIVEN
		ch := make(chan stream.StackEvent)
		done := make(chan bool)
		comp := &regularResourceComponent{
			logicalID: "EnvironmentManagerRole",
			statuses:  []stackStatus{},
			stream:    ch,
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
		require.Empty(t, comp.statuses)
	})
	t.Run("should add status when an event is received for the resource", func(t *testing.T) {
		// GIVEN
		ch := make(chan stream.StackEvent)
		done := make(chan bool)
		comp := &regularResourceComponent{
			logicalID: "EnvironmentManagerRole",
			statuses:  []stackStatus{},
			stream:    ch,
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
			{
				value:  "CREATE_FAILED",
				reason: "This IAM role already exists.",
			},
		}, comp.statuses)
	})
}

func TestRegularResourceComponent_Render(t *testing.T) {
	// GIVEN
	comp := &regularResourceComponent{
		description: "An ECS cluster to hold your services",
		statuses: []stackStatus{
			{
				value: "CREATE_COMPLETE",
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
	require.Equal(t, "- An ECS cluster to hold your services\t[create complete]\n", buf.String())
}
