// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/graph"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
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
	DoesContainerExist(context.Context, string) (bool, error)
	IsContainerRunning(context.Context, string) (bool, error)
	IsContainerCompleteOrSuccess(ctx context.Context, containerName string) (int, error)
	IsContainerHealthy(ctx context.Context, containerName string) (bool, error)
	Stop(context.Context, string) error
	Build(ctx context.Context, args *dockerengine.BuildArguments, w io.Writer) error
	Exec(ctx context.Context, container string, out io.Writer, cmd string, args ...string) error
	Rm(context.Context, string) error
}

const (
	orchestratorStoppedTaskID = -1
	pauseCtrTaskID            = 0
)

const (
	pauseCtrURI = "aws-copilot-pause"
	pauseCtrTag = "latest"
)

const (
	proxyPortStart = uint16(50000)
)

const (
	ctrStateUnknown         = "unknown"
	ctrStateRunningOrExited = "runningOrExited"
	ctrStateRemoved         = "removed"
)

const (
	ctrStateHealthy  = "healthy"
	ctrStateComplete = "complete"
	ctrStateSuccess  = "success"
	ctrStateStart    = "start"
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
	// buffered channel so that the orchestrator routine does not block and
	// can always send the error from both runErrs and action.Do to errs.
	errs := make(chan error, 1)

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
	task Task

	// optional vars for proxy
	hosts     []Host
	ssmTarget string
	network   *net.IPNet
}

// RunTaskOption adds optional data to RunTask.
type RunTaskOption func(*runTaskAction)

// Host represents a service reachable via the network.
type Host struct {
	Name string
	Port uint16
}

