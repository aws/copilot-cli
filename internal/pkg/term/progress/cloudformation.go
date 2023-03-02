// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/stream"
	"golang.org/x/sync/errgroup"
)

// StackSubscriber is the interface to subscribe to a CloudFormation stack event stream.
type StackSubscriber interface {
	Subscribe() <-chan stream.StackEvent
}

// StackSetSubscriber is the interface to subscribe channels to a CloudFormation stack set event stream.
type StackSetSubscriber interface {
	Subscribe() <-chan stream.StackSetOpEvent
}

// ResourceRendererOpts is optional configuration for a listening CloudFormation resource renderer.
type ResourceRendererOpts struct {
	StartEvent *stream.StackEvent // Specify the starting event for the resource instead of "[not started]".
	RenderOpts RenderOptions
}

// ECSServiceRendererCfg holds required configuration for initializing an ECS service renderer.
type ECSServiceRendererCfg struct {
	Streamer    StackSubscriber
	ECSClient   stream.ECSServiceDescriber
	CWClient    stream.CloudWatchDescriber
	LogicalID   string
	Description string
}

// ECSServiceRendererOpts holds optional configuration for a listening ECS service renderer.
type ECSServiceRendererOpts struct {
	Group      *errgroup.Group
	Ctx        context.Context
	RenderOpts RenderOptions
}

// ListeningChangeSetRenderer returns a component that listens for CloudFormation
// resource events from a stack mutated with a changeSet until the streamer stops.
func ListeningChangeSetRenderer(streamer StackSubscriber, stackName, description string, changes []Renderer, opts RenderOptions) DynamicRenderer {
	return &dynamicTreeComponent{
		Root: ListeningResourceRenderer(streamer, stackName, description, ResourceRendererOpts{
			RenderOpts: opts,
		}),
		Children: changes,
	}
}

// ListeningStackRenderer returns a component that listens for CloudFormation resource events
// from a stack mutated with CreateStack or UpdateStack until the stack is completed.
func ListeningStackRenderer(streamer StackSubscriber, stackName, description string, resourceDescriptions map[string]string, opts RenderOptions) DynamicRenderer {
	return listeningStackComponent(streamer, stackName, description, resourceDescriptions, opts)
}

// ListeningStackSetRenderer renders a component that listens for CloudFormation stack set events until the streamer stops.
func ListeningStackSetRenderer(streamer StackSetSubscriber, title string, opts RenderOptions) DynamicRenderer {
	comp := &stackSetComponent{
		stream:    streamer.Subscribe(),
		done:      make(chan struct{}),
		style:     opts,
		title:     title,
		separator: '\t',
		statuses:  []cfnStatus{notStartedStackStatus},
		stopWatch: newStopWatch(),
	}
	go comp.Listen()
	return comp
}

// ListeningResourceRenderer returns a tab-separated component that listens for
// CloudFormation stack events for a particular resource.
func ListeningResourceRenderer(streamer StackSubscriber, logicalID, description string, opts ResourceRendererOpts) DynamicRenderer {
	return listeningResourceComponent(streamer, logicalID, description, opts)
}

// ListeningECSServiceResourceRenderer is a ListeningResourceRenderer for the ECS service cloudformation resource
// and a ListeningRollingUpdateRenderer to render deployments.
func ListeningECSServiceResourceRenderer(cfg ECSServiceRendererCfg, opts ECSServiceRendererOpts) DynamicRenderer {
	g := new(errgroup.Group)
	ctx := context.Background()
	if opts.Group != nil {
		g = opts.Group
	}
	if opts.Ctx != nil {
		ctx = opts.Ctx
	}
	comp := &ecsServiceResourceComponent{
		cfnStream:    cfg.Streamer.Subscribe(),
		ecsDescriber: cfg.ECSClient,
		cwDescriber:  cfg.CWClient,
		logicalID:    cfg.LogicalID,
		group:        g,
		ctx:          ctx,
		renderOpts:   opts.RenderOpts,
		resourceRenderer: ListeningResourceRenderer(cfg.Streamer, cfg.LogicalID, cfg.Description, ResourceRendererOpts{
			RenderOpts: opts.RenderOpts,
		}),
		done: make(chan struct{}),
	}
	comp.newDeploymentRender = comp.newListeningRollingUpdateRenderer
	go comp.Listen()
	return comp
}

