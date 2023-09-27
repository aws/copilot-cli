package dockerengine

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"
)

type Scheduler struct {
	mu        sync.RWMutex
	curTask   Task
	curTaskID int
	running   bool

	errors chan error

	docker DockerCmdClient
}

func NewScheduler(docker DockerCmdClient) *Scheduler {
	return &Scheduler{
		errors: make(chan error),
		docker: docker,
	}
}

// logic in run_local is:
// go task.Start()
//  if a container stops
//  either - continue on to next container based on exit code
//  or - stop workload containers and fail
// go wait for ctrl-C
//  if ctrl-c, task.Stop()
// go watchForWrite()
//  if file written
//    rebuild containers
//    task.RestartWorkloadContainers()

// Start starts the task mananger with the given task. Use
// Restart() to run an updated task with the same manager. Any errors
// encountered by operations done by the task manager will be returned
// by Start().
func (s *Scheduler) Start(task Task) error {
	// TODO start should _stop and return nil_ after Stop() is called
	// so that the goroutine ends after Stop() is called
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already running. use Restart() to run a task.")
	}
	s.running = true
	s.mu.Unlock()

	ctx := context.Background()

	// start the pause container
	pauseRunOpts := task.pauseRunOptions()
	go s.run(-1, pauseRunOpts) // pause is used across taskIDs
	go func() {
		if err := s.waitForContainerToStart(ctx, pauseRunOpts.ContainerName); err != nil {
			s.errors <- fmt.Errorf("wait for pause container to start: %w", err)
		}

		s.Restart(task)
	}()

	for {
		select {
		case err := <-s.errors:
			// only return error if it came from the current task ID.
			// we _expect_ errors from previous task IDs as we shut them down.
			var runErr *runError
			switch {
			case errors.As(err, &runErr):
				s.mu.RLock()
				isCurTask := runErr.taskID == s.curTaskID
				s.mu.RUnlock()
				if isCurTask {
					return runErr.err
				}
			case err != nil:
				return err
			}
		}
	}
}

func (s *Scheduler) Restart(task Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	// ensure no pause container changes
	curOpts := s.curTask.pauseRunOptions()
	newOpts := task.pauseRunOptions()
	switch {
	case !maps.Equal(curOpts.EnvVars, newOpts.EnvVars):
		fallthrough
	case !maps.Equal(curOpts.Secrets, newOpts.Secrets):
		fallthrough
	case !maps.Equal(curOpts.ContainerPorts, newOpts.ContainerPorts):
		s.errors <- errors.New("new task requires recreating pause container")
	}

	if err := s.stopTask(context.Background(), s.curTask); err != nil {
		s.errors <- err
		return
	}

	s.curTask = task
	s.curTaskID++

	for name := range task.Containers {
		name := name
		go s.run(s.curTaskID, task.containerRunOptions(name))
	}
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}
	s.running = false

	// collect errors since we want to try to clean up everything we can
	var errs []error
	if err := s.stopTask(context.Background(), s.curTask); err != nil {
		errs = append(errs, err)
	}

	// stop pause container
	if err := s.docker.Stop(context.Background(), s.curTask.containerID("pause")); err != nil {
		errs = append(errs, fmt.Errorf("stop %q: %w", "pause", err))
	}

	if len(errs) > 0 {
		s.errors <- fmt.Errorf("stop: %w", errors.Join(errs...))
	}
}

func (s *Scheduler) stopTask(ctx context.Context, task Task) error {
	if len(task.Containers) == 0 {
		return nil
	}

	errCh := make(chan error)
	for name := range task.Containers {
		name := name
		go func() {
			if err := s.docker.Stop(ctx, task.containerID(name)); err != nil {
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

type Task struct {
	IDPrefix   string
	Containers map[string]ContainerDefinition
}

type ContainerDefinition struct {
	ImageURI string
	EnvVars  map[string]string
	Secrets  map[string]string
	Ports    map[string]string // host port -> container port
}

func (t *Task) pauseRunOptions() RunOptions {
	opts := RunOptions{
		ImageURI:       "public.ecr.aws/amazonlinux/amazonlinux:2023",
		ContainerName:  t.containerID("pause"),
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

func (t *Task) containerRunOptions(name string) RunOptions {
	ctr := t.Containers[name]
	return RunOptions{
		ImageURI:         ctr.ImageURI,
		ContainerName:    t.containerID(name),
		EnvVars:          ctr.EnvVars,
		Secrets:          ctr.Secrets,
		ContainerNetwork: t.containerID("pause"),
		// TODO logging
	}
}

func (t *Task) containerID(name string) string {
	return t.IDPrefix + name
}

type runError struct {
	taskID int
	err    error
}

func (r *runError) Error() string {
	return r.err.Error()
}

// run calls docker run using opts. Any errors are sent to
// t.errors, wrapped as a runError with the given taskID.
func (t *Scheduler) run(taskID int, opts RunOptions) {
	if err := t.docker.Run(context.Background(), &opts); err != nil {
		t.errors <- &runError{
			taskID: taskID,
			err:    err,
		}
	}
}
