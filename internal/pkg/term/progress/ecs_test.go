// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"strings"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/stream"
	"github.com/stretchr/testify/require"
)

func TestRollingUpdateComponent_Listen(t *testing.T) {
	t.Run("should update deployments to latest event and respect max number of failed msgs", func(t *testing.T) {
		// GIVEN
		events := make(chan stream.ECSService)
		done := make(chan struct{})
		c := &rollingUpdateComponent{
			padding:           0,
			maxLenFailureMsgs: 2,
			stream:            events,
			done:              done,
		}

		// WHEN
		go c.Listen()
		go func() {
			events <- stream.ECSService{
				Deployments: []stream.ECSDeployment{
					{
						Status:          "PRIMARY",
						TaskDefRevision: "2",
						DesiredCount:    10,
						PendingCount:    10,
						RolloutState:    "IN_PROGRESS",
					},
					{
						Status:          "ACTIVE",
						TaskDefRevision: "1",
						DesiredCount:    10,
						RunningCount:    10,
						RolloutState:    "COMPLETED",
					},
				},
				Rollbacks: []string{"alarm triggered"},
				LatestFailureEvents: []string{"event1", "event2", "event3"},
			}
			events <- stream.ECSService{
				Deployments: []stream.ECSDeployment{
					{
						Status:          "PRIMARY",
						TaskDefRevision: "2",
						DesiredCount:    10,
						RunningCount:    10,
						RolloutState:    "COMPLETED",
					},
				},
				Rollbacks: []string{"deployment rolled back"},
				LatestFailureEvents: []string{"event4"},
			}
			close(events)
		}()

		// THEN
		<-done // Listen should have closed the channel.
		require.Equal(t, []stream.ECSDeployment{
			{
				Status:          "PRIMARY",
				TaskDefRevision: "2",
				DesiredCount:    10,
				RunningCount:    10,
				RolloutState:    "COMPLETED",
			},
		}, c.deployments, "expected only the latest deployment to be stored")
		require.Equal(t, []string{"alarm triggered", "deployment rolled back"}, c.rollbackMsgs, "expected rollback event messages to be stored")
		require.Equal(t, []string{"event3", "event4"}, c.failureMsgs, "expected max len failure msgs to be respected")
	})
}

func TestRollingUpdateComponent_Render(t *testing.T) {
	testCases := map[string]struct {
		inDeployments  []stream.ECSDeployment
		inRollbackMsgs []string
		inFailureMsgs  []string

		wantedNumLines int
		wantedOut      string
	}{
		"should render only deployments if there are no failure messages": {
			inDeployments: []stream.ECSDeployment{
				{
					Status:          "PRIMARY",
					TaskDefRevision: "2",
					DesiredCount:    10,
					RunningCount:    10,
					RolloutState:    "COMPLETED",
				},
			},

			wantedNumLines: 3,
			wantedOut: `Deployments
           Revision  Rollout      Desired  Running  Failed  Pending
  PRIMARY  2         [completed]  10       10       0       0
`,
		},
		"should render a single failure event": {
			inFailureMsgs: []string{"(service my-svc) (task 1234) failed container health checks."},

			wantedNumLines: 3,
			wantedOut: `
✘ Latest failure event
  - (service my-svc) (task 1234) failed container health checks.
`,
		},
		"should split really long failure event messages": {
			inFailureMsgs: []string{
				"(service webapp-test-frontend-Service-ss036XlczgjO) (port 80) is unhealthy in (target-group arn:aws:elasticloadbalancing:us-west-2:1111: targetgroup/aaaaaaaaaaaa) due to (reason some-error).",
			},
			wantedNumLines: 5,
			wantedOut: `
✘ Latest failure event
  - (service webapp-test-frontend-Service-ss036XlczgjO) (port 80) is unhea
    lthy in (target-group arn:aws:elasticloadbalancing:us-west-2:1111: tar
    getgroup/aaaaaaaaaaaa) due to (reason some-error).
`,
		},
		"should render multiple failure messages in reverse order": {
			inFailureMsgs: []string{
				"(service my-svc) (task 1234) failed container health checks.",
				"(service my-svc) (task 5678) failed container health checks.",
			},
			wantedNumLines: 4,
			wantedOut: `
✘ Latest 2 failure events
  - (service my-svc) (task 5678) failed container health checks.
  - (service my-svc) (task 1234) failed container health checks.
`,
		},
		"should render a rollback event": {
			inRollbackMsgs: []string{"(service my-svc) (service my-svc) deployment ecs-svc/0205655736282798388 deployment failed: alarm detected.", "(service my-svc) rolling back to deployment ecs-svc/9086637243870003494."},

			wantedNumLines: 2,
			wantedOut: `(service my-svc) (service my-svc) deployment ecs-svc/0205655736282798388 deployment failed: alarm detected.
(service my-svc) rolling back to deployment ecs-svc/9086637243870003494.
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			buf := new(strings.Builder)
			c := &rollingUpdateComponent{
				deployments: tc.inDeployments,
				rollbackMsgs: tc.inRollbackMsgs,
				failureMsgs: tc.inFailureMsgs,
			}

			// WHEN
			nl, err := c.Render(buf)

			// THEN
			require.NoError(t, err)
			require.Equal(t, tc.wantedNumLines, nl, "number of lines expected did not match")
			require.Equal(t, tc.wantedOut, buf.String(), "the content written did not match")
		})
	}
}
