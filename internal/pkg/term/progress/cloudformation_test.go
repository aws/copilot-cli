// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation/stackset"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"

	"github.com/aws/copilot-cli/internal/pkg/stream"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

var (
	testDate = time.Date(2021, 1, 6, 0, 0, 0, 0, time.UTC)
)

type fakeClock struct {
	index        int
	wantedValues []time.Time
	numCalls     int
}

func (c *fakeClock) now() time.Time {
	t := c.wantedValues[c.index%len(c.wantedValues)]
	c.index += 1
	c.numCalls += 1
	return t
}

func TestStackComponent_Listen(t *testing.T) {
	// GIVEN
	ch := make(chan stream.StackEvent)
	done := make(chan struct{})
	wantedRenderers := []Renderer{
		&mockDynamicRenderer{
			content: "load balancer",
		},
		&mockDynamicRenderer{
			content: "fancy role",
		},
	}
	var actualRenderers []Renderer
	comp := &stackComponent{
		cfnStream: ch,
		resourceDescriptions: map[string]string{
			"ALB":  "load balancer",
			"Role": "fancy role",
		},
		seenResources: map[string]bool{},
		done:          done,
		addRenderer: func(event stream.StackEvent, _ string) {
			if event.LogicalResourceID == "ALB" {
				actualRenderers = append(actualRenderers, wantedRenderers[0])
			} else {
				actualRenderers = append(actualRenderers, wantedRenderers[1])
			}
		},
	}

	// WHEN
	go comp.Listen()
	go func() {
		ch <- stream.StackEvent{
			LogicalResourceID: "ALB",
			ResourceStatus:    "CREATE_IN_PROGRESS",
		}
		ch <- stream.StackEvent{
			LogicalResourceID: "Role",
			ResourceStatus:    "CREATE_IN_PROGRESS",
		}
		// Should not create another renderer.
		ch <- stream.StackEvent{
			LogicalResourceID: "ALB",
			ResourceStatus:    "CREATE_COMPLETE",
		}
		// Should not create another renderer.
		ch <- stream.StackEvent{
			LogicalResourceID: "Role",
			ResourceStatus:    "CREATE_COMPLETE",
		}
		close(ch)
	}()

	// THEN
	<-done
	require.Equal(t, wantedRenderers, actualRenderers)
}

func TestStackComponent_Render(t *testing.T) {
	// GIVEN
	comp := &stackComponent{
		resources: []Renderer{
			&mockDynamicRenderer{content: "hello\n"},
			&mockDynamicRenderer{content: "world\n"},
		},
	}
	buf := new(strings.Builder)

	// WHEN
	nl, err := comp.Render(buf)

	// THEN
	require.NoError(t, err)
	require.Equal(t, 2, nl, "expected a line for each renderer")
	require.Equal(t, `hello
world
`, buf.String(), "expected each renderer to be rendered")
}

func TestStackSetComponent_Listen(t *testing.T) {
	t.Run("should update statuses and timer when an operation event is received", func(t *testing.T) {
		// GIVEN
		ch := make(chan stream.StackSetOpEvent)
		done := make(chan struct{})
		clock := &fakeClock{
			wantedValues: []time.Time{testDate, testDate.Add(10 * time.Second)},
		}
		r := &stackSetComponent{
			stream:   ch,
			done:     done,
			statuses: []cfnStatus{notStartedStackStatus},
			stopWatch: &stopWatch{
				clock: clock,
			},
		}

		// WHEN
		go r.Listen()
		go func() {
			// emulate the streamer.
			ch <- stream.StackSetOpEvent{
				Operation: stackset.Operation{
					Status: "RUNNING",
				},
			}
			ch <- stream.StackSetOpEvent{
				Operation: stackset.Operation{
					Status: "SUCCEEDED",
				},
			}
			close(ch)
		}()

		// THEN
		<-r.Done()
		require.ElementsMatch(t, []cfnStatus{
			notStartedStackStatus,
			{
				value: stackset.OpStatus("RUNNING"),
			},
			{
				value: stackset.OpStatus("SUCCEEDED"),
			},
		}, r.statuses)
		_, hasStarted := r.stopWatch.elapsed()
		require.True(t, hasStarted, "the stopwatch should have started")
	})
}