// regularResourceComponent can display a simple CloudFormation stack resource event.
type regularResourceComponent struct {
	logicalID   string      // The LogicalID defined in the template for the resource.
	description string      // The human friendly explanation of the resource.
	statuses    []cfnStatus // In-order history of the CloudFormation status of the resource throughout the deployment.
	stopWatch   *stopWatch  // Timer to measure how long the operation takes to complete.

	padding   int  // Leading spaces before rendering the resource.
	separator rune // Character used to separate columns of text.

	stream <-chan stream.StackEvent
	done   chan struct{}
	mu     sync.Mutex
}

func listeningResourceComponent(streamer StackSubscriber, logicalID, description string, opts ResourceRendererOpts) *regularResourceComponent {
	comp := &regularResourceComponent{
		logicalID:   logicalID,
		description: description,
		statuses:    []cfnStatus{notStartedStackStatus},
		stopWatch:   newStopWatch(),
		stream:      streamer.Subscribe(),
		done:        make(chan struct{}),
		padding:     opts.RenderOpts.Padding,
		separator:   '\t',
	}
	if startEvent := opts.StartEvent; startEvent != nil {
		updateComponentStatus(&comp.mu, &comp.statuses, cfnStatus{
			value:  cloudformation.StackStatus(startEvent.ResourceStatus),
			reason: startEvent.ResourceStatusReason,
		})
		updateComponentTimer(&comp.mu, comp.statuses, comp.stopWatch)
	}
	go comp.Listen()
	return comp
}

// Listen updates the resource's status if a CloudFormation stack resource event is received.
func (c *regularResourceComponent) Listen() {
	for ev := range c.stream {
		if c.logicalID != ev.LogicalResourceID {
			continue
		}
		updateComponentStatus(&c.mu, &c.statuses, cfnStatus{
			value:  cloudformation.StackStatus(ev.ResourceStatus),
			reason: ev.ResourceStatusReason,
		})
		updateComponentTimer(&c.mu, c.statuses, c.stopWatch)
	}
	close(c.done) // No more events will be processed.
}

// Render prints the resource as a singleLineComponent and returns the number of lines written and the error if any.
func (c *regularResourceComponent) Render(out io.Writer) (numLines int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	components := cfnLineItemComponents(c.description, c.separator, c.statuses, c.stopWatch, c.padding)
	return renderComponents(out, components)
}

// Done returns a channel that's closed when there are no more events to Listen.
func (c *regularResourceComponent) Done() <-chan struct{} {
	return c.done
}

// stackComponent is a DynamicRenderer that can display CloudFormation stack events as they stream in.
type stackComponent struct {
	// Required inputs.
	cfnStream            <-chan stream.StackEvent
	stack                StackSubscriber
	resourceDescriptions map[string]string

	// Optional inputs.
	renderOpts RenderOptions

	// Sub-components.
	resources     []Renderer
	seenResources map[string]bool
	done          chan struct{}
	mu            sync.Mutex

	// Replaced in tests.
	addRenderer func(stream.StackEvent, string)
}

func listeningStackComponent(streamer StackSubscriber, stackName, description string, resourceDescriptions map[string]string, opts RenderOptions) *stackComponent {
	comp := &stackComponent{
		cfnStream:            streamer.Subscribe(),
		stack:                streamer,
		resourceDescriptions: resourceDescriptions,
		renderOpts:           opts,
		resources: []Renderer{
			// Add the stack as a resource to track.
			ListeningResourceRenderer(streamer, stackName, description, ResourceRendererOpts{
				RenderOpts: opts,
			}),
		},
		seenResources: map[string]bool{
			stackName: true,
		},
		done: make(chan struct{}),
	}
	comp.addRenderer = comp.addResourceRenderer
	go comp.Listen()
	return comp
}

