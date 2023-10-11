package scheduler

import (
	"context"
	"errors"
	"io"
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
		dockerEngine func(sync chan struct{}) DockerEngine
		logOptions   logOptionsFunc
		initTask     Task
		test         func(t *testing.T, s *Scheduler, sync chan struct{})

		runErrs []string
	}{
		"works with empty task definition": {
			dockerEngine: func(sync chan struct{}) DockerEngine {
				return &dockerenginetest.Double{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
				}
			},
			test: func(t *testing.T, s *Scheduler, sync chan struct{}) {},
		},
		"error returned if unable to check if pause container is running": {
			dockerEngine: func(sync chan struct{}) DockerEngine {
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
			dockerEngine: func(sync chan struct{}) DockerEngine {
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
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			syncCh := make(chan struct{})
			s := NewScheduler(tc.dockerEngine(syncCh), "prefix-", tc.logOptions)

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

				// they'll never match here
				require.ElementsMatch(t, tc.runErrs, actualErrs)
			}()

			go func() {
				defer wg.Done()
				tc.test(t, s, syncCh)
				<-done
				require.NoError(t, s.Stop())
			}()

			wg.Wait()
		})
	}
}
