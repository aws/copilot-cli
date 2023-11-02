// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"golang.org/x/sync/errgroup"
)

// Orchestrator manages running a Task. Only a single Task
// can be running at a time for a given Orchestrator.
type Orchestrator struct {
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
	Do(o *Orchestrator) error
}

type logOptionsFunc func(name string, ctr ContainerDefinition) dockerengine.RunLogOptions

// DockerEngine is used by Orchestrator to manage containers.
type DockerEngine interface {
	Run(context.Context, *dockerengine.RunOptions) error
	IsContainerRunning(context.Context, string) (bool, error)
	Stop(context.Context, string) error
	Build(ctx context.Context, args *dockerengine.BuildArguments, w io.Writer) error
	Exec(ctx context.Context, container string, out io.Writer, cmd string, args ...string) error
}

const (
	orchestratorStoppedTaskID = -1
	pauseCtrTaskID            = 0
)

const (
	pauseCtrURI = "aws-copilot-pause"
	pauseCtrTag = "latest"
)

//go:embed Pause-Dockerfile
var pauseDockerfile string

// New creates a new Orchestrator. idPrefix is a prefix used when
// naming containers that are run by the Orchestrator.
func New(docker DockerEngine, idPrefix string, logOptions logOptionsFunc) *Orchestrator {
	return &Orchestrator{
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

// Start starts the Orchestrator. Start must be called before any other
// orchestrator functions. Errors from containers run by the Orchestrator
// or from Orchestrator actions are sent to the returned error channel.
// The returned error channel is closed after calling Stop() has
// stopped the Orchestrator. An Orchestrator should only be Started once.
func (o *Orchestrator) Start() <-chan error {
	// close done when all goroutines created by Orchestrator have finished
	done := make(chan struct{})
	errs := make(chan error)

	// orchestrator routine
	o.wg.Add(1) // decremented by stopAction
	go func() {
		for {
			select {
			case action := <-o.actions:
				if err := action.Do(o); err != nil {
					errs <- err
				}
			case err := <-o.runErrs:
				errs <- err
			case <-done:
				close(errs)
				return
			}
		}
	}()

	go func() {
		o.wg.Wait()
		close(done)
	}()

	return errs
}

// RunTask stops the current running task and starts task.
func (o *Orchestrator) RunTask(task Task, opts ...RunTaskOption) {
	r := &runTaskAction{
		task: task,
	}
	for _, opt := range opts {
		opt(r)
	}

	// this guarantees the following:
	// - if r is pulled by the Orchestrator, any errors
	//   returned by it are reported by the Orchestrator.
	// - if Stop() is called _before_ the Orchestrator picks up this
	//   action, then this action is skipped.
	select {
	case <-o.stopped:
	case o.actions <- r:
	}
}

type runTaskAction struct {
	task              Task
	hosts             []Host
	remoteContainerID string
	network           net.IPNet
}

type RunTaskOption func(*runTaskAction)

type Host struct {
	Host string
	Port string
}

func RunTaskWithProxy(remoteContainerID string, network net.IPNet, hosts ...Host) RunTaskOption {
	return func(r *runTaskAction) {
		r.remoteContainerID = remoteContainerID
		r.hosts = hosts
		r.network = network
	}
}

func (a *runTaskAction) Do(o *Orchestrator) error {
	// we no longer care about errors from the old task
	taskID := o.curTaskID.Add(1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// cancelCtxOnStop calls cancel if Stop() is called before ctx finishes.
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		select {
		case <-ctx.Done():
		case <-o.stopped:
			cancel()
		}
	}()

	if taskID == 1 {
		if err := o.buildPauseContainer(ctx); err != nil {
			return fmt.Errorf("build pause container: %w", err)
		}

		// start the pause container
		opts := o.pauseRunOptions(a.task)
		o.run(pauseCtrTaskID, opts)
		if err := o.waitForContainerToStart(ctx, opts.ContainerName); err != nil {
			return fmt.Errorf("wait for pause container to start: %w", err)
		}

		// run commands to set up proxy connections
		if err := o.setupProxyConnections(ctx, opts.ContainerName, a); err != nil {
			return fmt.Errorf("setup proxy connections: %w", err)
		}
	} else {
		// ensure no pause container changes
		curOpts := o.pauseRunOptions(o.curTask)
		newOpts := o.pauseRunOptions(a.task)
		if !maps.Equal(curOpts.EnvVars, newOpts.EnvVars) ||
			!maps.Equal(curOpts.Secrets, newOpts.Secrets) ||
			!maps.Equal(curOpts.ContainerPorts, newOpts.ContainerPorts) {
			return errors.New("new task requires recreating pause container")
		}

		if err := o.stopTask(ctx, o.curTask); err != nil {
			return fmt.Errorf("stop existing task: %w", err)
		}
	}

	for name, ctr := range a.task.Containers {
		name, ctr := name, ctr
		o.run(taskID, o.containerRunOptions(name, ctr))
	}

	o.curTask = a.task
	return nil
}

func (o *Orchestrator) setupProxyConnections(ctx context.Context, pauseContainer string, a *runTaskAction) error {
	g, gctx := errgroup.WithContext(ctx)
	ports := make(map[Host]string)
	portsMu := &sync.Mutex{}

	for _, host := range a.hosts {
		host := host
		o.wg.Add(1)
		g.Go(func() error {
			defer o.wg.Done()
			port, err := o.setupProxyConnection(gctx, pauseContainer, a.remoteContainerID, host)
			if err != nil {
				return fmt.Errorf("setup proxy connection for %q: %w", host.Host, err)
			}

			portsMu.Lock()
			defer portsMu.Unlock()
			ports[host] = port
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	ip := a.network.IP
	increment := func(ip net.IP) (net.IP, error) {
		// make a copy of the previous ip
		tmp := ip.To4()
		ip = net.IPv4(tmp[0], tmp[1], tmp[2], tmp[3])
		ip = ip.To4()

		var inc func(idx int) error
		inc = func(idx int) error {
			if idx == -1 {
				return fmt.Errorf("ip overflow")
			}

			ip[idx]++
			if ip[idx] == 0 { // overflow occured
				return inc(idx - 1)
			}
			return nil
		}

		err := inc(3) // 3 since this is an ipv4 address
		if err != nil {
			return nil, fmt.Errorf("get next ip: %w", err)
		}
		if !a.network.Contains(ip) {
			return nil, fmt.Errorf("no more addresses in network")
		}
		return ip, err
	}

	for host, port := range ports {
		ipOut := &bytes.Buffer{}
		err := o.docker.Exec(ctx, pauseContainer, ipOut, "iptables",
			"-t", "nat",
			"-A", "OUTPUT",
			"-d", ip.String(),
			"-p", "tcp",
			"-m", "tcp",
			"--dport", host.Port,
			"-j", "REDIRECT",
			"--to-ports", port)
		if err != nil {
			return fmt.Errorf("modify iptables: %w", err)
		}

		err = o.docker.Exec(ctx, pauseContainer, ipOut, "iptables-save")
		if err != nil {
			return fmt.Errorf("save iptables: %w", err)
		}

		err = o.docker.Exec(ctx, pauseContainer, ipOut, "/bin/bash",
			"-c", fmt.Sprintf(`echo %s %s >> /etc/hosts`, ip.String(), host.Host))
		if err != nil {
			return fmt.Errorf("update /etc/hosts: %w", err)
		}

		ip, err = increment(ip)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *Orchestrator) setupProxyConnection(ctx context.Context, localContainer, remoteContainer string, host Host) (string, error) {
	out := &bytes.Buffer{}
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()

		err := o.docker.Exec(context.Background(), localContainer, out, "aws", "ssm", "start-session",
			"--target", remoteContainer,
			"--document-name", "AWS-StartPortForwardingSessionToRemoteHost",
			"--parameters", fmt.Sprintf(`{"host":["%s"],"portNumber":["%s"]}`, host.Host, host.Port))
		if err != nil && o.curTaskID.Load() != orchestratorStoppedTaskID {
			// should follow same pattern for reporting runtime errors as the pause container
			o.runErrs <- fmt.Errorf("proxy connection to %v:%v closed: %w", host.Host, host.Port, err)
		}
	}()

	var port string
	for port == "" {
		time.Sleep(100 * time.Millisecond)
		// the line we want looks like: (TODO update)
		// Port 3306
		for _, line := range strings.Split(out.String(), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "Port ") {
				split := strings.Split(line, " ")
				port = split[1] // TODO less brittle
				break
			}
		}
	}

	return port, nil
}

func (o *Orchestrator) buildPauseContainer(ctx context.Context) error {
	return o.docker.Build(ctx, &dockerengine.BuildArguments{
		URI:               pauseCtrURI,
		Tags:              []string{pauseCtrTag},
		DockerfileContent: pauseDockerfile,
	}, os.Stderr)
}

// Stop stops the current running task containers and the Orchestrator. Stop is
// idempotent and safe to call multiple times. Calls to RunTask() after calling Stop
// do nothing.
func (o *Orchestrator) Stop() {
	o.stopOnce.Do(func() {
		close(o.stopped)
		o.actions <- &stopAction{}
	})
}

type stopAction struct{}

func (a *stopAction) Do(o *Orchestrator) error {
	defer o.wg.Done()                            // for the Orchestrator
	o.curTaskID.Store(orchestratorStoppedTaskID) // ignore runtime errors

	// collect errors since we want to try to clean up everything we can
	var errs []error
	if err := o.stopTask(context.Background(), o.curTask); err != nil {
		errs = append(errs, err)
	}

	// stop pause container
	if err := o.docker.Stop(context.Background(), o.containerID("pause")); err != nil {
		errs = append(errs, fmt.Errorf("stop %q: %w", "pause", err))
	}

	return errors.Join(errs...)
}

// stopTask calls `docker stop` for all containers defined by task.
func (o *Orchestrator) stopTask(ctx context.Context, task Task) error {
	if len(task.Containers) == 0 {
		return nil
	}

	// errCh gets one error per container
	errCh := make(chan error)
	for name := range task.Containers {
		name := name
		go func() {
			if err := o.docker.Stop(ctx, o.containerID(name)); err != nil {
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
func (o *Orchestrator) waitForContainerToStart(ctx context.Context, id string) error {
	for {
		isRunning, err := o.docker.IsContainerRunning(ctx, id)
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
func (o *Orchestrator) containerID(name string) string {
	return o.idPrefix + name
}

// Task defines a set of Containers to be run together.
// Containers within a Task can talk to each other on localhost
// and are stopped and started as a group.
type Task struct {
	Containers   map[string]ContainerDefinition
	PauseSecrets map[string]string
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
func (o *Orchestrator) pauseRunOptions(t Task) dockerengine.RunOptions {
	opts := dockerengine.RunOptions{
		ImageURI:             fmt.Sprintf("%s:%s", pauseCtrURI, pauseCtrTag),
		ContainerName:        o.containerID("pause"),
		Command:              []string{"sleep", "infinity"},
		ContainerPorts:       make(map[string]string),
		Secrets:              t.PauseSecrets,
		AddLinuxCapabilities: []string{"NET_ADMIN"},
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
func (o *Orchestrator) containerRunOptions(name string, ctr ContainerDefinition) dockerengine.RunOptions {
	return dockerengine.RunOptions{
		ImageURI:         ctr.ImageURI,
		ContainerName:    o.containerID(name),
		EnvVars:          ctr.EnvVars,
		Secrets:          ctr.Secrets,
		ContainerNetwork: o.containerID("pause"),
		LogOptions:       o.logOptions(name, ctr),
	}
}

// run calls `docker run` using opts. Errors are only returned
// to the main Orchestrator routine if the taskID the container was run with
// matches the current taskID the Orchestrator is running.
func (o *Orchestrator) run(taskID int32, opts dockerengine.RunOptions) {
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()

		if err := o.docker.Run(context.Background(), &opts); err != nil {
			curTaskID := o.curTaskID.Load()
			if curTaskID == orchestratorStoppedTaskID {
				return
			}

			// the error is from the pause container
			// or from the currently running task
			if taskID == pauseCtrTaskID || taskID == curTaskID {
				o.runErrs <- fmt.Errorf("run %q: %w", opts.ContainerName, err)
			}
		}
	}()
}
