// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package initialize contains methods and structs needed to initialize jobs and services.
package initialize

import (
	"encoding"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
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
	Rel(path string) (string, error)
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
	App                     string
	Type                    string
	Name                    string
	DockerfilePath          string
	Image                   string
	Platform                manifest.PlatformArgsOrString
	Topics                  []manifest.TopicSubscription
	Queue                   manifest.SQSQueue
	PrivateOnlyEnvironments []string
}

// JobProps contains the information needed to represent a Job.
type JobProps struct {
	WorkloadProps
	Schedule    string
	HealthCheck manifest.ContainerHealthCheck
	Timeout     string
	Retries     int
}

// ServiceProps contains the information needed to represent a Service (port, HealthCheck, and workload common props).
type ServiceProps struct {
	WorkloadProps
	Ports       []uint16
	HealthCheck manifest.ContainerHealthCheck
	Private     bool
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
		path, err := w.Ws.Rel(props.DockerfilePath)
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
	manifestMsgFmt := "Wrote the manifest for %s %s at %s\n"
	if manifestExists {
		manifestMsgFmt = "Manifest file for %s %s already exists at %s, skipping writing it.\n"
	}

	path := displayPath(manifestPath)
	log.Successf(manifestMsgFmt, jobWlType, color.HighlightUserInput(props.Name), color.HighlightResource(path))
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

	path, err = w.Ws.Rel(manifestPath)
	if err != nil {
		return "", err
	}
	return path, nil
}

