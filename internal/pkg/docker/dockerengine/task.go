package dockerengine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type TaskManager struct {
	mu     sync.Mutex
	task   Task
	taskID int

	errors chan error
	runCtx context.Context

	docker DockerCmdClient
}

type runError struct {
	taskID int32
	err    error
}

func (r *runError) Error() string {
	return r.err.Error()
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

// Start starts the task mananger with the given task. Use
// Restart() to run an updated task with the same manager. Any errors
// encountered by operations done by the task manager will be returned
// by Start().
func (t *TaskManager) Start(task Task) error {
	ctx := context.Background()
	t.start(ctx, task.pauseRunOptions())

	t.Restart(task)
	/*
		for name := range task.Containers {
			name := name
			go func() {
				if err := t.start(ctx, task.containerRunOptions(name)); err != nil {
					t.errors <- fmt.Errorf("start %q: %w", name, err)
				}
			}()
		}
	*/

	select {
	case err := <-t.errors:
		// only return error if it came from the current task ID.
		// we _expect_ errors from previous task IDs as we shut them down.
		var runError *runError
		switch {
		case err == nil:
		case errors.As(err, &runError):
			t.mu.Lock()
			isCurTask := runError.taskID == t.taskID
			t.mu.Unlock()
		}
		if errors.As(err, &runError) {
			if err.taskID == t.taskID.Load() {
				return err.err
			}
		}
	}
}

func (t *TaskManager) Restart(task Task) {
	// need to find a way to ignore previous task errors in Start().
	// if exposed ports change, we have to restart pause too. for now, just error in this case
}

func (t *TaskManager) stopTask() {
}

func (t *TaskManager) startTask() {
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

// start starts the container and waits for it to be running.
// start creates a goroutine that is stopped with Stop() functions.
func (t *TaskManager) start(ctx context.Context, opts RunOptions) error {
	go func() {
		if err := t.docker.Run(t.runCtx, &opts); err != nil {
			t.errors <- err
		}
	}()

	for {
		// TODO use ctx
		isRunning, err := t.docker.IsContainerRunning(opts.ContainerName)
		if err != nil {
			return fmt.Errorf("check if container is running: %w", err)
		}
		if isRunning {
			return nil
		}
		// If the container isn't running yet, sleep for a short duration before checking again.
		time.Sleep(time.Second)
	}
}
