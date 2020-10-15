// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package initworkload

import (
	"encoding"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
)

const (
	fmtAddWlToAppStart    = "Creating ECR repositories for %s %s."
	fmtAddWlToAppFailed   = "Failed to create ECR repositories for %s %s.\n"
	fmtAddWlToAppComplete = "Created ECR repositories for %s %s.\n"
)

const (
	jobWlType = "job"
	svcWlType = "service"
)

// Store represents the methods needed to add workloads to the SSM parameter store.
type Store interface {
	GetApplication(appName string) (*config.Application, error)
	CreateService(service *config.Workload) error
	CreateJob(job *config.Workload) error
	ListServices(appName string) ([]*config.Workload, error)
	ListJobs(appName string) ([]*config.Workload, error)
}

// WorkloadAdder contains the methods needed to add jobs and services to an existing application.
type WorkloadAdder interface {
	AddJobToApp(app *config.Application, jobName string) error
	AddServiceToApp(app *config.Application, serviceName string) error
}

// Workspace contains the methods needed to manipulate a Copilot workspace.
type Workspace interface {
	CopilotDirPath() (string, error)
	WriteJobManifest(marshaler encoding.BinaryMarshaler, jobName string) (string, error)
	WriteServiceManifest(marshaler encoding.BinaryMarshaler, serviceName string) (string, error)
}

// Prog contains the methods needed to render multi-stage operations.
type Prog interface {
	Start(label string)
	Stop(label string)
}

// WorkloadProps contains the information needed to represent a Workload (job or service)
type WorkloadProps struct {
	App            string
	Type           string
	Name           string
	DockerfilePath string
	Image          string

	Schedule string
	Timeout  string
	Retries  int

	Port        uint16
	HealthCheck *manifest.ContainerHealthCheck
}

func (p *WorkloadProps) isValidWorkload() bool {
	return p.App != "" &&
		p.Type != "" &&
		p.Name != "" &&
		(p.DockerfilePath != "" || p.Image != "")
}

func (p *WorkloadProps) isValidService() bool {
	return p.isValidWorkload() &&
		p.Schedule == "" &&
		p.Timeout == "" &&
		p.Retries == 0
}

func (p *WorkloadProps) isValidJob() bool {
	return p.isValidWorkload() &&
		p.Schedule != "" &&
		p.Port == 0
}

type manifestWriter func(encoding.BinaryMarshaler, string) (string, error)

type manifestCreator func(*WorkloadProps) (encoding.BinaryMarshaler, error)

type workloadAdder func(*config.Application, string) error

type storeWorkloadAdder func(*config.Workload) error

// WorkloadInitializer holds the clients necessary to initialize either a
// service or job in an existing application.
type WorkloadInitializer struct {
	Store    Store
	Deployer WorkloadAdder
	Ws       Workspace
	Prog     Prog

	createManifest manifestCreator
	writeManifest  manifestWriter
	addWlToApp     workloadAdder
	addWlToStore   storeWorkloadAdder

	wlType string
}

// NewJobInitializer returns a struct which holds the clients and configuration needed to
// initialize a new job in an existing application
func NewJobInitializer(s Store, ws Workspace, p Prog, d WorkloadAdder) *WorkloadInitializer {
	return &WorkloadInitializer{
		Store: s,
		Ws:    ws,
		Prog:  p,

		createManifest: newJobManifest,
		writeManifest:  ws.WriteJobManifest,

		addWlToApp:   d.AddJobToApp,
		addWlToStore: s.CreateJob,

		wlType: jobWlType,
	}
}

// NewSvcInitializer returns a struct which holds the clients and configuration needed to
// initialize a new service in an existing application
func NewSvcInitializer(s Store, ws Workspace, p Prog, d WorkloadAdder) *WorkloadInitializer {
	return &WorkloadInitializer{
		Store: s,
		Ws:    ws,
		Prog:  p,

		// createManifest: newServiceManifest,
		writeManifest: ws.WriteServiceManifest,

		addWlToApp:   d.AddServiceToApp,
		addWlToStore: s.CreateService,

		wlType: svcWlType,
	}
}

