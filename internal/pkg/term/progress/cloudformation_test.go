// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"strings"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/stream"
	"github.com/aws/copilot-cli/internal/pkg/term/cursor"
	"github.com/stretchr/testify/require"
)

func TestStackComponent_Listen(t *testing.T) {
	t.Run("should not update status if no events are received for the logical ID", func(t *testing.T) {
		// GIVEN
		ch := make(chan stream.StackEvent)
		done := make(chan bool)
		comp := &stackComponent{
			logicalID: "phonetool-test",
			status:    "not started",
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
		require.Equal(t, "not started", comp.status)
	})
	t.Run("should update status when an event is received for stack", func(t *testing.T) {
		// GIVEN
		ch := make(chan stream.StackEvent)
		done := make(chan bool)
		comp := &stackComponent{
			logicalID: "phonetool-test",
			status:    "not started",
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
		require.Equal(t, "CREATE_COMPLETE", comp.status)
	})
}

func TestStackComponent_Render(t *testing.T) {
	// GIVEN
	comp := &stackComponent{
		description: `The environment stack "phonetool-test" contains your shared resources between services`,
		status:      "CREATE_COMPLETE",
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

	wanted := new(strings.Builder)
	cursor.EraseLine(wanted)
	wanted.WriteString("- The environment stack \"phonetool-test\" contains your shared resources between services\t[CREATE_COMPLETE]\n")
	wanted.WriteString("  - A load balancer to distribute traffic from the internet\n")
	wanted.WriteString("  - An ECS cluster to hold your services\n")
	require.Equal(t, wanted.String(), buf.String())
}

func TestRegularResourceComponent_Listen(t *testing.T) {
	t.Run("should not update status if no events are received for the logical ID", func(t *testing.T) {
		// GIVEN
		ch := make(chan stream.StackEvent)
		done := make(chan bool)
		comp := &regularResourceComponent{
			logicalID: "EnvironmentManagerRole",
			status:    "not started",
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
		require.Equal(t, "not started", comp.status)
	})
	t.Run("should update status when an event is received for the resource", func(t *testing.T) {
		// GIVEN
		ch := make(chan stream.StackEvent)
		done := make(chan bool)
		comp := &regularResourceComponent{
			logicalID: "EnvironmentManagerRole",
			status:    "not started",
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
				ResourceStatus:    "ROLLBACK_COMPLETE",
			}
			close(ch) // Close to notify that no more events will be sent.
		}()

		// THEN
		<-done // Wait for listen to exit.
		require.Equal(t, "CREATE_COMPLETE", comp.status)
	})
}

func TestRegularResourceComponent_Render(t *testing.T) {
	// GIVEN
	comp := &regularResourceComponent{
		description: "An ECS cluster to hold your services",
		status:      "CREATE_COMPLETE",
		separator:   '\t',
	}
	buf := new(strings.Builder)

	// WHEN
	nl, err := comp.Render(buf)

	// THEN
	require.NoError(t, err)
	require.Equal(t, 1, nl, "expected to be rendered as a single line component")

	wanted := new(strings.Builder)
	cursor.EraseLine(wanted)
	wanted.WriteString("- An ECS cluster to hold your services\t[CREATE_COMPLETE]\n")
	require.Equal(t, wanted.String(), buf.String())
}
