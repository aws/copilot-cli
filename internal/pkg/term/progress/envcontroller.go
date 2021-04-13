// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"context"
	"io"
)

// EnvControllerConfig holds the required parameters to create an environment controller component.
type EnvControllerConfig struct {
	// Common configuration.
	Description string
	RenderOpts  RenderOptions

	// Env controller action configuration.
	ActionStreamer  StackSubscriber
	ActionLogicalID string

	// Env stack configuration.
	EnvStreamer     StackSubscriber
	CancelEnvStream context.CancelFunc
	EnvStackName    string
	EnvResources    map[string]string
}

// ListeningEnvControllerRenderer returns a component that listens and can render CloudFormation resource events
// from the EnvControllerAction and the environment stack.
func ListeningEnvControllerRenderer(conf EnvControllerConfig) DynamicRenderer {
	return &envControllerComponent{
		cancelEnvStream: conf.CancelEnvStream,
		actionComponent: listeningResourceComponent(
			conf.ActionStreamer,
			conf.ActionLogicalID,
			conf.Description,
			ResourceRendererOpts{
				RenderOpts: conf.RenderOpts,
			}),
		stackComponent: listeningStackComponent(
			conf.EnvStreamer,
			conf.EnvStackName,
			conf.Description,
			conf.EnvResources,
			conf.RenderOpts,
		),
	}
}

type envControllerComponent struct {
	cancelEnvStream context.CancelFunc // Function that cancels streaming events from the environment stack.

	actionComponent *regularResourceComponent
	stackComponent  *stackComponent
}

// Render renders the environment stack if there are any stack event, otherwise
// renders the EnvControllerAction as a resource component.
func (c *envControllerComponent) Render(out io.Writer) (numLines int, err error) {
	if c.hasStackEvents() {
		return c.stackComponent.Render(out)
	}
	return c.actionComponent.Render(out)
}

// Done returns a channel that's closed when:
// If the environment stack has any updates, then when both the env stack and action are done updating.
// If there are no stack updates, then when the action is done updating.
func (c *envControllerComponent) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		// Wait for the env controller action to be done first.
		<-c.actionComponent.Done()

		// When the env controller action is done updating, we check if the env stack had any updates.
		// If there were no updates, then we notify the stack streamer that it should stop trying to stream events
		// because the env controller action never triggered an env stack update.
		if !c.hasStackEvents() {
			c.cancelEnvStream()
		}

		<-c.stackComponent.Done()
		close(done)
	}()
	return done
}

func (c *envControllerComponent) hasStackEvents() bool {
	c.stackComponent.mu.Lock()
	defer c.stackComponent.mu.Unlock()

	return len(c.stackComponent.resources) > 1
}