// RunTaskWithProxy returns a RunTaskOption that sets up a proxy connection to hosts.
func RunTaskWithProxy(ssmTarget string, network net.IPNet, hosts ...Host) RunTaskOption {
	return func(r *runTaskAction) {
		r.ssmTarget = ssmTarget
		r.hosts = hosts
		r.network = &network
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
		o.curTask = a.task
		if err := o.buildPauseContainer(ctx); err != nil {
			return fmt.Errorf("build pause container: %w", err)
		}

		// start the pause container
		opts := o.pauseRunOptions(a.task)
		o.run(pauseCtrTaskID, opts, true, cancel)
		if err := o.waitForContainerToStart(ctx, opts.ContainerName, true); err != nil {
			return fmt.Errorf("wait for pause container to start: %w", err)
		}

		if len(a.hosts) > 0 {
			if err := o.setupProxyConnections(ctx, opts.ContainerName, a); err != nil {
				return fmt.Errorf("setup proxy connections: %w", err)
			}
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
	o.curTask = a.task
	depGraph := buildDependencyGraph(a.task.Containers, ctrStateUnknown)
	err := depGraph.UpwardTraversal(ctx, func(ctx context.Context, containerName string) error {
		if len(a.task.Containers[containerName].DependsOn) > 0 {
			if err := o.waitForContainerDependencies(ctx, containerName, a); err != nil {
				return fmt.Errorf("wait for container %s dependencies: %w", containerName, err)
			}
		}
		o.run(taskID, o.containerRunOptions(containerName, a.task.Containers[containerName]), a.task.Containers[containerName].IsEssential, cancel)
		return o.waitForContainerToStart(ctx, o.containerID(containerName), a.task.Containers[containerName].IsEssential)
	}, ctrStateUnknown, ctrStateRunningOrExited)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return fmt.Errorf("upward traversal: %w", err)
	}
	return nil
}

func buildDependencyGraph(containers map[string]ContainerDefinition, status string) *graph.LabeledGraph[string] {
	var vertices []string
	for vertex := range containers {
		vertices = append(vertices, vertex)
	}
	dependencyGraph := graph.NewLabeledGraph(vertices, graph.WithStatus[string](status))
	for containerName, container := range containers {
		for depCtr := range container.DependsOn {
			dependencyGraph.Add(graph.Edge[string]{
				From: containerName,
				To:   depCtr,
			})
		}
	}
	return dependencyGraph
}

// setupProxyConnections creates proxy connections to a.hosts in pauseContainer.
// It assumes that pauseContainer is already running. A unique proxy connection
// is created for each host (in parallel) using AWS SSM Port Forwarding through
// a.ssmTarget. Then, each connection is assigned an IP from a.network,
// starting at the bottom of the IP range. Using iptables, TCP packets destined
// for the connection's assigned IP are redirected to the connection. Finally,
// the host's name is mapped to its assigned IP in /etc/hosts.
func (o *Orchestrator) setupProxyConnections(ctx context.Context, pauseContainer string, a *runTaskAction) error {
	fmt.Printf("\nSetting up proxy connections...\n")

	ports := make(map[Host]uint16)
	port := proxyPortStart
	for i := range a.hosts {
		ports[a.hosts[i]] = port
		port++
	}

	for _, host := range a.hosts {
		host := host
		portForHost := ports[host]

		o.wg.Add(1)
		go func() {
			defer o.wg.Done()

			err := o.docker.Exec(context.Background(), pauseContainer, io.Discard, "aws", "ssm", "start-session",
				"--target", a.ssmTarget,
				"--document-name", "AWS-StartPortForwardingSessionToRemoteHost",
				"--parameters", fmt.Sprintf(`{"host":["%s"],"portNumber":["%d"],"localPortNumber":["%d"]}`, host.Name, host.Port, portForHost))
			if err != nil {
				// report err as a runtime error from the pause container
				if o.curTaskID.Load() != orchestratorStoppedTaskID {
					o.runErrs <- fmt.Errorf("proxy to %v:%v: %w", host.Name, host.Port, err)
				}
			}
		}()
	}

	ip := a.network.IP
	for host, port := range ports {
		err := o.docker.Exec(ctx, pauseContainer, io.Discard, "iptables",
			"--table", "nat",
			"--append", "OUTPUT",
			"--destination", ip.String(),
			"--protocol", "tcp",
			"--match", "tcp",
			"--dport", strconv.Itoa(int(host.Port)),
			"--jump", "REDIRECT",
			"--to-ports", strconv.Itoa(int(port)))
		if err != nil {
			return fmt.Errorf("modify iptables: %w", err)
		}

		err = o.docker.Exec(ctx, pauseContainer, io.Discard, "/bin/bash",
			"-c", fmt.Sprintf(`echo %s %s >> /etc/hosts`, ip.String(), host.Name))
		if err != nil {
			return fmt.Errorf("update /etc/hosts: %w", err)
		}

		ip, err = ipv4Increment(ip, a.network)
		if err != nil {
			return fmt.Errorf("increment ip: %w", err)
		}

		fmt.Printf("Created connection to %v:%v\n", host.Name, host.Port)
	}

	fmt.Printf("Finished setting up proxy connections\n\n")
	return nil
}

// ipv4Increment returns a copy of ip that has been incremented.
func ipv4Increment(ip net.IP, network *net.IPNet) (net.IP, error) {
	// make a copy of the previous ip
	cpy := make(net.IP, len(ip))
	copy(cpy, ip)

	ipv4 := cpy.To4()

	var inc func(idx int) error
	inc = func(idx int) error {
		if idx == -1 {
			return errors.New("max ipv4 address")
		}

		ipv4[idx]++
		if ipv4[idx] == 0 { // overflow occured
			return inc(idx - 1)
		}
		return nil
	}

	err := inc(len(ipv4) - 1)
	if err != nil {
		return nil, err
	}
	if !network.Contains(ipv4) {
		return nil, fmt.Errorf("no more addresses in network")
	}
	return ipv4, nil
}

func (o *Orchestrator) buildPauseContainer(ctx context.Context) error {
	arch := "64bit"
	if strings.Contains(runtime.GOARCH, "arm") {
		arch = "arm64"
	}

	return o.docker.Build(ctx, &dockerengine.BuildArguments{
		URI:               pauseCtrURI,
		Tags:              []string{pauseCtrTag},
		DockerfileContent: pauseDockerfile,
		Args: map[string]string{
			"ARCH": arch,
		},
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

	fmt.Printf("\nStopping task...\n")
	// collect errors since we want to try to clean up everything we can
	var errs []error
	if err := o.stopTask(context.Background(), o.curTask); err != nil {
		errs = append(errs, err)
	}

	// stop pause container
	fmt.Printf("Stopping and Removing %q\n", "pause")
	if err := o.docker.Stop(context.Background(), o.containerID("pause")); err != nil {
		errs = append(errs, fmt.Errorf("stop %q: %w", "pause", err))
	}
	if err := o.docker.Rm(context.Background(), o.containerID("pause")); err != nil {
		errs = append(errs, fmt.Errorf("remove %q: %w", "pause", err))
	}
	fmt.Printf("Stopped and Removed %q\n", "pause")

	return errors.Join(errs...)
}

// stopTask calls `docker stop` for all containers defined by task.
func (o *Orchestrator) stopTask(ctx context.Context, task Task) error {
	if len(task.Containers) == 0 {
		return nil
	}

	// errCh gets one error per container
	errCh := make(chan error, len(task.Containers))
	depGraph := buildDependencyGraph(task.Containers, ctrStateRunningOrExited)
	err := depGraph.DownwardTraversal(ctx, func(ctx context.Context, name string) error {
		fmt.Printf("Stopping and Removing %q\n", name)
		if err := o.docker.Stop(ctx, o.containerID(name)); err != nil {
			errCh <- fmt.Errorf("stop %q: %w", name, err)
			return nil
		}
		if err := o.docker.Rm(ctx, o.containerID(name)); err != nil {
			errCh <- fmt.Errorf("remove %q: %w", name, err)
			return nil
		}
		// ensure that container is fully stopped before stopTask finishes blocking
		for {
			exists, err := o.docker.DoesContainerExist(ctx, o.containerID(name))
			if err != nil {
				errCh <- fmt.Errorf("polling container %q for removal: %w", name, err)
				return nil
			}

			if exists {
				select {
				case <-time.After(1 * time.Second):
					continue
				case <-ctx.Done():
					errCh <- fmt.Errorf("check container %q stopped: %w", name, ctx.Err())
					return nil
				}
			}

			fmt.Printf("Stopped and Removed %q\n", name)
			errCh <- nil
			return nil
		}
	}, ctrStateRunningOrExited, ctrStateRemoved)

	if err != nil {
		return fmt.Errorf("downward traversal: %w", err)
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
func (o *Orchestrator) waitForContainerToStart(ctx context.Context, id string, isEssential bool) error {
	for {
		isRunning, err := o.docker.IsContainerRunning(ctx, id)
		switch {
		case err != nil:
			var errContainerExited *dockerengine.ErrContainerExited
			if errors.As(err, &errContainerExited) && !isEssential {
				return nil
			}
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

func (o *Orchestrator) waitForContainerDependencies(ctx context.Context, containerName string, a *runTaskAction) error {
	var deps []string
	for depName, state := range a.task.Containers[containerName].DependsOn {
		deps = append(deps, fmt.Sprintf("%s->%s", depName, state))
	}
	logMsg := strings.Join(deps, ", ")
	fmt.Printf("Waiting for container %q dependencies: [%s]\n", containerName, color.Emphasize(logMsg))
	eg, ctx := errgroup.WithContext(ctx)
	for name, state := range a.task.Containers[containerName].DependsOn {
		name, state := name, state
		eg.Go(func() error {
			ctrId := o.containerID(name)
			isEssential := a.task.Containers[name].IsEssential
			ticker := time.NewTicker(700 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					switch state {
					case ctrStateStart:
						err := o.waitForContainerToStart(ctx, ctrId, isEssential)
						if err != nil {
							return err
						}
						if isEssential {
							log.Successf("Successfully container %s is running\n", ctrId)
							return nil
						}
						fmt.Printf("non-essential container %q started successfully\n", ctrId)
						return nil
					case ctrStateHealthy:
						healthy, err := o.docker.IsContainerHealthy(ctx, ctrId)
						if err != nil {
							if !isEssential {
								fmt.Printf("non-essential container %q failed to be %q: %v\n", ctrId, state, err)
								return nil
							}
							return fmt.Errorf("essential container %q failed to be %q: %w", ctrId, state, err)
						}
						if healthy {
							log.Successf("Successfully dependency container %q reached %q\n", ctrId, state)
							return nil
						}
					case ctrStateComplete:
						exitCode, err := o.docker.IsContainerCompleteOrSuccess(ctx, ctrId)
						if err != nil {
							return fmt.Errorf("dependency container %q failed to be %q: %w", ctrId, state, err)
						}
						if exitCode != -1 {
							log.Successf("Successfully dependency container %q exited with code: %d\n", ctrId, exitCode)
							return nil
						}
					case ctrStateSuccess:
						exitCode, err := o.docker.IsContainerCompleteOrSuccess(ctx, ctrId)
						if err != nil {
							return fmt.Errorf("dependency container %q failed to be %q: %w", ctrId, state, err)
						}
						if exitCode > 0 {
							return fmt.Errorf("dependency container %q exited with code %d instead of %d exit code", ctrId, exitCode, 0)
						}
						if exitCode == 0 {
							log.Successf("Successfully dependency container %q reached %q\n", ctrId, state)
							return nil
						}
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})
	}
	return eg.Wait()
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
	ImageURI    string
	EnvVars     map[string]string
	Secrets     map[string]string
	Ports       map[string]string // host port -> container port
	IsEssential bool
	DependsOn   map[string]string
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
		Init:                 true,
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
func (o *Orchestrator) run(taskID int32, opts dockerengine.RunOptions, isEssential bool, cancel context.CancelFunc) {
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		err := o.docker.Run(context.Background(), &opts)

		// if the orchestrator has already stopped,
		// we don't want to report the error
		curTaskID := o.curTaskID.Load()
		if curTaskID == orchestratorStoppedTaskID {
			return
		}

		// the error is from the pause container
		// or from the currently running task
		if taskID == pauseCtrTaskID || taskID == curTaskID {
			var errContainerExited *dockerengine.ErrContainerExited
			if !isEssential && (errors.As(err, &errContainerExited) || err == nil) {
				return
			}
			if err == nil {
				err = errors.New("container stopped unexpectedly")
			}
			// cancel context to indicate all the other go routines spawned by `graph.UpwardTarversal`.
			cancel()
			o.runErrs <- fmt.Errorf("run %q: %w", opts.ContainerName, err)
		}
	}()
}
