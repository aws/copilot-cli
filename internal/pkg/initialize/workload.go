// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package initialize contains methods and structs needed to initialize jobs and services.
package initialize

import (
	"encoding"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
)

const (
	fmtAddWlToAppStart    = "Creating ECR repositories for %s %s."
	fmtAddWlToAppFailed   = "Failed to create ECR repositories for %s %s.\n\n"
	fmtAddWlToAppComplete = "Created ECR repositories for %s %s.\n\n"
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
}

// JobProps contains the information needed to represent a Job.
type JobProps struct {
	WorkloadProps
	Schedule    string
	HealthCheck *manifest.ContainerHealthCheck
	Timeout     string
	Retries     int
}

// ServiceProps contains the information needed to represent a Service (port, HealthCheck, and workload common props).
type ServiceProps struct {
	WorkloadProps
	Port        uint16
	HealthCheck *manifest.ContainerHealthCheck
	appDomain   *string
}

// WorkloadInitializer holds the clients necessary to initialize either a
// service or job in an existing application.
type WorkloadInitializer struct {
	Store    Store
	Deployer WorkloadAdder
	Ws       Workspace
	Prog     Prog
}

// Service writes the service manifest, creates an ECR repository, and adds the service to SSM.
func (w *WorkloadInitializer) Service(i *ServiceProps) (string, error) {
	return w.initService(i)
}

