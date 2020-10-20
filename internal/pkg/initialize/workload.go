// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package initialize

import (
	"encoding"
	"fmt"
	"os"
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

var fmtErrUnrecognizedWlType = "unrecognized workload type %s"

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

// WorkloadProps contains the information needed to represent a Workload (job or service).
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

// WorkloadInitializer holds the clients necessary to initialize either a
// service or job in an existing application.
type WorkloadInitializer struct {
	Store    Store
	Deployer WorkloadAdder
	Ws       Workspace
	Prog     Prog
}

// NewWorkloadInitializer returns a struct which holds the clients and configuration needed to
// initialize a new workload in an existing application
func NewWorkloadInitializer(s Store, ws Workspace, p Prog, d WorkloadAdder) *WorkloadInitializer {
	return &WorkloadInitializer{
		Store:    s,
		Ws:       ws,
		Prog:     p,
		Deployer: d,
	}
}

func (w *WorkloadInitializer) createManifest(props *WorkloadProps, wlType string) (encoding.BinaryMarshaler, error) {
	switch wlType {
	case svcWlType:
		return w.newServiceManifest(props)
	case jobWlType:
		return newJobManifest(props)
	default:
		return nil, fmt.Errorf(fmtErrUnrecognizedWlType, wlType)
	}
}

func (w *WorkloadInitializer) writeManifest(mf encoding.BinaryMarshaler, wlName string, wlType string) (string, error) {
	switch wlType {
	case svcWlType:
		return w.Ws.WriteServiceManifest(mf, wlName)
	case jobWlType:
		return w.Ws.WriteJobManifest(mf, wlName)
	default:
		return "", fmt.Errorf(fmtErrUnrecognizedWlType, wlType)
	}
}

func (w *WorkloadInitializer) addWlToApp(app *config.Application, wlName string, wlType string) error {
	switch wlType {
	case svcWlType:
		return w.Deployer.AddServiceToApp(app, wlName)
	case jobWlType:
		return w.Deployer.AddJobToApp(app, wlName)
	default:
		return fmt.Errorf(fmtErrUnrecognizedWlType, wlType)
	}
}

func (w *WorkloadInitializer) addWlToStore(wl *config.Workload, wlType string) error {
	switch wlType {
	case svcWlType:
		return w.Store.CreateService(wl)
	case jobWlType:
		return w.Store.CreateJob(wl)
	default:
		return fmt.Errorf(fmtErrUnrecognizedWlType, wlType)
	}
}

func (w *WorkloadInitializer) initWorkload(props *WorkloadProps, wlType string) (manifestPath string, err error) {
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

	mf, err := w.createManifest(props, wlType)
	if err != nil {
		return "", err
	}

	var manifestExists bool
	manifestPath, err = w.writeManifest(mf, props.Name, wlType)
	if err != nil {
		e, ok := err.(*workspace.ErrFileExists)
		if !ok {
			return "", fmt.Errorf("write %s manifest: %w", wlType, err)
		}
		manifestExists = true
		manifestPath = e.FileName
	}

	manifestPath, err = relPath(manifestPath)
	if err != nil {
		return "", err
	}

	manifestMsgFmt := "Wrote the manifest for %s %s at %s\n"
	if manifestExists {
		manifestMsgFmt = "Manifest file for %s %s already exists at %s, skipping writing it.\n"
	}
	log.Successf(manifestMsgFmt, wlType, color.HighlightUserInput(props.Name), color.HighlightResource(manifestPath))
	var helpText string
	if wlType == jobWlType {
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
	w.Prog.Start(fmt.Sprintf(fmtAddWlToAppStart, wlType, props.Name))
	if err := w.addWlToApp(app, props.Name, wlType); err != nil {
		w.Prog.Stop(log.Serrorf(fmtAddWlToAppFailed, wlType, props.Name))
		return "", fmt.Errorf("add %s %s to application %s: %w", wlType, props.Name, props.App, err)
	}
	w.Prog.Stop(log.Ssuccessf(fmtAddWlToAppComplete, wlType, props.Name))

	// add job to ssm
	if err := w.addWlToStore(&config.Workload{
		App:  props.App,
		Name: props.Name,
		Type: props.Type,
	}, wlType); err != nil {
		return "", fmt.Errorf("saving %s %s: %w", wlType, props.Name, err)
	}
	return manifestPath, nil
}

// Job writes the job manifest, creates an ECR repository, and adds the job to SSM.
func (w *WorkloadInitializer) Job(i *WorkloadProps) (string, error) {
	return w.initWorkload(i, jobWlType)
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
	return w.initWorkload(i, svcWlType)
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

// relPath returns the path relative to the current working directory.
func relPath(fullPath string) (string, error) {
	wkdir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	path, err := filepath.Rel(wkdir, fullPath)
	if err != nil {
		return "", fmt.Errorf("get relative path of file: %w", err)
	}
	return path, nil
}
