package dockerengine

import (
	"context"
	"errors"
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
	//noLogs := func(name string, ctr ContainerDefinition) RunLogOptions {
	//	return RunLogOptions{
	//		Output: io.Discard,
	//	}
	//}

	tests := map[string]struct {
		dockerEngine func(sync chan struct{}) DockerEngine
		logOptions   logOptionsFunc
		initTask     Task
		test         func(t *testing.T, s *Scheduler, sync chan struct{})

		runErrs []string
	}{
		/*
			"works with empty task definition": {
				dockerEngine: func(sync chan struct{}) DockerEngine {
					return &dockerEngineDouble{
						IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
							return false, errors.New("some error")
						},
					}
				},
				test: func(t *testing.T, s *Scheduler, sync chan struct{}) {
					require.NoError(t, s.Stop())
				},
			},
		*/
		"error returned if unable to check if pause container is running": {
			dockerEngine: func(sync chan struct{}) DockerEngine {
				return &dockerEngineDouble{
					IsContainerRunningFn: func(ctx context.Context, name string) (bool, error) {
						defer func() { sync <- struct{}{} }()
						return false, errors.New("some error")
					},
				}
			},
			test: func(t *testing.T, s *Scheduler, sync chan struct{}) {
				<-sync
				require.NoError(t, s.Stop())
			},
			runErrs: []string{
				`wait for pause container to start: check if "prefix-pause" is running: some error`,
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
			testDone := make(chan struct{})

			go func() {
				defer wg.Done()

				var actualErrs []string
				for {
					select {
					case err := <-runErrs:
						actualErrs = append(actualErrs, err.Error())
					case <-testDone:
						require.ElementsMatch(t, tc.runErrs, actualErrs)
						return
					}
				}
			}()

			go func() {
				defer wg.Done()
				defer close(testDone)
				tc.test(t, s, syncCh)
			}()

			wg.Wait()
		})
	}
}