// Job writes the job manifest, creates an ECR repository, and adds the job to SSM.
func (w *WorkloadInitializer) Job(i *JobProps) (string, error) {
	return w.initJob(i)
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

func (w *WorkloadInitializer) initJob(props *JobProps) (string, error) {
	if props.DockerfilePath != "" {
		path, err := relativeDockerfilePath(w.Ws, props.DockerfilePath)
		if err != nil {
			return "", err
		}
		props.DockerfilePath = path
	}

	var manifestExists bool
	mf, err := newJobManifest(props)
	if err != nil {
		return "", err
	}
	manifestPath, err := w.Ws.WriteJobManifest(mf, props.Name)
	if err != nil {
		e, ok := err.(*workspace.ErrFileExists)
		if !ok {
			return "", fmt.Errorf("write %s manifest: %w", jobWlType, err)
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
	log.Successf(manifestMsgFmt, jobWlType, color.HighlightUserInput(props.Name), color.HighlightResource(manifestPath))
	var sched = props.Schedule
	if props.Schedule == "" {
		sched = "None"
	}
	helpText := fmt.Sprintf("Your manifest contains configurations like your container size and job schedule (%s).", sched)
	log.Infoln(color.Help(helpText))
	log.Infoln()

	app, err := w.Store.GetApplication(props.App)
	if err != nil {
		return "", fmt.Errorf("get application %s: %w", props.App, err)
	}

	err = w.addJobToAppAndSSM(app, props.WorkloadProps)
	if err != nil {
		return "", err
	}
	return manifestPath, nil
}

func (w *WorkloadInitializer) initService(props *ServiceProps) (string, error) {
	if props.DockerfilePath != "" {
		path, err := relativeDockerfilePath(w.Ws, props.DockerfilePath)
		if err != nil {
			return "", err
		}
		props.DockerfilePath = path
	}
	app, err := w.Store.GetApplication(props.App)
	if err != nil {
		return "", fmt.Errorf("get application %s: %w", props.App, err)
	}
	if app.Domain != "" {
		props.appDomain = aws.String(app.Domain)
	}

	var manifestExists bool
	mf, err := w.newServiceManifest(props)
	if err != nil {
		return "", err
	}
	manifestPath, err := w.Ws.WriteServiceManifest(mf, props.Name)
	if err != nil {
		e, ok := err.(*workspace.ErrFileExists)
		if !ok {
			return "", fmt.Errorf("write %s manifest: %w", svcWlType, err)
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
	log.Successf(manifestMsgFmt, svcWlType, color.HighlightUserInput(props.Name), color.HighlightResource(manifestPath))

	helpText := "Your manifest contains configurations like your container size and port."
	if props.Port != 0 {
		helpText = fmt.Sprintf("Your manifest contains configurations like your container size and port (:%d).", props.Port)
	}
	log.Infoln(color.Help(helpText))
	log.Infoln()

	err = w.addSvcToAppAndSSM(app, props.WorkloadProps)
	if err != nil {
		return "", err
	}
	return manifestPath, nil
}

func (w *WorkloadInitializer) addSvcToAppAndSSM(app *config.Application, props WorkloadProps) error {
	return w.addWlToAppAndSSM(app, props, svcWlType)
}

func (w *WorkloadInitializer) addJobToAppAndSSM(app *config.Application, props WorkloadProps) error {
	return w.addWlToAppAndSSM(app, props, jobWlType)
}

func (w *WorkloadInitializer) addWlToAppAndSSM(app *config.Application, props WorkloadProps, wlType string) error {
	w.Prog.Start(fmt.Sprintf(fmtAddWlToAppStart, wlType, props.Name))
	if err := w.addWlToApp(app, props.Name, wlType); err != nil {
		w.Prog.Stop(log.Serrorf(fmtAddWlToAppFailed, wlType, props.Name))
		return fmt.Errorf("add %s %s to application %s: %w", wlType, props.Name, props.App, err)
	}
	w.Prog.Stop(log.Ssuccessf(fmtAddWlToAppComplete, wlType, props.Name))

	if err := w.addWlToStore(&config.Workload{
		App:  props.App,
		Name: props.Name,
		Type: props.Type,
	}, wlType); err != nil {
		return fmt.Errorf("saving %s %s: %w", wlType, props.Name, err)
	}

	return nil
}

func newJobManifest(i *JobProps) (encoding.BinaryMarshaler, error) {
	switch i.Type {
	case manifest.ScheduledJobType:
		return manifest.NewScheduledJob(&manifest.ScheduledJobProps{
			WorkloadProps: &manifest.WorkloadProps{
				Name:       i.Name,
				Dockerfile: i.DockerfilePath,
				Image:      i.Image,
			},
			HealthCheck: i.HealthCheck,
			Schedule:    i.Schedule,
			Timeout:     i.Timeout,
			Retries:     i.Retries,
		}), nil
	default:
		return nil, fmt.Errorf("job type %s doesn't have a manifest", i.Type)

	}
}

func (w *WorkloadInitializer) newServiceManifest(i *ServiceProps) (encoding.BinaryMarshaler, error) {
	switch i.Type {
	case manifest.LoadBalancedWebServiceType:
		return w.newLoadBalancedWebServiceManifest(i)
	case manifest.RequestDrivenWebServiceType:
		return w.newRequestDrivenWebServiceManifest(i), nil
	case manifest.BackendServiceType:
		return newBackendServiceManifest(i)
	default:
		return nil, fmt.Errorf("service type %s doesn't have a manifest", i.Type)
	}
}

func (w *WorkloadInitializer) newLoadBalancedWebServiceManifest(i *ServiceProps) (*manifest.LoadBalancedWebService, error) {
	props := &manifest.LoadBalancedWebServiceProps{
		WorkloadProps: &manifest.WorkloadProps{
			Name:       i.Name,
			Dockerfile: i.DockerfilePath,
			Image:      i.Image,
		},
		Port:        i.Port,
		HealthCheck: i.HealthCheck,
		AppDomain:   i.appDomain,
		Path:        "/",
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

func (w *WorkloadInitializer) newRequestDrivenWebServiceManifest(i *ServiceProps) *manifest.RequestDrivenWebService {
	props := &manifest.RequestDrivenWebServiceProps{
		WorkloadProps: &manifest.WorkloadProps{
			Name:       i.Name,
			Dockerfile: i.DockerfilePath,
			Image:      i.Image,
		},
		Port: i.Port,
	}
	return manifest.NewRequestDrivenWebService(props)
}

func newBackendServiceManifest(i *ServiceProps) (*manifest.BackendService, error) {
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