func TestStackSetComponent_Render(t *testing.T) {
	t.Run("renders a stack set operation that succeeded", func(t *testing.T) {
		// GIVEN
		r := &stackSetComponent{
			title: "Update stack set demo-infrastructure",
			statuses: []cfnStatus{
				notStartedStackStatus,
				{
					value: stackset.OpStatus("SUCCEEDED"),
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
		nl, err := r.Render(buf)

		// THEN
		require.NoError(t, err)
		require.Equal(t, 1, nl, "expected to be rendered as a single line component")
		require.Equal(t, "- Update stack set demo-infrastructure\t[succeeded]\t[70.1s]\n", buf.String())
	})
	t.Run("renders a stack set operation that failed", func(t *testing.T) {
		// GIVEN
		r := &stackSetComponent{
			title: "Update stack set demo-infrastructure",
			statuses: []cfnStatus{
				notStartedStackStatus,
				{
					value: stackset.OpStatus("RUNNING"),
				},
				{
					value:  stackset.OpStatus("FAILED"),
					reason: "The Operation 1 has failed to create",
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
		nl, err := r.Render(buf)

		// THEN
		require.NoError(t, err)
		require.Equal(t, 2, nl, "expected 2 entries to be printed to the terminal")
		require.Equal(t, "- Update stack set demo-infrastructure\t[failed]\t[0.0s]\n"+
			"  The Operation 1 has failed to create\t\t\n", buf.String())
	})
}

func TestRegularResourceComponent_Listen(t *testing.T) {
	t.Run("should not add status if no events are received for the logical ID", func(t *testing.T) {
		// GIVEN
		ch := make(chan stream.StackEvent)
		done := make(chan struct{})
		comp := &regularResourceComponent{
			logicalID: "EnvironmentManagerRole",
			statuses:  []cfnStatus{notStartedStackStatus},
			stopWatch: &stopWatch{
				clock: &fakeClock{
					wantedValues: []time.Time{testDate},
				},
			},
			stream: ch,
			done:   done,
		}

		// WHEN
		go comp.Listen()
		go func() {
			ch <- stream.StackEvent{
				LogicalResourceID: "ServiceDiscoveryNamespace",
				ResourceStatus:    "CREATE_COMPLETE",
			}
			close(ch) // Close to notify that no more events will be sent.
		}()

		// THEN
		<-done // Wait for listen to exit.
		require.ElementsMatch(t, []cfnStatus{notStartedStackStatus}, comp.statuses)
		_, hasStarted := comp.stopWatch.elapsed()
		require.False(t, hasStarted, "the stopwatch should not have started")
	})
	t.Run("should add status when an event is received for the resource", func(t *testing.T) {
		// GIVEN
		ch := make(chan stream.StackEvent)
		done := make(chan struct{})
		comp := &regularResourceComponent{
			logicalID: "EnvironmentManagerRole",
			statuses:  []cfnStatus{notStartedStackStatus},
			stopWatch: &stopWatch{
				clock: &fakeClock{
					wantedValues: []time.Time{testDate},
				},
			},
			stream: ch,
			done:   done,
		}

		// WHEN
		go comp.Listen()
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
		require.ElementsMatch(t, []cfnStatus{
			notStartedStackStatus,
			{
				value:  cloudformation.StackStatus("CREATE_FAILED"),
				reason: "This IAM role already exists.",
			},
		}, comp.statuses)
		elapsed, hasStarted := comp.stopWatch.elapsed()
		require.True(t, hasStarted, "the stopwatch should have started when an event was received")
		require.Equal(t, time.Duration(0), elapsed)
	})
	t.Run("should keep timer running if multiple in progress events are received", func(t *testing.T) {
		// GIVEN
		ch := make(chan stream.StackEvent)
		done := make(chan struct{})
		fc := &fakeClock{
			wantedValues: []time.Time{testDate, testDate.Add(10 * time.Second)},
		}
		comp := &regularResourceComponent{
			logicalID: "EnvironmentManagerRole",
			statuses:  []cfnStatus{notStartedStackStatus},
			stopWatch: &stopWatch{
				clock: fc,
			},
			stream: ch,
			done:   done,
		}

		// WHEN
		go comp.Listen()
		go func() {
			ch <- stream.StackEvent{
				LogicalResourceID: "EnvironmentManagerRole",
				ResourceStatus:    "CREATE_IN_PROGRESS",
			}
			ch <- stream.StackEvent{
				LogicalResourceID: "EnvironmentManagerRole",
				ResourceStatus:    "CREATE_IN_PROGRESS",
			}
			close(ch) // Close to notify that no more events will be sent.
		}()

		// THEN
		<-done // Wait for listen to exit.
		_, hasStarted := comp.stopWatch.elapsed()
		require.True(t, hasStarted, "the stopwatch should have started when an event was received")
		require.Equal(t, 2, fc.numCalls, "stop watch should retrieve the current time only twice, start should not be called twice")
	})
}

func TestRegularResourceComponent_Render(t *testing.T) {
	t.Run("renders a resource that was created Successfully immediately", func(t *testing.T) {
		// GIVEN
		comp := &regularResourceComponent{
			description: "An ECS cluster to hold your services",
			statuses: []cfnStatus{
				notStartedStackStatus,
				{
					value: cloudformation.StackStatus("CREATE_COMPLETE"),
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
			statuses: []cfnStatus{
				notStartedStackStatus,
				{
					value: cloudformation.StackStatus("CREATE_IN_PROGRESS"),
				},
				{
					value: cloudformation.StackStatus("CREATE_FAILED"),
					reason: "The following resource(s) failed to create: [PublicSubnet2, CloudformationExecutionRole, " +
						"PrivateSubnet1, InternetGatewayAttachment, PublicSubnet1, ServiceDiscoveryNamespace," +
						" PrivateSubnet2], EnvironmentSecurityGroup, PublicRouteTable]. Rollback requested by user.",
				},
				{
					value: cloudformation.StackStatus("DELETE_COMPLETE"),
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
			statuses: []cfnStatus{
				notStartedStackStatus,
				{
					value: cloudformation.StackStatus("CREATE_IN_PROGRESS"),
				},
				{
					value:  cloudformation.StackStatus("CREATE_FAILED"),
					reason: "Resource creation cancelled",
				},
				{
					value:  cloudformation.StackStatus("DELETE_FAILED"),
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

func TestEcsServiceResourceComponent_Listen(t *testing.T) {
	t.Run("should create a deployment renderer if the service goes into in progress", func(t *testing.T) {
		// GIVEN
		ch := make(chan stream.StackEvent)
		deployDone := make(chan struct{})
		resourceDone := make(chan struct{})
		c := &ecsServiceResourceComponent{
			cfnStream: ch,
			logicalID: "Service",
			group:     new(errgroup.Group),
			ctx:       context.Background(),
			done:      make(chan struct{}),
			resourceRenderer: &mockDynamicRenderer{
				done: resourceDone,
			},
			newDeploymentRender: func(s string, t time.Time) DynamicRenderer {
				return &mockDynamicRenderer{
					done: deployDone,
				}
			},
		}

		// WHEN
		go c.Listen()
		go func() {
			ch <- stream.StackEvent{
				LogicalResourceID:  "Service",
				PhysicalResourceID: "",
				ResourceStatus:     "CREATE_IN_PROGRESS",
			}
			ch <- stream.StackEvent{
				LogicalResourceID:  "Service",
				PhysicalResourceID: "arn:aws:ecs:us-west-2:1111:service/webapp-test-Cluster/webapp-test-frontend",
				ResourceStatus:     "CREATE_IN_PROGRESS",
			}
			// Close channels to notify that the ecs service deployment is done.
			close(deployDone)
			close(resourceDone)
			close(ch)
		}()

		// THEN
		<-c.done // Wait for listen to exit.
		require.NotNil(t, c.deploymentRenderer, "expected the deployment renderer to be initialized")
	})
	t.Run("should not create a deployment renderer if the service never goes in create or update in progress", func(t *testing.T) {
		// GIVEN
		ch := make(chan stream.StackEvent)
		deployDone := make(chan struct{})
		resourceDone := make(chan struct{})
		c := &ecsServiceResourceComponent{
			cfnStream: ch,
			logicalID: "Service",
			group:     new(errgroup.Group),
			ctx:       context.Background(),
			done:      make(chan struct{}),
			resourceRenderer: &mockDynamicRenderer{
				done: resourceDone,
			},
			newDeploymentRender: func(s string, t time.Time) DynamicRenderer {
				return &mockDynamicRenderer{
					done: deployDone,
				}
			},
		}

		// WHEN
		go c.Listen()
		go func() {
			ch <- stream.StackEvent{
				LogicalResourceID:  "Service",
				PhysicalResourceID: "arn:aws:ecs:us-west-2:1111:service/webapp-test-Cluster/webapp-test-frontend",
				ResourceStatus:     "CREATE_COMPLETE",
			}
			ch <- stream.StackEvent{
				LogicalResourceID:  "Service",
				PhysicalResourceID: "arn:aws:ecs:us-west-2:1111:service/webapp-test-Cluster/webapp-test-frontend",
				ResourceStatus:     "DELETE_IN_PROGRESS",
			}
			ch <- stream.StackEvent{
				LogicalResourceID:  "Service",
				PhysicalResourceID: "arn:aws:ecs:us-west-2:1111:service/webapp-test-Cluster/webapp-test-frontend",
				ResourceStatus:     "DELETE_COMPLETE",
			}
			// Close channels to notify that the ecs service deployment is done.
			close(deployDone)
			close(resourceDone)
			close(ch)
		}()

		// THEN
		<-c.done // Wait for listen to exit.
		require.Nil(t, c.deploymentRenderer, "expected the deployment renderer to be nil")
	})
}

func TestEcsServiceResourceComponent_Render(t *testing.T) {
	t.Run("renders only the resource renderer if there is no deployment in progress", func(t *testing.T) {
		// GIVEN
		buf := new(strings.Builder)
		c := &ecsServiceResourceComponent{
			resourceRenderer: &mockDynamicRenderer{
				content: "resource\n",
			},
		}

		// WHEN
		nl, err := c.Render(buf)

		// THEN
		require.Nil(t, err)
		require.Equal(t, 1, nl)
		require.Equal(t, "resource\n", buf.String())
	})
	t.Run("renders both resource and deployment if deployment in progress", func(t *testing.T) {
		// GIVEN
		buf := new(strings.Builder)
		c := &ecsServiceResourceComponent{
			resourceRenderer: &mockDynamicRenderer{
				content: "resource\n",
			},
			deploymentRenderer: &mockDynamicRenderer{
				content: "deployment\n",
			},
		}

		// WHEN
		nl, err := c.Render(buf)

		// THEN
		require.Nil(t, err)
		require.Equal(t, 2, nl)
		require.Equal(t, "resource\n"+
			"deployment\t\t\n", buf.String())
	})
}
