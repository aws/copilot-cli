// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/aws/tags"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/repository"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

type deployWkldVars struct {
	appName      string
	name         string
	envName      string
	imageTag     string
	resourceTags map[string]string
}

type deploySvcOpts struct {
	deployWkldVars

	store              store
	ws                 wsSvcDirReader
	imageBuilderPusher imageBuilderPusher
	unmarshal          func([]byte) (interface{}, error)
	s3                 artifactUploader
	cmd                runner
	addons             templater
	appCFN             appResourcesGetter
	svcCFN             cloudformation.CloudFormation
	sessProvider       sessionProvider
	envUpgradeCmd      actionCommand

	spinner progress
	sel     wsSelector
	prompt  prompter

	// cached variables
	targetApp         *config.Application
	targetEnvironment *config.Environment
	targetSvc         *config.Workload
	imageDigest       string
	buildRequired     bool
}

func newSvcDeployOpts(vars deployWkldVars) (*deploySvcOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}
	prompter := prompt.New()
	return &deploySvcOpts{
		deployWkldVars: vars,

		store:        store,
		ws:           ws,
		unmarshal:    manifest.UnmarshalWorkload,
		spinner:      termprogress.NewSpinner(log.DiagnosticWriter),
		sel:          selector.NewWorkspaceSelect(prompter, store, ws),
		prompt:       prompter,
		cmd:          exec.NewCmd(),
		sessProvider: sessions.NewProvider(),
	}, nil
}

