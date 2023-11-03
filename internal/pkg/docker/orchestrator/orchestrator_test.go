// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine/dockerenginetest"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator(t *testing.T) {
	noLogs := func(name string, ctr ContainerDefinition) dockerengine.RunLogOptions {
		return dockerengine.RunLogOptions{
			Output: io.Discard,
		}
	}

	tests := map[string]struct {
		dockerEngine func(t *testing.T, sync chan struct{}) DockerEngine
		logOptions   logOptionsFunc
		test         func(t *testing.T, o *Orchestrator, sync chan struct{})

		errs []string
	}{
		"stop and start": {
			dockerEngine: func(t *testing.T, sync chan struct{}) DockerEngine {
				return &dockerenginetest.Double{}
			},
			test: func(t *testing.T, o *Orchestrator, sync chan struct{}) {},
		},
		"error if fail to build pause container": {
			dockerEngine: func(t *testing.T, sync chan struct{}) DockerEngine {
				return &dockerenginetest.Double{
					BuildFn: func(ctx context.Context, ba *dockerengine.BuildArguments, w io.Writer) error {
						return errors.New("some error")
					},
				}
			},
			test: func(t *testing.T, o *Orchestrator, sync chan struct{}) {
				o.RunTask(Task{})
			},
			errs: []string{
				`build pause container: some error`,
			},
		},
		"error if unable to check if pause container is running": {
			dockerEngine: func(t *testing.T, sync chan struct{}) DockerEngine {
				return &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return false, errors.New("some error")
					},
				}
			},
			test: func(t *testing.T, o *Orchestrator, sync chan struct{}) {
				o.RunTask(Task{})
			},
			errs: []string{
				`wait for pause container to start: check if "prefix-pause" is running: some error`,
			},
		},
		"error stopping task": {
			dockerEngine: func(t *testing.T, sync chan struct{}) DockerEngine {
				return &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
					StopFn: func(ctx context.Context, name string) error {
						if name == "prefix-success" {
							return nil
						}
						return errors.New("some error")
					},
				}
			},
			logOptions: noLogs,
			test: func(t *testing.T, o *Orchestrator, sync chan struct{}) {
				o.RunTask(Task{
					Containers: map[string]ContainerDefinition{
						"foo":     {},
						"bar":     {},
						"success": {},
					},
				})
			},
			errs: []string{
				`stop "pause": some error`,
				`stop "foo": some error`,
				`stop "bar": some error`,
			},
		},
		"error restarting new task due to pause changes": {
			dockerEngine: func(t *testing.T, sync chan struct{}) DockerEngine {
				return &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
				}
			},
			logOptions: noLogs,
			test: func(t *testing.T, o *Orchestrator, sync chan struct{}) {
				o.RunTask(Task{
					Containers: map[string]ContainerDefinition{
						"foo": {
							Ports: map[string]string{
								"8080": "80",
							},
						},
					},
				})
				o.RunTask(Task{
					Containers: map[string]ContainerDefinition{
						"foo": {
							Ports: map[string]string{
								"10000": "80",
							},
						},
					},
				})
			},
			errs: []string{
				"new task requires recreating pause container",
			},
		},
		"success with a task": {
			dockerEngine: func(t *testing.T, sync chan struct{}) DockerEngine {
				return &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
					RunFn: func(ctx context.Context, opts *dockerengine.RunOptions) error {
						// validate pause container has correct ports and secrets
						if opts.ContainerName == "prefix-pause" {
							require.Equal(t, map[string]string{
								"8080": "80",
								"9000": "90",
							}, opts.ContainerPorts)
							require.Equal(t, map[string]string{
								"A_SECRET": "very secret",
							}, opts.Secrets)
						}
						return nil
					},
				}
			},
			logOptions: noLogs,
			test: func(t *testing.T, o *Orchestrator, sync chan struct{}) {
				o.RunTask(Task{
					PauseSecrets: map[string]string{
						"A_SECRET": "very secret",
					},
					Containers: map[string]ContainerDefinition{
						"foo": {
							Ports: map[string]string{
								"8080": "80",
							},
						},
						"bar": {
							Ports: map[string]string{
								"9000": "90",
							},
						},
					},
				})
			},
			errs: []string{},
		},
		"proxy setup connection returns error": {
			dockerEngine: func(t *testing.T, sync chan struct{}) DockerEngine {
				return &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
					ExecFn: func(ctx context.Context, ctr string, w io.Writer, cmd string, args ...string) error {
						if cmd == "aws" {
							sync <- struct{}{}
							return errors.New("some error")
						}
						return nil
					},
				}
			},
			logOptions: noLogs,
			test: func(t *testing.T, o *Orchestrator, sync chan struct{}) {
				_, ipNet, err := net.ParseCIDR("172.20.0.0/16")
				require.NoError(t, err)

				o.RunTask(Task{
					PauseSecrets: map[string]string{
						"A_SECRET": "very secret",
					},
					Containers: map[string]ContainerDefinition{
						"foo": {
							Ports: map[string]string{
								"8080": "80",
							},
						},
						"bar": {
							Ports: map[string]string{
								"9000": "90",
							},
						},
					},
				}, RunTaskWithProxy("ecs:cluster_task_ctr", *ipNet, Host{
					Host: "remote-foo",
					Port: "80",
				}))

				<-sync
			},
			errs: []string{`setup proxy connections: setup proxy connection for "remote-foo": some error`},
		},
		"proxy success": {
			dockerEngine: func(t *testing.T, sync chan struct{}) DockerEngine {
				return &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
					ExecFn: func(ctx context.Context, ctr string, w io.Writer, cmd string, args ...string) error {
						if cmd == "aws" {
							fmt.Fprintf(w, "Port 61972 opened for sessionId mySessionId\n")
						}
						return nil
					},
					RunFn: func(ctx context.Context, opts *dockerengine.RunOptions) error {
						if opts.ContainerName == "prefix-foo" {
							sync <- struct{}{}
						}
						return nil
					},
				}
			},
			logOptions: noLogs,
			test: func(t *testing.T, o *Orchestrator, sync chan struct{}) {
				_, ipNet, err := net.ParseCIDR("172.20.0.0/16")
				require.NoError(t, err)

				o.RunTask(Task{
					PauseSecrets: map[string]string{
						"A_SECRET": "very secret",
					},
					Containers: map[string]ContainerDefinition{
						"foo": {
							Ports: map[string]string{
								"8080": "80",
							},
						},
						"bar": {
							Ports: map[string]string{
								"9000": "90",
							},
						},
					},
				}, RunTaskWithProxy("ecs:cluster_task_ctr", *ipNet, Host{
					Host: "remote-foo",
					Port: "80",
				}))

				// don't return until Run() has been called for foo
				<-sync
			},
		},
		"proxy success, connection runtime error": {
			dockerEngine: func(t *testing.T, sync chan struct{}) DockerEngine {
				runCalled := make(chan struct{})
				return &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
					ExecFn: func(ctx context.Context, ctr string, w io.Writer, cmd string, args ...string) error {
						if cmd == "aws" {
							fmt.Fprintf(w, "Port 61972 opened for sessionId mySessionId\n")
							<-runCalled // wait to return error after Run() is called
							fmt.Printf("returning\n")
							return errors.New("some error")
						} else if cmd == "iptables" {
							close(runCalled)
						}
						return nil
					},
					RunFn: func(ctx context.Context, opts *dockerengine.RunOptions) error {
						if opts.ContainerName == "prefix-foo" {
							sync <- struct{}{}
						}
						return nil
					},
				}
			},
			logOptions: noLogs,
			test: func(t *testing.T, o *Orchestrator, sync chan struct{}) {
				_, ipNet, err := net.ParseCIDR("172.20.0.0/16")
				require.NoError(t, err)

				o.RunTask(Task{
					PauseSecrets: map[string]string{
						"A_SECRET": "very secret",
					},
					Containers: map[string]ContainerDefinition{
						"foo": {
							Ports: map[string]string{
								"8080": "80",
							},
						},
						"bar": {
							Ports: map[string]string{
								"9000": "90",
							},
						},
					},
				}, RunTaskWithProxy("ecs:cluster_task_ctr", *ipNet, Host{
					Host: "remote-foo",
					Port: "80",
				}))

				// don't stop until Run() has been called for foo
				<-sync
			},
			errs: []string{`proxy connection to remote-foo:80 closed: some error`},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			syncCh := make(chan struct{})
			o := New(tc.dockerEngine(t, syncCh), "prefix-", tc.logOptions)

			wg := &sync.WaitGroup{}
			wg.Add(2)

			errs := o.Start()

			go func() {
				defer wg.Done()

				var actualErrs []string
				for err := range errs {
					actualErrs = append(actualErrs, strings.Split(err.Error(), "\n")...)
				}

				require.ElementsMatch(t, tc.errs, actualErrs)
			}()

			go func() {
				defer wg.Done()

				tc.test(t, o, syncCh)
				o.Stop()
			}()

			wg.Wait()
		})
	}
}
