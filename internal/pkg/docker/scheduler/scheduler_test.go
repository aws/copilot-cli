package scheduler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine/dockerenginetest"
	"github.com/stretchr/testify/require"
)

func TestScheduler(t *testing.T) {
	noLogs := func(name string, ctr ContainerDefinition) dockerengine.RunLogOptions {
		return dockerengine.RunLogOptions{
			Output: io.Discard,
		}
	}

	tests := map[string]struct {
		dockerEngine func(t *testing.T, sync chan struct{}) DockerEngine
		logOptions   logOptionsFunc
		initTask     Task
		test         func(t *testing.T, s *Scheduler, sync chan struct{})

		runErrs  []string
		stopErrs []string // Stop errors are created by errors.Join
	}{
		"works with empty task definition": {
			dockerEngine: func(t *testing.T, sync chan struct{}) DockerEngine {
				return &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
				}
			},
			test: func(t *testing.T, s *Scheduler, sync chan struct{}) {},
		},
		"error returned if unable to check if pause container is running": {
			dockerEngine: func(t *testing.T, sync chan struct{}) DockerEngine {
				return &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return false, errors.New("some error")
					},
				}
			},
			test: func(t *testing.T, s *Scheduler, sync chan struct{}) {},
			runErrs: []string{
				`wait for pause container to start: check if "prefix-pause" is running: some error`,
			},
		},
		"error running container foo": {
			dockerEngine: func(t *testing.T, sync chan struct{}) DockerEngine {
				return &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
					RunFn: func(ctx context.Context, opts *dockerengine.RunOptions) error {
						if opts.ContainerName == "prefix-foo" {
							return errors.New("some error")
						}
						return nil
					},
				}
			},
			logOptions: noLogs,
			test:       func(t *testing.T, s *Scheduler, sync chan struct{}) {},
			initTask: Task{
				Containers: map[string]ContainerDefinition{
					"foo": {},
				},
			},
			runErrs: []string{
				`run "prefix-foo": some error`,
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
			test:       func(t *testing.T, s *Scheduler, sync chan struct{}) {},
			initTask: Task{
				Containers: map[string]ContainerDefinition{
					"foo":     {},
					"bar":     {},
					"success": {},
				},
			},
			stopErrs: []string{
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
					RunFn: func(ctx context.Context, ro *dockerengine.RunOptions) error {
					},
				}
			},
			logOptions: noLogs,
			initTask: Task{
				Containers: map[string]ContainerDefinition{
					"foo": {
						Ports: map[string]string{
							"8080": "80",
						},
					},
				},
			},
			test: func(t *testing.T, s *Scheduler, sync chan struct{}) {
				// TODO need to wait for the first Start() task to start first
				// before starting the second task
				err := s.Restart(Task{
					Containers: map[string]ContainerDefinition{
						"foo": {
							Ports: map[string]string{
								"10000": "80",
							},
						},
					},
				})
				fmt.Printf("error: %s\n", err)
				require.EqualError(t, err, "new task requires recreating pause container")
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			syncCh := make(chan struct{})
			s := NewScheduler(tc.dockerEngine(t, syncCh), "prefix-", tc.logOptions)

			wg := &sync.WaitGroup{}
			wg.Add(2)

			runErrs := s.Start(tc.initTask)
			done := make(chan struct{})

			go func() {
				defer wg.Done()
				defer close(done)

				if len(tc.runErrs) == 0 {
					return
				}

				var actualErrs []string
				for err := range runErrs {
					actualErrs = append(actualErrs, err.Error())

					if len(tc.runErrs) == len(actualErrs) {
						require.ElementsMatch(t, tc.runErrs, actualErrs)
						return
					}
				}

				require.ElementsMatch(t, tc.runErrs, actualErrs)
			}()

			go func() {
				defer wg.Done()
				tc.test(t, s, syncCh)
				<-done
				err := s.Stop()
				if len(tc.stopErrs) == 0 {
					require.NoError(t, err)
				} else {
					err := errors.Unwrap(err)
					require.ElementsMatch(t, tc.stopErrs, strings.Split(err.Error(), "\n"))
				}
			}()

			wg.Wait()
		})
	}
}
