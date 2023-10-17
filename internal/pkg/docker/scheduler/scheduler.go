// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
)

// Scheduler manages running a Task. Only a single Task
// can be running at a time for a given Scheduler.
type Scheduler struct {
	idPrefix   string
	logOptions logOptionsFunc

	curTask   Task
	curTaskID atomic.Int32
	runErrs   chan error
	stopped   chan struct{}
	wg        *sync.WaitGroup
	actions   chan action
	stopOnce  *sync.Once

	docker DockerEngine
}

type action interface {
	Do(s *Scheduler) error
}

type logOptionsFunc func(name string, ctr ContainerDefinition) dockerengine.RunLogOptions

// DockerEngine is used by Scheduler to manage containers.
type DockerEngine interface {
	Run(context.Context, *dockerengine.RunOptions) error
	IsContainerRunning(context.Context, string) (bool, error)
	Stop(context.Context, string) error
}

const (
	stoppedTaskID  = -1
	pauseCtrTaskID = 0
)

const (
	pauseContainerURI = "public.ecr.aws/amazonlinux/amazonlinux:2023"
)

// NewScheduler creates a new Scheduler. idPrefix is a prefix used when
// naming containers that are run by the Scheduler.
func NewScheduler(docker DockerEngine, idPrefix string, logOptions logOptionsFunc) *Scheduler {
	return &Scheduler{
		idPrefix:   idPrefix,
		logOptions: logOptions,
		stopped:    make(chan struct{}),
		docker:     docker,
		wg:         &sync.WaitGroup{},
		stopOnce:   &sync.Once{},
		actions:    make(chan action),
		runErrs:    make(chan error),
	}
}

// Start starts the Scheduler. Start must be called before any other
// Scheduler functions. Errors from containers run by the scheduler
// or from Scheduler actions are sent to the returned error channel.
// The returned error channel is closed after calling Stop() has
// stopped the Scheduler. A Scheduler should only be Started once.
func (s *Scheduler) Start() chan error {
	// close done when all goroutines created by scheduler have finished
	done := make(chan struct{})
	errs := make(chan error)

	// scheduler routine
	s.wg.Add(1) // decremented by stopAction
	go func() {
		for {
			select {
			case action := <-s.actions:
				if err := action.Do(s); err != nil {
					errs <- err
				}
			case err := <-s.runErrs:
				errs <- err
			case <-done:
				close(errs)
				return
			}
		}
	}()

	go func() {
		s.wg.Wait()
		close(done)
	}()

	return errs
}

// RunTask stops the current running task and starts task.
func (s *Scheduler) RunTask(task Task) {
	// this guarantees the following:
	// - if runTaskAction{} is pulled by the scheduler, any errors
	//   returned by it are reported by the scheduler.
	// - if Stop() is called _before_ the scheduler picks up this
	//   action, then this action is skipped.
	select {
	case <-s.stopped:
	case s.actions <- &runTaskAction{
		task: task,
	}:
	}
}

type runTaskAction struct {
	task Task
}

func (r *runTaskAction) Do(s *Scheduler) error {
	// we no longer care about errors from the old task
	taskID := s.curTaskID.Add(1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// cancelCtxOnStop calls cancel if Stop() is called before ctx finishes.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		select {
		case <-ctx.Done():
		case <-s.stopped:
			cancel()
		}
	}()

	if taskID == 1 {
		// start the pause container
		opts := s.pauseRunOptions(r.task)
		s.run(pauseCtrTaskID, opts)
		if err := s.waitForContainerToStart(ctx, opts.ContainerName); err != nil {
			return fmt.Errorf("wait for pause container to start: %w", err)
		}
	} else {
		// ensure no pause container changes
		curOpts := s.pauseRunOptions(s.curTask)
		newOpts := s.pauseRunOptions(r.task)
		if !maps.Equal(curOpts.EnvVars, newOpts.EnvVars) ||
			!maps.Equal(curOpts.Secrets, newOpts.Secrets) ||
			!maps.Equal(curOpts.ContainerPorts, newOpts.ContainerPorts) {
			return errors.New("new task requires recreating pause container")
		}

		if err := s.stopTask(ctx, s.curTask); err != nil {
			return fmt.Errorf("stop existing task: %w", err)
		}
	}

	for name, ctr := range r.task.Containers {
		name, ctr := name, ctr
		s.run(taskID, s.containerRunOptions(name, ctr))
	}

	s.curTask = r.task
	return nil
}

