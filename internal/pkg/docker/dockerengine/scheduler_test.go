package dockerengine

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type dockerEngineDouble struct {
	StopFn               func(context.Context, string) error
	IsContainerRunningFn func(context.Context, string) (bool, error)
	RunFn                func(context.Context, *RunOptions) error
}

func (d *dockerEngineDouble) Stop(ctx context.Context, name string) error {
	if d.StopFn == nil {
		return nil
	}
	return d.StopFn(ctx, name)
}

func (d *dockerEngineDouble) IsContainerRunning(ctx context.Context, name string) (bool, error) {
	if d.IsContainerRunningFn == nil {
		return false, nil
	}
	return d.IsContainerRunningFn(ctx, name)
}

func (d *dockerEngineDouble) Run(ctx context.Context, opts *RunOptions) error {
	if d.RunFn == nil {
		return nil
	}
	return d.RunFn(ctx, opts)
}

func TestScheduler(t *testing.T) {
	noLogs := func(name string, ctr ContainerDefinition) RunLogOptions {
		return RunLogOptions{
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
				return &dockerEngineDouble{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
				}
			},
			test: func(t *testing.T, s *Scheduler, sync chan struct{}) {},
		},
		"error returned if unable to check if pause container is running": {
			dockerEngine: func(sync chan struct{}) DockerEngine {
				return &dockerEngineDouble{
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
				return &dockerEngineDouble{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						return true, nil
					},
					RunFn: func(ctx context.Context, opts *RunOptions) error {
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