func (w *WorkloadInitializer) initWorkload(props *WorkloadProps) (manifestPath string, err error) {
	app, err := w.Store.GetApplication(props.App)
	if err != nil {
		return "", fmt.Errorf("get application %s: %w", props.App, err)
	}

	if props.DockerfilePath != "" {
		path, err := relativeDockerfilePath(w.Ws, props.DockerfilePath)
		if err != nil {
			return "", err
		}
		props.DockerfilePath = path
	}

	mf, err := w.createManifest(props)
	if err != nil {
		return "", err
	}

	var manifestExists bool
	manifestPath, err = w.writeManifest(mf, props.Name)
	if err != nil {
		e, ok := err.(*workspace.ErrFileExists)
		if !ok {
			return "", fmt.Errorf("write %s manifest: %w", w.wlType, err)
		}
		manifestExists = true
		manifestPath = e.FileName
	}

	manifestPath, err = workspace.RelPath(manifestPath)
	if err != nil {
		return "", err
	}

	manifestMsgFmt := "Wrote the manifest for %s %s at %s\n"
	if manifestExists {
		manifestMsgFmt = "Manifest file for %s %s already exists at %s, skipping writing it.\n"
	}
	log.Successf(manifestMsgFmt, w.wlType, color.HighlightUserInput(props.Name), color.HighlightResource(manifestPath))
	var helpText string
	if w.wlType == jobWlType {
		helpText = fmt.Sprintf("Your manifest contains configurations like your container size and job schedule (%s).", props.Schedule)
	} else {
		helpText = "Your manifest contains configurations like your container size and port."
		if props.Port != 0 {
			helpText = fmt.Sprintf("Your manifest contains configurations like your container size and port (:%d).", props.Port)
		}
	}
	log.Infoln(color.Help(helpText))
	log.Infoln()

	// add workload to application
	w.Prog.Start(fmt.Sprintf(fmtAddWlToAppStart, w.wlType, props.Name))
	if err := w.addWlToApp(app, props.Name); err != nil {
		w.Prog.Stop(log.Serrorf(fmtAddWlToAppFailed, w.wlType, props.Name))
		return "", fmt.Errorf("add %s %s to application %s: %w", w.wlType, props.Name, props.App, err)
	}
	w.Prog.Stop(log.Ssuccessf(fmtAddWlToAppComplete, w.wlType, props.Name))

	// add job to ssm
	if err := w.addWlToStore(&config.Workload{
		App:  props.App,
		Name: props.Name,
		Type: props.Type,
	}); err != nil {
		return "", fmt.Errorf("saving %s %s: %w", w.wlType, props.Name, err)
	}
	return manifestPath, nil
}

// Job writes the job manifest, creates an ECR repository, and adds the job to SSM.
func (w *WorkloadInitializer) Job(i *WorkloadProps) (string, error) {
	if !i.isValidJob() {
		return "", errors.New("input properties do not specify a valid job")
	}
	return w.initWorkload(i)
}

func newJobManifest(i *WorkloadProps) (encoding.BinaryMarshaler, error) {
	switch i.Type {
	case manifest.ScheduledJobType:
		return manifest.NewScheduledJob(&manifest.ScheduledJobProps{
			WorkloadProps: &manifest.WorkloadProps{
				Name:       i.Name,
				Dockerfile: i.DockerfilePath,
				Image:      i.Image,
			},
			Schedule: i.Schedule,
			Timeout:  i.Timeout,
			Retries:  i.Retries,
		}), nil
	default:
		return nil, fmt.Errorf("job type %s doesn't have a manifest", i.Type)

	}
}

// Service writes the service manifest, creates an ECR repository, and adds the service to SSM.
func (w *WorkloadInitializer) Service(i *WorkloadProps) (string, error) {
	if !i.isValidService() {
		return "", errors.New("input properties do not specify a valid service")
	}
	return w.initWorkload(i)
}

func (w *WorkloadInitializer) newServiceManifest(i *WorkloadProps) (encoding.BinaryMarshaler, error) {
	switch i.Type {
	case manifest.LoadBalancedWebServiceType:
		return w.newLoadBalancedWebServiceManifest(i)
	case manifest.BackendServiceType:
		return newBackendServiceManifest(i)
	default:
		return nil, fmt.Errorf("service type %s doesn't have a manifest", i.Type)
	}
}

func (w *WorkloadInitializer) newLoadBalancedWebServiceManifest(i *WorkloadProps) (*manifest.LoadBalancedWebService, error) {
	props := &manifest.LoadBalancedWebServiceProps{
		WorkloadProps: &manifest.WorkloadProps{
			Name:       i.Name,
			Dockerfile: i.DockerfilePath,
			Image:      i.Image,
		},
		Port: i.Port,
		Path: "/",
	}
	existingSvcs, err := w.Store.ListServices(i.App)
	if err != nil {
		return nil, err
	}
	// We default to "/" for the first service, but if there's another
	// Load Balanced Web Service, we use the svc name as the default, instead.
	for _, existingSvc := range existingSvcs {
		if existingSvc.Type == manifest.LoadBalancedWebServiceType && existingSvc.Name != i.Name {
			props.Path = i.Name
			break
		}
	}
	return manifest.NewLoadBalancedWebService(props), nil
}

func newBackendServiceManifest(i *WorkloadProps) (*manifest.BackendService, error) {
	return manifest.NewBackendService(manifest.BackendServiceProps{
		WorkloadProps: manifest.WorkloadProps{
			Name:       i.Name,
			Dockerfile: i.DockerfilePath,
			Image:      i.Image,
		},
		Port:        i.Port,
		HealthCheck: i.HealthCheck,
	}), nil
}

// relativeDockerfilePath returns the path from the workspace root to the Dockerfile.
func relativeDockerfilePath(ws Workspace, path string) (string, error) {
	copilotDirPath, err := ws.CopilotDirPath()
	if err != nil {
		return "", fmt.Errorf("get copilot directory: %w", err)
	}
	wsRoot := filepath.Dir(copilotDirPath)
	absDfPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("get absolute path: %v", err)
	}
	relDfPath, err := filepath.Rel(wsRoot, absDfPath)
	if err != nil {
		return "", fmt.Errorf("find relative path from workspace root to Dockerfile: %v", err)
	}
	return relDfPath, nil
}