// Listen consumes stack events from the stream.
// On new resource events, if the resource's LogicalID has a description
// then the resource is added to the list of sub-components to render.
func (c *stackComponent) Listen() {
	for ev := range c.cfnStream {
		logicalID := ev.LogicalResourceID
		if _, ok := c.seenResources[logicalID]; ok {
			continue
		}
		c.seenResources[logicalID] = true

		description, ok := c.resourceDescriptions[logicalID]
		if !ok {
			continue
		}
		c.addRenderer(ev, description)
	}
	// Close the done channel once all the renderers are done listening.
	for _, r := range c.resources {
		if dr, ok := r.(DynamicRenderer); ok {
			<-dr.Done()
		}
	}
	close(c.done)
}

// Render renders all resources in the stack to out.
func (c *stackComponent) Render(out io.Writer) (numLines int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return renderComponents(out, c.resources)
}

// Done returns a channel that's closed when there are no more events to Listen.
func (c *stackComponent) Done() <-chan struct{} {
	return c.done
}

func (c *stackComponent) addResourceRenderer(ev stream.StackEvent, description string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	opts := ResourceRendererOpts{
		StartEvent: &ev,
		RenderOpts: NestedRenderOptions(c.renderOpts),
	}
	c.resources = append(c.resources, ListeningResourceRenderer(c.stack, ev.LogicalResourceID, description, opts))
}

// stackSetComponent is a DynamicRenderer that can display stack set operation events.
type stackSetComponent struct {
	stream <-chan stream.StackSetOpEvent
	done   chan struct{}

	style     RenderOptions
	title     string
	separator rune

	mu        sync.Mutex
	statuses  []cfnStatus
	stopWatch *stopWatch
}

// Listen consumes stack set operation events and updates the status until the streamer closes the channel.
func (c *stackSetComponent) Listen() {
	for ev := range c.stream {
		updateComponentStatus(&c.mu, &c.statuses, cfnStatus{
			value:  ev.Operation.Status,
			reason: ev.Operation.Reason,
		})
		updateComponentTimer(&c.mu, c.statuses, c.stopWatch)
	}
	close(c.done)
}

// Render renders the stack set status updates to out and returns the total number of lines written and error if any.
func (c *stackSetComponent) Render(out io.Writer) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	components := cfnLineItemComponents(c.title, c.separator, c.statuses, c.stopWatch, c.style.Padding)
	return renderComponents(out, components)
}

// Done returns a channel that's closed when there are no more events to Listen.
func (c *stackSetComponent) Done() <-chan struct{} {
	return c.done
}

// ecsServiceResourceComponent can display an ECS service created with CloudFormation.
type ecsServiceResourceComponent struct {
	// Required inputs.
	cfnStream    <-chan stream.StackEvent   // Subscribed stream to initialize the deploymentRenderer.
	ecsDescriber stream.ECSServiceDescriber // Client needed to create an ECSDeploymentStreamer.
	cwDescriber  stream.CloudWatchDescriber // Client needed to create a CloudwatchAlarmStreamer.
	logicalID    string                     // LogicalID for the service.

	// Optional inputs.
	group      *errgroup.Group // Existing group to catch ECSDeploymentStreamer errors.
	ctx        context.Context // Context for the ECSDeploymentStreamer.
	renderOpts RenderOptions

	// Sub-components.
	resourceRenderer   DynamicRenderer
	deploymentRenderer Renderer

	done                chan struct{}
	mu                  sync.Mutex
	newDeploymentRender func(string, time.Time) DynamicRenderer // Overriden in tests.
}

