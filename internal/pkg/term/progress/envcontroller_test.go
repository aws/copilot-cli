// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"strings"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"

	"github.com/stretchr/testify/require"
)

func TestEnvControllerComponent_Render(t *testing.T) {
	t.Run("renders the environment stack component if there are any stack updates", func(t *testing.T) {
		// GIVEN
		c := &envControllerComponent{
			actionComponent: &regularResourceComponent{},
			stackComponent: &stackComponent{
				resources: []Renderer{
					&mockDynamicRenderer{content: "env-stack\n"},
					&mockDynamicRenderer{content: "alb\n"},
					&mockDynamicRenderer{content: "nat\n"},
				},
			},
		}
		buf := new(strings.Builder)

		// WHEN
		nl, err := c.Render(buf)

		// THEN
		require.NoError(t, err)
		require.Equal(t, 3, nl)
		require.Equal(t, `env-stack
alb
nat
`, buf.String())
	})
	t.Run("renders the env controller action resource if there are no environment stack updates", func(t *testing.T) {
		// GIVEN
		c := &envControllerComponent{
			actionComponent: &regularResourceComponent{
				description: "Env controller action",
				statuses: []cfnStatus{
					notStartedStackStatus,
					{
						value: cloudformation.StackStatus("CREATE_IN_PROGRESS"),
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
			},
			stackComponent: &stackComponent{
				resources: []Renderer{
					&mockDynamicRenderer{content: "env-stack\n"},
				},
			},
		}
		buf := new(strings.Builder)

		// WHEN
		nl, err := c.Render(buf)

		// THEN
		require.NoError(t, err)
		require.Equal(t, 1, nl)
		require.Equal(t, "- Env controller action\t[create in progress]\t[10.0s]\n", buf.String())
	})
}

func TestEnvControllerComponent_Done(t *testing.T) {
	t.Run("should cancel environment stack streamer if there are no stack event updates after the action is done", func(t *testing.T) {
		// GIVEN
		actionDone := make(chan struct{})
		stackDone := make(chan struct{})
		var isCanceled bool
		c := &envControllerComponent{
			cancelEnvStream: func() {
				isCanceled = true
			},
			actionComponent: &regularResourceComponent{
				done: actionDone,
			},
			stackComponent: &stackComponent{
				resources: []Renderer{
					&mockDynamicRenderer{content: "env-stack\n"}, // Only the env stack is present, no other updates.
				},
				done: stackDone,
			},
		}

		// WHEN
		done := c.Done()
		go func() {
			close(actionDone)
			close(stackDone)
		}()

		// THEN
		select {
		case <-time.After(5 * time.Second):
			require.Fail(t, "done channel is not closed, test deadline exceeded")
		case <-done:
			require.True(t, isCanceled, "stack streamer should have been canceled when component is done")
			return
		}
	})
	t.Run("should not cancel stack streamer if it is emitting events", func(t *testing.T) {
		// GIVEN
		actionDone := make(chan struct{})
		stackDone := make(chan struct{})
		c := &envControllerComponent{
			cancelEnvStream: func() {
				require.Fail(t, "stack streamer should not be canceled.")
			},
			actionComponent: &regularResourceComponent{
				done: actionDone,
			},
			stackComponent: &stackComponent{
				resources: []Renderer{
					&mockDynamicRenderer{content: "env-stack\n"},
					&mockDynamicRenderer{content: "alb\n"}, // The streamer is emitting update events.
				},
				done: stackDone,
			},
		}

		// WHEN
		done := c.Done()
		go func() {
			close(actionDone)
			close(stackDone)
		}()

		// THEN
		select {
		case <-time.After(5 * time.Second):
			require.Fail(t, "done channel is not closed, test deadline exceeded")
		case <-done:
			return
		}
	})
}
