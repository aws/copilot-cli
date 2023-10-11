// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockerengine

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"
	"time"
)

// Scheduler manages running a Task. Only a single Task
// can be running at a time for a given Scheduler. A Scheduler
// can only be Start()-ed once; multiple calls to Start() will
// break things.
type Scheduler struct {
	idPrefix   string
	logOptions logOptionsFunc

	mu        sync.RWMutex
	curTask   Task
	curTaskID atomic.Int32
	errors    chan error
	stopped   chan struct{}
	wg        *sync.WaitGroup

	docker DockerEngine
}

type logOptionsFunc func(name string, ctr ContainerDefinition) RunLogOptions

type DockerEngine interface {
	Stop(context.Context, string) error
	IsContainerRunning(context.Context, string) (bool, error)
	Run(context.Context, *RunOptions) error
}

// NewScheduler creates a new Scheduler. idPrefix is a prefix used when
// naming containers that are run by the Scheduler.
func NewScheduler(docker DockerEngine, idPrefix string, logOptions logOptionsFunc) *Scheduler {
	return &Scheduler{
		idPrefix:   idPrefix,
		logOptions: logOptions,
		stopped:    make(chan struct{}),
		docker:     docker,
		wg:         &sync.WaitGroup{},
	}
}

// Start starts the Scheduler with the given task. Use
// Restart() to run a new task with the Scheduler. The first
// error the Scheduler has occur from a running container or
// while performing docker operations will be returned. Start
// calls Stop() when it exits.
func (s *Scheduler) Start(task Task) <-chan error {
	s.errors = make(chan error)

	// start the pause container
	pauseRunOpts := s.pauseRunOptions(task)
	s.run(-1, pauseRunOpts)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.waitForContainerToStart(context.Background(), pauseRunOpts.ContainerName); err != nil {
			s.runtimeErr(fmt.Errorf("wait for pause container to start: %w", err))
		}

		if err := s.Restart(task); err != nil {
			s.runtimeErr(fmt.Errorf("start initial task: %w", err))
		}
	}()

	return s.errors
}

// Restart stops the current running task and starts task.
// Errors that occur while Restarting will be returned by Start().
func (s *Scheduler) Restart(task Task) error {
	// we no longer care about errors from the old task
	taskID := s.curTaskID.Add(1)

	s.mu.Lock()
	defer s.mu.Unlock()

	// ensure no pause container changes
	if taskID != 1 {
		curOpts := s.pauseRunOptions(s.curTask)
		newOpts := s.pauseRunOptions(task)
		if !maps.Equal(curOpts.EnvVars, newOpts.EnvVars) ||
			!maps.Equal(curOpts.Secrets, newOpts.Secrets) ||
			!maps.Equal(curOpts.ContainerPorts, newOpts.ContainerPorts) {
			return errors.New("new task requires recreating pause container")
		}
	}

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

	if err := s.stopTask(ctx, s.curTask); err != nil {
		return fmt.Errorf("stop existing task: %w", err)
	}

	s.curTask = task
	for name, ctr := range task.Containers {
		name, ctr := name, ctr
		s.run(taskID, s.containerRunOptions(name, ctr))
	}

	return nil
}

// Stop stops the task and scheduler containers. If Stop() has already been
// called, it does nothing and returns nil.
func (s *Scheduler) Stop() error {
	select {
	case <-s.stopped:
		// only need to stop once
		s.wg.Wait()
		return nil
	default:
		// ignore run errors
		s.curTaskID.Add(1)
		close(s.stopped)
	}

	s.mu.Lock()
	defer func() {
		s.mu.Unlock()
		s.wg.Wait()
		close(s.errors)
	}()

	// collect errors since we want to try to clean up everything we can
	var errs []error
	if err := s.stopTask(context.Background(), s.curTask); err != nil {
		errs = append(errs, err)
	}

	// stop pause container
	if err := s.docker.Stop(context.Background(), s.containerID("pause")); err != nil {
		errs = append(errs, fmt.Errorf("stop %q: %w", "pause", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("stop: %w", errors.Join(errs...))
	}
	return nil
}

// stopTask calls `docker stop` for all containers defined by task.
func (s *Scheduler) stopTask(ctx context.Context, task Task) error {
	if len(task.Containers) == 0 {
		return nil
	}

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
func (s *Scheduler) pauseRunOptions(t Task) RunOptions {
	opts := RunOptions{
		ImageURI:       "public.ecr.aws/amazonlinux/amazonlinux:2023",
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
func (s *Scheduler) containerRunOptions(name string, ctr ContainerDefinition) RunOptions {
	return RunOptions{
		ImageURI:         ctr.ImageURI,
		ContainerName:    s.containerID(name),
		EnvVars:          ctr.EnvVars,
		Secrets:          ctr.Secrets,
		ContainerNetwork: s.containerID("pause"),
		LogOptions:       s.logOptions(name, ctr),
	}
}

func (s *Scheduler) runtimeErr(err error) {
	s.errors <- err
	/*
		select {
		case <-s.stopped:
		default:
			s.errors <- err
		}
	*/
}

// run calls `docker run` using opts. Errors are only returned
// to the main scheduler routine if the taskID the container was run with
// matches the current taskID the scheduler is running.
func (s *Scheduler) run(taskID int32, opts RunOptions) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		if err := s.docker.Run(context.Background(), &opts); err != nil {
			if taskID == -1 || taskID == s.curTaskID.Load() {
				s.runtimeErr(fmt.Errorf("run %q: %w", opts.ContainerName, err))
			}
		}
	}()
}