// Stop stops the current running task containers and the Scheduler. Stop is
// idempotent and safe to call multiple times. Calls to RunTask() after calling Stop
// do nothing.
func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopped)
		s.actions <- &stopAction{}
	})
}

type stopAction struct{}

func (a *stopAction) Do(s *Scheduler) error {
	defer s.wg.Done()                // for the scheduler
	s.curTaskID.Store(stoppedTaskID) // ignore runtime errors

	// collect errors since we want to try to clean up everything we can
	var errs []error
	if err := s.stopTask(context.Background(), s.curTask); err != nil {
		errs = append(errs, err)
	}

	// stop pause container
	if err := s.docker.Stop(context.Background(), s.containerID("pause")); err != nil {
		errs = append(errs, fmt.Errorf("stop %q: %w", "pause", err))
	}

	return errors.Join(errs...)
}

// stopTask calls `docker stop` for all containers defined by task.
func (s *Scheduler) stopTask(ctx context.Context, task Task) error {
	if len(task.Containers) == 0 {
		return nil
	}

	// errCh gets one error per container
	errCh := make(chan error)
	for name := range task.Containers {
		name := name
		go func() {
			if err := s.docker.Stop(ctx, s.containerID(name)); err != nil {
				errCh <- fmt.Errorf("stop %q: %w", name, err)
				return
			}
			errCh <- nil
		}()
	}

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
		if len(errs) == len(task.Containers) {
			break
		}
	}

	return errors.Join(errs...)
}

// waitForContainerToStart blocks until the container specified by id starts.
func (s *Scheduler) waitForContainerToStart(ctx context.Context, id string) error {
	for {
		isRunning, err := s.docker.IsContainerRunning(ctx, id)
		switch {
		case err != nil:
			return fmt.Errorf("check if %q is running: %w", id, err)
		case isRunning:
			return nil
		}

		select {
		case <-time.After(1 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// containerID returns the full ID for a container with name run by s.
func (s *Scheduler) containerID(name string) string {
	return s.idPrefix + name
}

// Task defines a set of Containers to be run together.
// Containers within a Task can talk to each other on localhost
// and are stopped and started as a group.
type Task struct {
	Containers map[string]ContainerDefinition
}

// ContainerDefinition defines information necessary to run a container.
type ContainerDefinition struct {
	ImageURI string
	EnvVars  map[string]string
	Secrets  map[string]string
	Ports    map[string]string // host port -> container port
}

// pauseRunOptions returns RunOptions for the pause container for t.
// The pause container owns the networking namespace that is shared
// among all of the containers in the task.
func (s *Scheduler) pauseRunOptions(t Task) dockerengine.RunOptions {
	opts := dockerengine.RunOptions{
		ImageURI:       pauseContainerURI,
		ContainerName:  s.containerID("pause"),
		Command:        []string{"sleep", "infinity"},
		ContainerPorts: make(map[string]string),
	}

	for _, ctr := range t.Containers {
		for hostPort, ctrPort := range ctr.Ports {
			// TODO some error if host port is already defined?
			opts.ContainerPorts[hostPort] = ctrPort
		}
	}
	return opts
}

// containerRunOptions returns RunOptions for the given container.
func (s *Scheduler) containerRunOptions(name string, ctr ContainerDefinition) dockerengine.RunOptions {
	return dockerengine.RunOptions{
		ImageURI:         ctr.ImageURI,
		ContainerName:    s.containerID(name),
		EnvVars:          ctr.EnvVars,
		Secrets:          ctr.Secrets,
		ContainerNetwork: s.containerID("pause"),
		LogOptions:       s.logOptions(name, ctr),
	}
}

// run calls `docker run` using opts. Errors are only returned
// to the main scheduler routine if the taskID the container was run with
// matches the current taskID the scheduler is running.
func (s *Scheduler) run(taskID int32, opts dockerengine.RunOptions) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		if err := s.docker.Run(context.Background(), &opts); err != nil {
			curTaskID := s.curTaskID.Load()
			if curTaskID == stoppedTaskID {
				return
			}

			// the error is from the pause container
			// or from the currently running task
			if taskID == pauseCtrTaskID || taskID == curTaskID {
				s.runErrs <- fmt.Errorf("run %q: %w", opts.ContainerName, err)
			}
		}
	}()
}
