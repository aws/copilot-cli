// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
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
	generateHosts := func(n int) []Host {
		hosts := make([]Host, n)
		for i := 0; i < n; i++ {
			hosts[i].Name = strconv.Itoa(i)
			hosts[i].Port = uint16(i)
		}
		return hosts
	}

	type test func(*testing.T, *Orchestrator)

	tests := map[string]struct {
		logOptions     logOptionsFunc
		test           func(t *testing.T) (test, DockerEngine)
		stopAfterNErrs int

		errs []string
	}{
		"stop and start": {
			test: func(t *testing.T) (test, DockerEngine) {
				return func(t *testing.T, o *Orchestrator) {}, &dockerenginetest.Double{}
			},
		},
		"error if fail to build pause container": {
			test: func(t *testing.T) (test, DockerEngine) {
				de := &dockerenginetest.Double{
					BuildFn: func(ctx context.Context, ba *dockerengine.BuildArguments, w io.Writer) error {
						return errors.New("some error")
					},
				}
				return func(t *testing.T, o *Orchestrator) {
					o.RunTask(Task{})
				}, de
			},
			errs: []string{
				`build pause container: some error`,
			},
		},
		"error if unable to check if pause container is running": {
			test: func(t *testing.T) (test, DockerEngine) {
				de := &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return false, errors.New("some error")
					},
				}
				return func(t *testing.T, o *Orchestrator) {
					o.RunTask(Task{})
				}, de
			},
			errs: []string{
				`wait for pause container to start: check if "prefix-pause" is running: some error`,
			},
		},
		"error stopping task": {
			logOptions: noLogs,
			test: func(t *testing.T) (test, DockerEngine) {
				de := &dockerenginetest.Double{
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
				return func(t *testing.T, o *Orchestrator) {
					o.RunTask(Task{
						Containers: map[string]ContainerDefinition{
							"foo":     {},
							"bar":     {},
							"success": {},
						},
					})
				}, de
			},
			errs: []string{
				`stop "pause": some error`,
				`stop "foo": some error`,
				`stop "bar": some error`,
			},
		},
		"error restarting new task due to pause changes": {
			logOptions: noLogs,
			test: func(t *testing.T) (test, DockerEngine) {
				de := &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
				}
				return func(t *testing.T, o *Orchestrator) {
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
				}, de
			},
			errs: []string{
				"new task requires recreating pause container",
			},
		},
		"success with a task": {
			logOptions: noLogs,
			test: func(t *testing.T) (test, DockerEngine) {
				de := &dockerenginetest.Double{
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
				return func(t *testing.T, o *Orchestrator) {
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
				}, de
			},
			errs: []string{},
		},
		"proxy setup, connection returns error": {
			logOptions: noLogs,
			test: func(t *testing.T) (test, DockerEngine) {
				de := &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
					ExecFn: func(ctx context.Context, ctr string, w io.Writer, cmd string, args ...string) error {
						if cmd == "aws" {
							return errors.New("some error")
						}
						return nil
					},
				}
				return func(t *testing.T, o *Orchestrator) {
					_, ipNet, err := net.ParseCIDR("172.20.0.0/16")
					require.NoError(t, err)

					o.RunTask(Task{}, RunTaskWithProxy("ecs:cluster_task_ctr", *ipNet, Host{
						Name: "remote-foo",
						Port: 80,
					}))
				}, de
			},
			stopAfterNErrs: 1,
			errs:           []string{`proxy to remote-foo:80: some error`},
		},
		"proxy setup, ip increment error": {
			logOptions: noLogs,
			test: func(t *testing.T) (test, DockerEngine) {
				de := &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
					ExecFn: func(ctx context.Context, ctr string, w io.Writer, cmd string, args ...string) error {
						if cmd == "aws" {
							fmt.Fprintf(w, "Port 61972 opened for sessionId mySessionId\n")
						}
						return nil
					},
				}
				return func(t *testing.T, o *Orchestrator) {
					_, ipNet, err := net.ParseCIDR("255.255.255.254/31") // 255.255.255.254 - 255.255.255.255
					require.NoError(t, err)

					o.RunTask(Task{}, RunTaskWithProxy("ecs:cluster_task_ctr", *ipNet, generateHosts(3)...))
				}, de
			},
			stopAfterNErrs: 1,
			errs:           []string{`setup proxy connections: increment ip: max ipv4 address`},
		},
		"proxy setup, ip tables error": {
			logOptions: noLogs,
			test: func(t *testing.T) (test, DockerEngine) {
				de := &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
					ExecFn: func(ctx context.Context, ctr string, w io.Writer, cmd string, args ...string) error {
						if cmd == "aws" {
							fmt.Fprintf(w, "Port 61972 opened for sessionId mySessionId\n")
						} else if cmd == "iptables" {
							return errors.New("some error")
						}
						return nil
					},
				}
				return func(t *testing.T, o *Orchestrator) {
					_, ipNet, err := net.ParseCIDR("172.20.0.0/16")
					require.NoError(t, err)

					o.RunTask(Task{}, RunTaskWithProxy("ecs:cluster_task_ctr", *ipNet, Host{
						Name: "remote-foo",
						Port: 80,
					}))
				}, de
			},
			stopAfterNErrs: 1,
			errs:           []string{`setup proxy connections: modify iptables: some error`},
		},
		"proxy setup, ip tables save error": {
			logOptions: noLogs,
			test: func(t *testing.T) (test, DockerEngine) {
				de := &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
					ExecFn: func(ctx context.Context, ctr string, w io.Writer, cmd string, args ...string) error {
						if cmd == "aws" {
							fmt.Fprintf(w, "Port 61972 opened for sessionId mySessionId\n")
						} else if cmd == "iptables-save" {
							return errors.New("some error")
						}
						return nil
					},
				}
				return func(t *testing.T, o *Orchestrator) {
					_, ipNet, err := net.ParseCIDR("172.20.0.0/16")
					require.NoError(t, err)

					o.RunTask(Task{}, RunTaskWithProxy("ecs:cluster_task_ctr", *ipNet, Host{
						Name: "remote-foo",
						Port: 80,
					}))
				}, de
			},
			stopAfterNErrs: 1,
			errs:           []string{`setup proxy connections: save iptables: some error`},
		},
		"proxy setup, /etc/hosts error": {
			logOptions: noLogs,
			test: func(t *testing.T) (test, DockerEngine) {
				de := &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
					ExecFn: func(ctx context.Context, ctr string, w io.Writer, cmd string, args ...string) error {
						if cmd == "aws" {
							fmt.Fprintf(w, "Port 61972 opened for sessionId mySessionId\n")
						} else if cmd == "/bin/bash" {
							return errors.New("some error")
						}
						return nil
					},
				}
				return func(t *testing.T, o *Orchestrator) {
					_, ipNet, err := net.ParseCIDR("172.20.0.0/16")
					require.NoError(t, err)

					o.RunTask(Task{}, RunTaskWithProxy("ecs:cluster_task_ctr", *ipNet, Host{
						Name: "remote-foo",
						Port: 80,
					}))
				}, de
			},
			stopAfterNErrs: 1,
			errs:           []string{`setup proxy connections: update /etc/hosts: some error`},
		},
		"proxy success": {
			logOptions: noLogs,
			test: func(t *testing.T) (test, DockerEngine) {
				waitUntilRun := make(chan struct{})
				de := &dockerenginetest.Double{
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
							close(waitUntilRun)
						}
						return nil
					},
				}
				return func(t *testing.T, o *Orchestrator) {
					_, ipNet, err := net.ParseCIDR("172.20.0.0/16")
					require.NoError(t, err)

					o.RunTask(Task{
						Containers: map[string]ContainerDefinition{
							"foo": {},
						},
					}, RunTaskWithProxy("ecs:cluster_task_ctr", *ipNet, Host{
						Name: "remote-foo",
						Port: 80,
					}))

					<-waitUntilRun
				}, de
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			test, dockerEngine := tc.test(t)
			o := New(dockerEngine, "prefix-", tc.logOptions)

			wg := &sync.WaitGroup{}
			wg.Add(2)

			stopCh := make(chan struct{})
			errs := o.Start()

			go func() {
				defer wg.Done()
				if tc.stopAfterNErrs == 0 {
					close(stopCh)
				}

				var actualErrs []string
				for err := range errs {
					actualErrs = append(actualErrs, strings.Split(err.Error(), "\n")...)
					if len(actualErrs) == tc.stopAfterNErrs {
						close(stopCh)
					}
				}

				require.ElementsMatch(t, tc.errs, actualErrs)
			}()

			go func() {
				defer wg.Done()

				test(t, o)
				<-stopCh
				o.Stop()
			}()

			wg.Wait()
		})
	}
}

func TestIPv4Increment(t *testing.T) {
	tests := map[string]struct {
		cidrIP string

		wantErr string
		wantIP  string
	}{
		"increment": {
			cidrIP: "10.0.0.15/24",
			wantIP: "10.0.0.16",
		},
		"overflows to next octet": {
			cidrIP: "10.0.0.255/16",
			wantIP: "10.0.1.0",
		},
		"error if no more ipv4 addresses": {
			cidrIP:  "255.255.255.255/16",
			wantErr: "max ipv4 address",
		},
		"error if out of network": {
			cidrIP:  "10.0.0.255/24",
			wantErr: "no more addresses in network",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ip, network, err := net.ParseCIDR(tc.cidrIP)
			require.NoError(t, err)

			gotIP, gotErr := ipv4Increment(ip, network)
			if tc.wantErr != "" {
				require.EqualError(t, gotErr, tc.wantErr)
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tc.wantIP, gotIP.String())
			}
		})
	}
}