// Listen creates deploymentRenderers if the service is being created, or updated.
// It closes the Done channel if the CFN resource is Done and the deploymentRenderers are also Done.
func (c *ecsServiceResourceComponent) Listen() {
	renderers := []DynamicRenderer{c.resourceRenderer}
	for ev := range c.cfnStream {
		if c.logicalID != ev.LogicalResourceID {
			continue
		}
		if cloudformation.StackStatus(ev.ResourceStatus).UpsertInProgress() {
			if ev.PhysicalResourceID == "" {
				// New service creates receive two "CREATE_IN_PROGRESS" events.
				// The first event doesn't have a service name yet, the second one has.
				continue
			}
			// Start a deployment renderer if a service deployment is happening.
			renderer := c.newDeploymentRender(ev.PhysicalResourceID, ev.Timestamp)
			c.mu.Lock()
			c.deploymentRenderer = renderer
			c.mu.Unlock()
			renderers = append(renderers, renderer)
		}
	}

	// Close the done channel once all the renderers are done listening.
	for _, r := range renderers {
		<-r.Done()
	}
	close(c.done)
}

// Render writes the status of the CloudFormation ECS service resource, followed with details around the
// service deployment if a deployment is happening.
func (c *ecsServiceResourceComponent) Render(out io.Writer) (numLines int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	buf := new(bytes.Buffer)

	nl, err := c.resourceRenderer.Render(buf)
	if err != nil {
		return 0, err
	}
	numLines += nl

	var deploymentRenderer Renderer = &noopComponent{}
	if c.deploymentRenderer != nil {
		deploymentRenderer = c.deploymentRenderer
	}

	sw := &suffixWriter{
		buf:    buf,
		suffix: []byte{'\t', '\t'}, // Add two columns to the deployment renderer so that it aligns with resources.
	}
	nl, err = deploymentRenderer.Render(sw)
	if err != nil {
		return 0, err
	}
	numLines += nl

	if _, err = buf.WriteTo(out); err != nil {
		return 0, err
	}
	return numLines, nil
}

// Done returns a channel that's closed when there are no more events to Listen.
func (c *ecsServiceResourceComponent) Done() <-chan struct{} {
	return c.done
}

func (c *ecsServiceResourceComponent) newListeningRollingUpdateRenderer(serviceARN string, startTime time.Time) DynamicRenderer {
	cluster, service := parseServiceARN(serviceARN)
	streamer := stream.NewECSDeploymentStreamer(c.ecsDescriber, c.cwDescriber, cluster, service, startTime)
	renderer := ListeningRollingUpdateRenderer(streamer, NestedRenderOptions(c.renderOpts))
	c.group.Go(func() error {
		return stream.Stream(c.ctx, streamer)
	})
	return renderer
}

func updateComponentStatus(mu *sync.Mutex, statuses *[]cfnStatus, newStatus cfnStatus) {
	mu.Lock()
	defer mu.Unlock()

	*statuses = append(*statuses, newStatus)
}

func updateComponentTimer(mu *sync.Mutex, statuses []cfnStatus, sw *stopWatch) {
	mu.Lock()
	defer mu.Unlock()

	// There is always at least two elements {notStartedStatus, <new event>}
	curStatus, nextStatus := statuses[len(statuses)-2], statuses[len(statuses)-1]
	switch {
	case nextStatus.value.InProgress():
		// It's possible that CloudFormation sends multiple "CREATE_IN_PROGRESS" events back to back,
		// we don't want to reset the timer then.
		if curStatus.value.InProgress() {
			return
		}
		sw.reset()
		sw.start()
	default:
		if curStatus == notStartedStackStatus {
			// The resource went from [not started] to a finished state immediately.
			// So start the timer and then immediately finish it.
			sw.start()
		}
		sw.stop()
	}
}

func cfnLineItemComponents(description string, separator rune, statuses []cfnStatus, sw *stopWatch, padding int) []Renderer {
	columns := []string{fmt.Sprintf("- %s", description), prettifyLatestStackStatus(statuses), prettifyElapsedTime(sw)}
	components := []Renderer{
		&singleLineComponent{
			Text:    strings.Join(columns, string(separator)),
			Padding: padding,
		},
	}

	for _, failureReason := range failureReasons(statuses) {
		for _, text := range splitByLength(failureReason, maxCellLength) {
			components = append(components, &singleLineComponent{
				Text:    strings.Join([]string{colorFailureReason(text), "", ""}, string(separator)),
				Padding: padding + nestedComponentPadding,
			})
		}
	}
	return components
}
