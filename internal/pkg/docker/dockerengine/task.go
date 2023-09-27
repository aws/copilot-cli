package dockerengine

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"
)

type TaskManager struct {
	mu        sync.RWMutex
	curTask   Task
	curTaskID int
	done      bool

	errors chan error

	docker DockerCmdClient
}

func NewTaskManager(docker DockerCmdClient) *TaskManager {
	return &TaskManager{
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
func (t *TaskManager) Start(task Task) error {
	ctx := context.Background()

	// start the pause container
	pauseRunOpts := task.pauseRunOptions()
	go t.run(-1, pauseRunOpts) // pause is used across taskIDs
	go func() {
		if err := t.waitForContainerToStart(ctx, pauseRunOpts.ContainerName); err != nil {
			t.errors <- fmt.Errorf("wait for pause container to start: %w", err)
		}

		t.Restart(task)
	}()

	for {
		select {
		case err := <-t.errors:
			// only return error if it came from the current task ID.
			// we _expect_ errors from previous task IDs as we shut them down.
			var runErr *runError
			switch {
			case errors.As(err, &runErr):
				t.mu.RLock()
				isCurTask := runErr.taskID == t.curTaskID
				t.mu.RUnlock()
				if isCurTask {
					return runErr.err
				}
			case err != nil:
				return err
			}
		}
	}
}

func (t *TaskManager) Restart(task Task) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.done {
		return
	}

	// ensure no pause container changes
	curOpts := t.curTask.pauseRunOptions()
	newOpts := task.pauseRunOptions()
	switch {
	case !maps.Equal(curOpts.EnvVars, newOpts.EnvVars):
		fallthrough
	case !maps.Equal(curOpts.Secrets, newOpts.Secrets):
		fallthrough
	case !maps.Equal(curOpts.ContainerPorts, newOpts.ContainerPorts):
		t.errors <- errors.New("new task requires recreating pause container")
	}

	if err := t.stopTask(context.Background(), t.curTask); err != nil {
		t.errors <- err
		return
	}

	t.curTask = task
	t.curTaskID++

	for name := range task.Containers {
		name := name
		go t.run(t.curTaskID, task.containerRunOptions(name))
	}
}

func (t *TaskManager) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.done {
		return
	}
	t.done = true

	// collect errors since we want to try to clean up everything we can
	var errs []error
	if err := t.stopTask(context.Background(), t.curTask); err != nil {
		errs = append(errs, err)
	}

	// stop pause container
	// TODO add ctx to docker.Stop() here and in stopTask
	if err := t.docker.Stop(t.curTask.containerID("pause")); err != nil {
		errs = append(errs, fmt.Errorf("stop %q: %w", "pause", err))
	}

	if len(errs) > 0 {
		t.errors <- fmt.Errorf("stop: %w", errors.Join(errs...))
	}
}

func (t *TaskManager) stopTask(ctx context.Context, task Task) error {
	if len(task.Containers) == 0 {
		return nil
	}

	errCh := make(chan error)
	for name := range task.Containers {
		name := name
		go func() {
			if err := t.docker.Stop(task.containerID(name)); err != nil {
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

func (t *TaskManager) waitForContainerToStart(ctx context.Context, id string) error {
	for {
		isRunning, err := t.docker.IsContainerRunning(ctx, id)
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
func (t *TaskManager) run(taskID int, opts RunOptions) {
	if err := t.docker.Run(context.Background(), &opts); err != nil {
		t.errors <- &runError{
			taskID: taskID,
			err:    err,
		}
	}
}
