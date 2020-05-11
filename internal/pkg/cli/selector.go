package cli

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
)

// selector prompts users to select an app, environments, or service.
// We can use this instead of repeating our own selectors in our own
// commands.
type selector struct {
	prompt prompter
}

type selectWorkspaceServiceRequest struct {
	Prompt string          // instructs users on what to select
	Help   string          // provides help text on what users are selecting
	Lister wsServiceLister // lists local services
}

type selectServiceRequest struct {
	Prompt string        // instructs users on what to select
	Help   string        // provides help text on what users are selecting
	App    string        // the app whose services you'd like to list
	Lister serviceLister // lists all services for an app
}

type selectEnvRequest struct {
	Prompt string            // instructs users on what to select
	Help   string            // provides help text on what users are selecting
	App    string            // the app whose envs you'd like to list
	Lister environmentLister // lists all envs in an app
}

type selectAppRequest struct {
	Prompt string            // instructs users on what to select
	Help   string            // provides help text on what users are selecting
	Lister applicationLister // lists all apps in an account and region
}

// SelectWorkspaceService fetches all services in the workspace and then prompts the user to select one.
func (s *selector) SelectWorkspaceService(req *selectWorkspaceServiceRequest) (string, error) {
	serviceNames, err := s.retrieveWorkspaceServices(req.Lister)
	if err != nil {
		return "", fmt.Errorf("list services: %w", err)
	}
	if len(serviceNames) == 1 {
		log.Infof("Only found one service, defaulting to: %s\n", color.HighlightUserInput(serviceNames[0]))
		return serviceNames[0], nil
	}

	selectedServiceName, err := s.prompt.SelectOne(req.Prompt, req.Help, serviceNames)
	if err != nil {
		return "", fmt.Errorf("select local service: %w", err)
	}
	return selectedServiceName, nil
}

// SelectService fetches all services in an app and prompts the user to select one.
func (s *selector) SelectService(req *selectServiceRequest) (string, error) {
	services, err := s.retrieveServices(req.App, req.Lister)
	if err != nil {
		return "", fmt.Errorf("get services for app %s: %w", req.App, err)
	}
	if len(services) == 0 {
		log.Infof("Couldn't find any services associated with app %s, try initializing one: %s\n",
			color.HighlightUserInput(req.App),
			color.HighlightCode("copilot svc init"))
		return "", fmt.Errorf("no services found in app %s", req.App)
	}
	if len(services) == 1 {
		log.Infof("Only found one service, defaulting to: %s\n", color.HighlightUserInput(services[0]))
		return services[0], nil
	}
	selectedAppName, err := s.prompt.SelectOne(req.Prompt,
		req.Help,
		services)
	if err != nil {
		return "", fmt.Errorf("select service: %w", err)
	}
	return selectedAppName, nil
}

// SelectEnvironment fetches all the environments in an app and prompts the user to select one.
func (s *selector) SelectEnvironment(req *selectEnvRequest) (string, error) {
	envs, err := s.retrieveEnvironments(req.App, req.Lister)
	if err != nil {
		return "", fmt.Errorf("get environments for app %s from metadata store: %w", req.App, err)
	}
	if len(envs) == 0 {
		log.Infof("Couldn't find any environments associated with app %s, try initializing one: %s\n",
			color.HighlightUserInput(req.App),
			color.HighlightCode("copilot env init"))
		return "", fmt.Errorf("no environments found in app %s", req.App)
	}
	if len(envs) == 1 {
		log.Infof("Only found one environment, defaulting to: %s\n", color.HighlightUserInput(envs[0]))
		return envs[0], nil
	}
	selectedEnvName, err := s.prompt.SelectOne(req.Prompt,
		req.Help,
		envs)
	if err != nil {
		return "", fmt.Errorf("select environment: %w", err)
	}
	return selectedEnvName, nil
}

// SelectApplication fetches all the apps in an account/region and prompts the user to select one.
func (s *selector) SelectApplication(req *selectAppRequest) (string, error) {
	appNames, err := s.retrieveApps(req.Lister)
	if err != nil {
		return "", err
	}
	if len(appNames) == 0 {
		log.Infof("Couldn't find any apps in this region and account. Try initializing one with %s\n",
			color.HighlightCode("copilot app init"))
		return "", fmt.Errorf("no apps found")

	}

	if len(appNames) == 1 {
		log.Infof("Only found one app, defaulting to: %s\n", color.HighlightUserInput(appNames[0]))
		return appNames[0], nil
	}

	proj, err := s.prompt.SelectOne(
		req.Prompt,
		req.Help,
		appNames,
	)

	if err != nil {
		return "", fmt.Errorf("select app: %w", err)
	}

	return proj, nil
}

func (s *selector) retrieveApps(lister applicationLister) ([]string, error) {
	apps, err := lister.ListApplications()
	if err != nil {
		return nil, fmt.Errorf("list apps: %w", err)
	}
	appNames := make([]string, len(apps))
	for ind, app := range apps {
		appNames[ind] = app.Name
	}
	return appNames, nil
}

func (s *selector) retrieveEnvironments(App string, lister environmentLister) ([]string, error) {
	envs, err := lister.ListEnvironments(App)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	envsNames := make([]string, len(envs))
	for ind, env := range envs {
		envsNames[ind] = env.Name
	}
	return envsNames, nil
}

func (s *selector) retrieveServices(appName string, lister serviceLister) ([]string, error) {
	services, err := lister.ListServices(appName)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	serviceNames := make([]string, len(services))
	for ind, service := range services {
		serviceNames[ind] = service.Name
	}
	return serviceNames, nil
}

func (s *selector) retrieveWorkspaceServices(lister wsServiceLister) ([]string, error) {
	localServiceNames, err := lister.ServiceNames()
	if err != nil {
		return nil, err
	}
	if len(localServiceNames) == 0 {
		return nil, errors.New("no services found in workspace")
	}
	return localServiceNames, nil
}