// Validate returns an error if the user inputs are invalid.
func (o *deploySvcOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	if o.name != "" {
		if err := o.validateSvcName(); err != nil {
			return err
		}
	}
	if o.envName != "" {
		if err := o.validateEnvName(); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts the user for any required fields that are not provided.
func (o *deploySvcOpts) Ask() error {
	if err := o.askSvcName(); err != nil {
		return err
	}
	if err := o.askEnvName(); err != nil {
		return err
	}
	return nil
}

// Execute builds and pushes the container image for the service,
func (o *deploySvcOpts) Execute() error {
	o.imageTag = imageTagFromGit(o.cmd, o.imageTag) // Best effort assign git tag.
	env, err := targetEnv(o.store, o.appName, o.envName)
	if err != nil {
		return err
	}
	o.targetEnvironment = env

	app, err := o.store.GetApplication(o.appName)
	if err != nil {
		return err
	}
	o.targetApp = app

	svc, err := o.store.GetService(o.appName, o.name)
	if err != nil {
		return fmt.Errorf("get service configuration: %w", err)
	}
	o.targetSvc = svc

	if err := o.configureClients(); err != nil {
		return err
	}

	if err := o.envUpgradeCmd.Execute(); err != nil {
		return fmt.Errorf(`execute "env upgrade --app %s --name %s": %v`, o.appName, o.targetEnvironment.Name, err)
	}

	if err := o.configureContainerImage(); err != nil {
		return err
	}

	addonsURL, err := o.pushAddonsTemplateToS3Bucket()
	if err != nil {
		return err
	}

	if err := o.deploySvc(addonsURL); err != nil {
		return err
	}

	return o.showSvcURI()
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *deploySvcOpts) RecommendedActions() []string {
	return nil
}

func (o *deploySvcOpts) validateSvcName() error {
	names, err := o.ws.ServiceNames()
	if err != nil {
		return fmt.Errorf("list services in the workspace: %w", err)
	}
	for _, name := range names {
		if o.name == name {
			return nil
		}
	}
	return fmt.Errorf("service %s not found in the workspace", color.HighlightUserInput(o.name))
}

func (o *deploySvcOpts) validateEnvName() error {
	if _, err := targetEnv(o.store, o.appName, o.envName); err != nil {
		return err
	}
	return nil
}

func targetEnv(s store, appName, envName string) (*config.Environment, error) {
	env, err := s.GetEnvironment(appName, envName)
	if err != nil {
		return nil, fmt.Errorf("get environment %s configuration: %w", envName, err)
	}
	return env, nil
}

func (o *deploySvcOpts) askSvcName() error {
	if o.name != "" {
		return nil
	}

	name, err := o.sel.Service("Select a service in your workspace", "")
	if err != nil {
		return fmt.Errorf("select service: %w", err)
	}
	o.name = name
	return nil
}

func (o *deploySvcOpts) askEnvName() error {
	if o.envName != "" {
		return nil
	}

	name, err := o.sel.Environment("Select an environment", "", o.appName)
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.envName = name
	return nil
}

func (o *deploySvcOpts) configureClients() error {
	defaultSessEnvRegion, err := o.sessProvider.DefaultWithRegion(o.targetEnvironment.Region)
	if err != nil {
		return fmt.Errorf("create ECR session with region %s: %w", o.targetEnvironment.Region, err)
	}

	envSession, err := o.sessProvider.FromRole(o.targetEnvironment.ManagerRoleARN, o.targetEnvironment.Region)
	if err != nil {
		return fmt.Errorf("assuming environment manager role: %w", err)
	}

	// ECR client against tools account profile AND target environment region
	repoName := fmt.Sprintf("%s/%s", o.appName, o.name)
	registry := ecr.New(defaultSessEnvRegion)
	o.imageBuilderPusher, err = repository.New(repoName, registry)
	if err != nil {
		return fmt.Errorf("initiate image builder pusher: %w", err)
	}

	o.s3 = s3.New(defaultSessEnvRegion)

	// CF client against env account profile AND target environment region
	o.svcCFN = cloudformation.New(envSession)

	addonsSvc, err := addon.New(o.name)
	if err != nil {
		return fmt.Errorf("initiate addons service: %w", err)
	}
	o.addons = addonsSvc

	// client to retrieve an application's resources created with CloudFormation
	defaultSess, err := o.sessProvider.Default()
	if err != nil {
		return fmt.Errorf("create default session: %w", err)
	}
	o.appCFN = cloudformation.New(defaultSess)

	cmd, err := newEnvUpgradeOpts(envUpgradeVars{
		appName: o.appName,
		name:    o.targetEnvironment.Name,
	})
	if err != nil {
		return fmt.Errorf("new env upgrade command: %v", err)
	}
	o.envUpgradeCmd = cmd
	return nil
}

func (o *deploySvcOpts) configureContainerImage() error {
	svc, err := o.manifest()
	if err != nil {
		return err
	}
	required, err := manifest.ServiceDockerfileBuildRequired(svc)
	if err != nil {
		return err
	}
	if !required {
		return nil
	}
	// If it is built from local Dockerfile, build and push to the ECR repo.
	buildArg, err := o.dfBuildArgs(svc)
	if err != nil {
		return err
	}
	digest, err := o.imageBuilderPusher.BuildAndPush(exec.NewDockerCommand(), buildArg)
	if err != nil {
		return fmt.Errorf("build and push image: %w", err)
	}
	o.imageDigest = digest
	o.buildRequired = true
	return nil
}

func (o *deploySvcOpts) dfBuildArgs(svc interface{}) (*exec.BuildArguments, error) {
	copilotDir, err := o.ws.CopilotDirPath()
	if err != nil {
		return nil, fmt.Errorf("get copilot directory: %w", err)
	}
	return buildArgs(o.name, o.imageTag, copilotDir, svc)
}

func buildArgs(name, imageTag, copilotDir string, unmarshaledManifest interface{}) (*exec.BuildArguments, error) {
	type dfArgs interface {
		BuildArgs(rootDirectory string) *manifest.DockerBuildArgs
	}
	mf, ok := unmarshaledManifest.(dfArgs)
	if !ok {
		return nil, fmt.Errorf("%s does not have required method BuildArgs()", name)
	}
	var tags []string
	if imageTag != "" {
		tags = append(tags, imageTag)
	}
	args := mf.BuildArgs(filepath.Dir(copilotDir))
	return &exec.BuildArguments{
		Dockerfile: *args.Dockerfile,
		Context:    *args.Context,
		Args:       args.Args,
		CacheFrom:  args.CacheFrom,
		Target:     aws.StringValue(args.Target),
		Tags:       tags,
	}, nil
}

// pushAddonsTemplateToS3Bucket generates the addons template for the service and pushes it to S3.
// If the service doesn't have any addons, it returns the empty string and no errors.
// If the service has addons, it returns the URL of the S3 object storing the addons template.
func (o *deploySvcOpts) pushAddonsTemplateToS3Bucket() (string, error) {
	template, err := o.addons.Template()
	if err != nil {
		var notExistErr *addon.ErrAddonsDirNotExist
		if errors.As(err, &notExistErr) {
			// addons doesn't exist for service, the url is empty.
			return "", nil
		}
		return "", fmt.Errorf("retrieve addons template: %w", err)
	}
	resources, err := o.appCFN.GetAppResourcesByRegion(o.targetApp, o.targetEnvironment.Region)
	if err != nil {
		return "", fmt.Errorf("get app resources: %w", err)
	}

	reader := strings.NewReader(template)
	url, err := o.s3.PutArtifact(resources.S3Bucket, fmt.Sprintf(deploy.AddonsCfnTemplateNameFormat, o.name), reader)
	if err != nil {
		return "", fmt.Errorf("put addons artifact to bucket %s: %w", resources.S3Bucket, err)
	}
	return url, nil
}

func (o *deploySvcOpts) manifest() (interface{}, error) {
	raw, err := o.ws.ReadServiceManifest(o.name)
	if err != nil {
		return nil, fmt.Errorf("read service %s manifest file: %w", o.name, err)
	}
	mft, err := o.unmarshal(raw)
	if err != nil {
		return nil, fmt.Errorf("unmarshal service %s manifest: %w", o.name, err)
	}
	return mft, nil
}

func (o *deploySvcOpts) runtimeConfig(addonsURL string) (*stack.RuntimeConfig, error) {
	if !o.buildRequired {
		return &stack.RuntimeConfig{
			AddonsTemplateURL: addonsURL,
			AdditionalTags:    tags.Merge(o.targetApp.Tags, o.resourceTags),
		}, nil
	}
	resources, err := o.appCFN.GetAppResourcesByRegion(o.targetApp, o.targetEnvironment.Region)
	if err != nil {
		return nil, fmt.Errorf("get application %s resources from region %s: %w", o.targetApp.Name, o.targetEnvironment.Region, err)
	}
	repoURL, ok := resources.RepositoryURLs[o.name]
	if !ok {
		return nil, &errRepoNotFound{
			wlName:       o.name,
			envRegion:    o.targetEnvironment.Region,
			appAccountID: o.targetApp.AccountID,
		}
	}
	return &stack.RuntimeConfig{
		AddonsTemplateURL: addonsURL,
		AdditionalTags:    tags.Merge(o.targetApp.Tags, o.resourceTags),
		Image: &stack.ECRImage{
			RepoURL:  repoURL,
			ImageTag: o.imageTag,
			Digest:   o.imageDigest,
		},
	}, nil
}

func (o *deploySvcOpts) stackConfiguration(addonsURL string) (cloudformation.StackConfiguration, error) {
	mft, err := o.manifest()
	if err != nil {
		return nil, err
	}
	rc, err := o.runtimeConfig(addonsURL)
	if err != nil {
		return nil, err
	}
	var conf cloudformation.StackConfiguration
	switch t := mft.(type) {
	case *manifest.LoadBalancedWebService:
		if o.targetApp.RequiresDNSDelegation() {
			conf, err = stack.NewHTTPSLoadBalancedWebService(t, o.targetEnvironment.Name, o.targetEnvironment.App, *rc)
		} else {
			conf, err = stack.NewLoadBalancedWebService(t, o.targetEnvironment.Name, o.targetEnvironment.App, *rc)
		}
	case *manifest.RequestDrivenWebService:
		conf, err = stack.NewRequestDrivenWebService(t, o.targetEnvironment.Name, o.targetEnvironment.App, *rc)
	case *manifest.BackendService:
		conf, err = stack.NewBackendService(t, o.targetEnvironment.Name, o.targetEnvironment.App, *rc)
	default:
		return nil, fmt.Errorf("unknown manifest type %T while creating the CloudFormation stack", t)
	}
	if err != nil {
		return nil, fmt.Errorf("create stack configuration: %w", err)
	}
	return conf, nil
}

func (o *deploySvcOpts) deploySvc(addonsURL string) error {
	conf, err := o.stackConfiguration(addonsURL)
	if err != nil {
		return err
	}

	if err := o.svcCFN.DeployService(os.Stderr, conf, awscloudformation.WithRoleARN(o.targetEnvironment.ExecutionRoleARN)); err != nil {
		return fmt.Errorf("deploy service: %w", err)
	}
	return nil
}

func (o *deploySvcOpts) showSvcURI() error {
	type identifier interface {
		URI(string) (string, error)
	}

	var ecsSvcDescriber identifier
	var err error
	switch o.targetSvc.Type {
	case manifest.LoadBalancedWebServiceType:
		ecsSvcDescriber, err = describe.NewLBWebServiceDescriber(describe.NewLBWebServiceConfig{
			NewServiceConfig: describe.NewServiceConfig{
				App:         o.appName,
				Svc:         o.name,
				ConfigStore: o.store,
			},
		})
	case manifest.RequestDrivenWebServiceType:
		ecsSvcDescriber, err = describe.NewRDWebServiceDescriber(describe.NewRDWebServiceConfig{
			NewServiceConfig: describe.NewServiceConfig{
				App:         o.appName,
				Svc:         o.name,
				ConfigStore: o.store,
			},
		})
	case manifest.BackendServiceType:
		ecsSvcDescriber, err = describe.NewBackendServiceDescriber(describe.NewBackendServiceConfig{
			NewServiceConfig: describe.NewServiceConfig{
				App:         o.appName,
				Svc:         o.name,
				ConfigStore: o.store,
			},
		})
	default:
		err = errors.New("unexpected service type")
	}
	if err != nil {
		return fmt.Errorf("create describer for service type %s: %w", o.targetSvc.Type, err)
	}

	uri, err := ecsSvcDescriber.URI(o.targetEnvironment.Name)
	if err != nil {
		return fmt.Errorf("get uri for environment %s: %w", o.targetEnvironment.Name, err)
	}
	switch o.targetSvc.Type {
	case manifest.BackendServiceType:
		msg := fmt.Sprintf("Deployed %s.\n", color.HighlightUserInput(o.name))
		if uri != describe.BlankServiceDiscoveryURI {
			msg = fmt.Sprintf("Deployed %s, its service discovery endpoint is %s.\n", color.HighlightUserInput(o.name), color.HighlightResource(uri))
		}
		log.Success(msg)
	default:
		log.Successf("Deployed %s, you can access it at %s.\n", color.HighlightUserInput(o.name), color.HighlightResource(uri))
	}
	return nil
}

// buildSvcDeployCmd builds the `svc deploy` subcommand.
func buildSvcDeployCmd() *cobra.Command {
	vars := deployWkldVars{}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys a service to an environment.",
		Long:  `Deploys a service to an environment.`,
		Example: `
  Deploys a service named "frontend" to a "test" environment.
  /code $ copilot svc deploy --name frontend --env test
  Deploys a service with additional resource tags.
  /code $ copilot svc deploy --resource-tags source/revision=bb133e7,deployment/initiator=manual`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newSvcDeployOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Execute(); err != nil {
				return err
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringToStringVar(&vars.resourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)

	return cmd
}