func (w *WorkloadInitializer) initService(props *ServiceProps) (string, error) {
	if props.DockerfilePath != "" {
		path, err := w.Ws.Rel(props.DockerfilePath)
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

	manifestMsgFmt := "Wrote the manifest for %s %s at %s\n"
	if manifestExists {
		manifestMsgFmt = "Manifest file for %s %s already exists at %s, skipping writing it.\n"
	}

	path := displayPath(manifestPath)
	log.Successf(manifestMsgFmt, svcWlType, color.HighlightUserInput(props.Name), color.HighlightResource(path))

	helpText := "Your manifest contains configurations like your container size and port."
	if len(props.Ports) > 0 {
		helpText = fmt.Sprintf("Your manifest contains configurations like your container size and port (:%s).", strings.Trim(strings.Replace(fmt.Sprint(props.Ports), " ", " ", -1), "[]"))
	}
	log.Infoln(color.Help(helpText))
	log.Infoln()

	err = w.addSvcToAppAndSSM(app, props.WorkloadProps)
	if err != nil {
		return "", err
	}

	path, err = w.Ws.Rel(manifestPath)
	if err != nil {
		return "", err
	}
	return path, nil
}

func (w *WorkloadInitializer) addSvcToAppAndSSM(app *config.Application, props WorkloadProps) error {
	return w.addWlToAppAndSSM(app, props, svcWlType)
}

func (w *WorkloadInitializer) addJobToAppAndSSM(app *config.Application, props WorkloadProps) error {
	return w.addWlToAppAndSSM(app, props, jobWlType)
}

func (w *WorkloadInitializer) addWlToAppAndSSM(app *config.Application, props WorkloadProps, wlType string) error {
	if err := w.addWlToApp(app, props.Name, wlType); err != nil {
		return fmt.Errorf("add %s %s to application %s: %w", wlType, props.Name, props.App, err)
	}

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
	case manifestinfo.ScheduledJobType:
		return manifest.NewScheduledJob(&manifest.ScheduledJobProps{
			WorkloadProps: &manifest.WorkloadProps{
				Name:                    i.Name,
				Dockerfile:              i.DockerfilePath,
				Image:                   i.Image,
				PrivateOnlyEnvironments: i.PrivateOnlyEnvironments,
			},
			HealthCheck: i.HealthCheck,
			Platform:    i.Platform,
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
	case manifestinfo.LoadBalancedWebServiceType:
		return w.newLoadBalancedWebServiceManifest(i)
	case manifestinfo.RequestDrivenWebServiceType:
		return w.newRequestDrivenWebServiceManifest(i), nil
	case manifestinfo.BackendServiceType:
		return newBackendServiceManifest(i)
	case manifestinfo.WorkerServiceType:
		return newWorkerServiceManifest(i)
	case manifestinfo.StaticSiteType:
		return manifest.NewStaticSite(i.Name), nil
	default:
		return nil, fmt.Errorf("service type %s doesn't have a manifest", i.Type)
	}
}

func (w *WorkloadInitializer) newLoadBalancedWebServiceManifest(inProps *ServiceProps) (*manifest.LoadBalancedWebService, error) {
	/*var httpVersion string
	if inProps.Ports[0] == commonGRPCPort { // we set protocol to gRPC if the port is 50051 which we will continue to do for main listener rule hence inProps.Ports[0].
		log.Infof("Detected port %s, setting HTTP protocol version to %s in the manifest.\n",
			color.HighlightUserInput(strconv.Itoa(int(inProps.Ports[0]))), color.HighlightCode(manifest.GRPCProtocol))
		httpVersion = manifest.GRPCProtocol
	}*/
	outProps := &manifest.LoadBalancedWebServiceProps{
		WorkloadProps: &manifest.WorkloadProps{
			Name:                    inProps.Name,
			Dockerfile:              inProps.DockerfilePath,
			Image:                   inProps.Image,
			PrivateOnlyEnvironments: inProps.PrivateOnlyEnvironments,
		},
		Path:  "/",
		Ports: inProps.Ports,
		//HTTPVersion: httpVersion,
		HealthCheck: inProps.HealthCheck,
		Platform:    inProps.Platform,
	}
	existingSvcs, err := w.Store.ListServices(inProps.App)
	if err != nil {
		return nil, err
	}
	// We default to "/" for the first service or if the application is initialized with a domain, but if there's another
	// Load Balanced Web Service, we use the svc name as the default, instead.
	if aws.StringValue(inProps.appDomain) == "" {
		for _, existingSvc := range existingSvcs {
			if existingSvc.Type == manifestinfo.LoadBalancedWebServiceType && existingSvc.Name != inProps.Name {
				outProps.Path = inProps.Name
				break
			}
		}
	}
	return manifest.NewLoadBalancedWebService(outProps), nil
}

func (w *WorkloadInitializer) newRequestDrivenWebServiceManifest(i *ServiceProps) *manifest.RequestDrivenWebService {
	props := &manifest.RequestDrivenWebServiceProps{
		WorkloadProps: &manifest.WorkloadProps{
			Name:       i.Name,
			Dockerfile: i.DockerfilePath,
			Image:      i.Image,
		},
		Port:     i.Ports[0],
		Platform: i.Platform,
		Private:  i.Private,
	}
	return manifest.NewRequestDrivenWebService(props)
}

func newBackendServiceManifest(i *ServiceProps) (*manifest.BackendService, error) {
	return manifest.NewBackendService(manifest.BackendServiceProps{
		WorkloadProps: manifest.WorkloadProps{
			Name:                    i.Name,
			Dockerfile:              i.DockerfilePath,
			Image:                   i.Image,
			PrivateOnlyEnvironments: i.PrivateOnlyEnvironments,
		},
		Port:        i.Ports[0],
		HealthCheck: i.HealthCheck,
		Platform:    i.Platform,
	}), nil
}

func newWorkerServiceManifest(i *ServiceProps) (*manifest.WorkerService, error) {
	return manifest.NewWorkerService(manifest.WorkerServiceProps{
		WorkloadProps: manifest.WorkloadProps{
			Name:                    i.Name,
			Dockerfile:              i.DockerfilePath,
			Image:                   i.Image,
			PrivateOnlyEnvironments: i.PrivateOnlyEnvironments,
		},
		HealthCheck: i.HealthCheck,
		Platform:    i.Platform,
		Topics:      i.Topics,
		Queue:       i.Queue,
	}), nil
}

// Copy of cli.displayPath
func displayPath(target string) string {
	if !filepath.IsAbs(target) {
		return filepath.Clean(target)
	}

	base, err := os.Getwd()
	if err != nil {
		return filepath.Clean(target)
	}

	rel, err := filepath.Rel(base, target)
	if err != nil {
		// No path from base to target available, return target as is.
		return filepath.Clean(target)
	}
	return rel
}
