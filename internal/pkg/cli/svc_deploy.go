// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/aws/copilot-cli/internal/pkg/aws/identity"

	"github.com/aws/aws-sdk-go/aws"
	"golang.org/x/mod/semver"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/aws/tags"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
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
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	fmtForceUpdateSvcStart    = "Forcing an update for service %s from environment %s"
	fmtForceUpdateSvcFailed   = "Failed to force an update for service %s from environment %s: %v.\n"
	fmtForceUpdateSvcComplete = "Forced an update for service %s from environment %s.\n"
)

var aliasUsedWithoutDomainFriendlyText = fmt.Sprintf("To use %s, your application must be associated with a domain: %s.\n",
	color.HighlightCode("http.alias"),
	color.HighlightCode("copilot app init --domain example.com"))

type deployWkldVars struct {
	appName        string
	name           string
	envName        string
	imageTag       string
	resourceTags   map[string]string
	forceNewUpdate bool
}

type uploadCustomResourcesOpts struct {
	uploader      customResourcesUploader
	newS3Uploader func() (uploader, error)
}

type deploySvcOpts struct {
	deployWkldVars

	store               store
	deployStore         *deploy.Store
	ws                  wsSvcDirReader
	fs                  *afero.Afero
	imageBuilderPusher  imageBuilderPusher
	unmarshal           func([]byte) (manifest.WorkloadManifest, error)
	newInterpolator     func(app, env string) interpolator
	s3                  uploader
	cmd                 runner
	addons              templater
	appCFN              appResourcesGetter
	svcCFN              serviceDeployer
	newSvcUpdater       func(func(*session.Session) svcForceUpdater)
	sessProvider        sessionProvider
	envUpgradeCmd       actionCommand
	newAppVersionGetter func(string) (versionGetter, error)
	endpointGetter      endpointGetter
	snsTopicGetter      deployedEnvironmentLister
	identity            identityService
	subnetLister        vpcSubnetLister
	envDescriber        envDescriber

	spinner progress
	sel     wsSelector
	prompt  prompter

	// cached variables
	targetApp         *config.Application
	targetEnvironment *config.Environment
	targetSvc         *config.Workload
	appliedManifest   interface{}
	imageDigest       string
	buildRequired     bool
	addonsURL         string
	envFileARN        string
	appEnvResources   *stack.AppRegionalResources
	workspacePath     string
	rdSvcAlias        string
	svcUpdater        svcForceUpdater
	now               func() time.Time

	subscriptions []manifest.TopicSubscription

	uploadOpts *uploadCustomResourcesOpts
}

func newSvcDeployOpts(vars deployWkldVars) (*deploySvcOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}
	deployStore, err := deploy.NewStore(store)
	if err != nil {
		return nil, fmt.Errorf("new deploy store: %w", err)
	}
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}
	prompter := prompt.New()
	opts := &deploySvcOpts{
		deployWkldVars: vars,

		store:       store,
		deployStore: deployStore,
		ws:          ws,
		fs:          &afero.Afero{Fs: afero.NewOsFs()},
		unmarshal:   manifest.UnmarshalWorkload,
		spinner:     termprogress.NewSpinner(log.DiagnosticWriter),
		sel:         selector.NewWorkspaceSelect(prompter, store, ws),
		prompt:      prompter,
		newAppVersionGetter: func(appName string) (versionGetter, error) {
			d, err := describe.NewAppDescriber(appName)
			if err != nil {
				return nil, fmt.Errorf("new app describer for application %s: %w", appName, err)
			}
			return d, nil
		},
		newInterpolator: newManifestInterpolator,
		cmd:             exec.NewCmd(),
		now:             time.Now,
		sessProvider:    sessions.NewProvider(),
		snsTopicGetter:  deployStore,
	}
	opts.uploadOpts = newUploadCustomResourcesOpts(opts)
	return opts, err
}

func newManifestInterpolator(app, env string) interpolator {
	return manifest.NewInterpolator(app, env)
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

	if err := o.pushArtifactsToS3(); err != nil {
		return err
	}

	if err := o.deploySvc(); err != nil {
		return err
	}
	log.Successf("Deployed service %s.\n", color.HighlightUserInput(o.name))
	return nil
}

// RecommendActions returns follow-up actions the user can take after successfully executing the command.
func (o *deploySvcOpts) RecommendActions() error {
	var recommendations []string
	uriRecs, err := o.uriRecommendedActions()
	if err != nil {
		return err
	}
	recommendations = append(recommendations, uriRecs...)
	recommendations = append(recommendations, o.publishRecommendedActions()...)
	recommendations = append(recommendations, o.subscribeRecommendedActions()...)
	logRecommendedActions(recommendations)
	return nil
}

func (o *deploySvcOpts) validateSvcName() error {
	names, err := o.ws.ListServices()
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

	d, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
		App:         o.appName,
		Env:         o.envName,
		ConfigStore: o.store,
		DeployStore: o.deployStore,
	})
	if err != nil {
		return fmt.Errorf("create describer for environment %s in application %s: %w", o.envName, o.appName, err)
	}
	o.envDescriber = d
	o.subnetLister = ec2.New(envSession)

	// ECR client against tools account profile AND target environment region.
	repoName := fmt.Sprintf("%s/%s", o.appName, o.name)
	registry := ecr.New(defaultSessEnvRegion)
	o.imageBuilderPusher, err = repository.New(repoName, registry)
	if err != nil {
		return fmt.Errorf("initiate image builder pusher: %w", err)
	}

	o.s3 = s3.New(envSession)

	o.newSvcUpdater = func(f func(*session.Session) svcForceUpdater) {
		o.svcUpdater = f(envSession)
	}

	// CF client against env account profile AND target environment region.
	o.svcCFN = cloudformation.New(envSession)

	o.endpointGetter, err = describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
		App:         o.appName,
		Env:         o.envName,
		ConfigStore: o.store,
	})
	if err != nil {
		return fmt.Errorf("initiate env describer: %w", err)
	}
	addonsSvc, err := addon.New(o.name)
	if err != nil {
		return fmt.Errorf("initiate addons service: %w", err)
	}
	o.addons = addonsSvc

	// client to retrieve an application's resources created with CloudFormation.
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

	// client to retrieve caller identity.
	id := identity.New(defaultSess)
	o.identity = id
	return nil
}

func (o *deploySvcOpts) publicCIDRBlocks() ([]string, error) {
	envDescription, err := o.envDescriber.Describe()
	if err != nil {
		return nil, fmt.Errorf("describe environment %s: %w", o.envName, err)
	}
	vpcID := envDescription.EnvironmentVPC.ID
	subnets, err := o.subnetLister.ListVPCSubnets(vpcID)
	if err != nil {
		return nil, fmt.Errorf("list subnets of vpc %s in environment %s: %w", vpcID, o.envName, err)
	}
	var cidrBlocks []string
	for _, subnet := range subnets.Public {
		cidrBlocks = append(cidrBlocks, subnet.CIDRBlock)
	}
	return cidrBlocks, nil
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

	digest, err := o.imageBuilderPusher.BuildAndPush(dockerengine.New(exec.NewCmd()), buildArg)
	if err != nil {
		return fmt.Errorf("build and push image: %w", err)
	}
	o.imageDigest = digest
	o.buildRequired = true
	return nil
}

func (o *deploySvcOpts) dfBuildArgs(svc interface{}) (*dockerengine.BuildArguments, error) {
	if err := o.retrieveWorkspacePath(); err != nil {
		return nil, err
	}
	return buildArgs(o.name, o.imageTag, o.workspacePath, svc)
}

func (o *deploySvcOpts) pushArtifactsToS3() error {
	mft, err := o.manifest()
	if err != nil {
		return err
	}
	if err := o.pushEnvFilesToS3Bucket(envFile(mft)); err != nil {
		return err
	}
	return o.pushAddonsTemplateToS3Bucket()
}

func (o *deploySvcOpts) pushEnvFilesToS3Bucket(path string) error {
	if path == "" {
		return nil
	}
	if err := o.retrieveWorkspacePath(); err != nil {
		return err
	}
	content, err := o.fs.ReadFile(filepath.Join(o.workspacePath, path))
	if err != nil {
		return fmt.Errorf("read env file %s: %w", path, err)
	}
	if err := o.retrieveAppResourcesForEnvRegion(); err != nil {
		return err
	}
	reader := bytes.NewReader(content)
	url, err := o.s3.Upload(o.appEnvResources.S3Bucket, s3.MkdirSHA256(path, content), reader)
	if err != nil {
		return fmt.Errorf("put env file %s artifact to bucket %s: %w", path, o.appEnvResources.S3Bucket, err)
	}
	bucket, key, err := s3.ParseURL(url)
	if err != nil {
		return fmt.Errorf("parse s3 url: %w", err)
	}
	// The app and environment are always within the same partition.
	region := o.targetEnvironment.Region
	partition, err := partitions.Region(region).Partition()
	if err != nil {
		return err
	}
	o.envFileARN = s3.FormatARN(partition.ID(), fmt.Sprintf("%s/%s", bucket, key))
	return nil
}

func (o *deploySvcOpts) pushAddonsTemplateToS3Bucket() error {
	template, err := o.addons.Template()
	if err != nil {
		var notFoundErr *addon.ErrAddonsNotFound
		if errors.As(err, &notFoundErr) {
			// addons doesn't exist for service, the url is empty.
			return nil
		}
		return fmt.Errorf("retrieve addons template: %w", err)
	}
	if err := o.retrieveAppResourcesForEnvRegion(); err != nil {
		return err
	}
	reader := strings.NewReader(template)
	url, err := o.s3.Upload(o.appEnvResources.S3Bucket, fmt.Sprintf(deploy.AddonsCfnTemplateNameFormat, o.name), reader)
	if err != nil {
		return fmt.Errorf("put addons artifact to bucket %s: %w", o.appEnvResources.S3Bucket, err)
	}
	o.addonsURL = url
	return nil
}

func (o *deploySvcOpts) manifest() (interface{}, error) {
	if o.appliedManifest != nil {
		return o.appliedManifest, nil
	}

	raw, err := o.ws.ReadWorkloadManifest(o.name)
	if err != nil {
		return nil, fmt.Errorf("read service %s manifest file: %w", o.name, err)
	}
	interpolated, err := o.newInterpolator(o.appName, o.envName).Interpolate(string(raw))
	if err != nil {
		return nil, fmt.Errorf("interpolate environment variables for %s manifest: %w", o.name, err)
	}
	mft, err := o.unmarshal([]byte(interpolated))
	if err != nil {
		return nil, fmt.Errorf("unmarshal service %s manifest: %w", o.name, err)
	}
	envMft, err := mft.ApplyEnv(o.envName)
	if err != nil {
		return nil, fmt.Errorf("apply environment %s override: %s", o.envName, err)
	}
	if err := envMft.Validate(); err != nil {
		return nil, fmt.Errorf("validate manifest against environment %s: %s", o.envName, err)
	}
	o.appliedManifest = envMft // cache the results.
	return envMft, nil
}

func (o *deploySvcOpts) runtimeConfig() (*stack.RuntimeConfig, error) {
	endpoint, err := o.endpointGetter.ServiceDiscoveryEndpoint()
	if err != nil {
		return nil, err
	}

	if !o.buildRequired {
		return &stack.RuntimeConfig{
			AddonsTemplateURL:        o.addonsURL,
			EnvFileARN:               o.envFileARN,
			AdditionalTags:           tags.Merge(o.targetApp.Tags, o.resourceTags),
			ServiceDiscoveryEndpoint: endpoint,
			AccountID:                o.targetEnvironment.AccountID,
			Region:                   o.targetEnvironment.Region,
		}, nil
	}

	if err := o.retrieveAppResourcesForEnvRegion(); err != nil {
		return nil, err
	}

	repoURL, ok := o.appEnvResources.RepositoryURLs[o.name]
	if !ok {
		return nil, &errRepoNotFound{
			wlName:       o.name,
			envRegion:    o.targetEnvironment.Region,
			appAccountID: o.targetApp.AccountID,
		}
	}
	return &stack.RuntimeConfig{
		AddonsTemplateURL: o.addonsURL,
		EnvFileARN:        o.envFileARN,
		AdditionalTags:    tags.Merge(o.targetApp.Tags, o.resourceTags),
		Image: &stack.ECRImage{
			RepoURL:  repoURL,
			ImageTag: o.imageTag,
			Digest:   o.imageDigest,
		},
		ServiceDiscoveryEndpoint: endpoint,
		AccountID:                o.targetEnvironment.AccountID,
		Region:                   o.targetEnvironment.Region,
	}, nil
}

func uploadRDWSCustomResources(o *uploadCustomResourcesOpts, appEnvResources *stack.AppRegionalResources) (map[string]string, error) {
	s3Client, err := o.newS3Uploader()
	if err != nil {
		return nil, err
	}

	urls, err := o.uploader.UploadRequestDrivenWebServiceCustomResources(func(key string, objects ...s3.NamedBinary) (string, error) {
		return s3Client.ZipAndUpload(appEnvResources.S3Bucket, key, objects...)
	})
	if err != nil {
		return nil, fmt.Errorf("upload custom resources to bucket %s: %w", appEnvResources.S3Bucket, err)
	}

	return urls, nil
}

func uploadNLBWSCustomResources(o *uploadCustomResourcesOpts, appEnvResources *stack.AppRegionalResources) (map[string]string, error) {
	s3Client, err := o.newS3Uploader()
	if err != nil {
		return nil, err
	}

	urls, err := o.uploader.UploadNetworkLoadBalancedWebServiceCustomResources(func(key string, objects ...s3.NamedBinary) (string, error) {
		return s3Client.ZipAndUpload(appEnvResources.S3Bucket, key, objects...)
	})
	if err != nil {
		return nil, fmt.Errorf("upload custom resources to bucket %s: %w", appEnvResources.S3Bucket, err)
	}

	return urls, nil
}

func (o *deploySvcOpts) stackConfiguration() (cloudformation.StackConfiguration, error) {
	mft, err := o.manifest()
	if err != nil {
		return nil, err
	}
	rc, err := o.runtimeConfig()
	if err != nil {
		return nil, err
	}
	o.newSvcUpdater(func(s *session.Session) svcForceUpdater {
		return ecs.New(s)
	})
	var conf cloudformation.StackConfiguration
	switch t := mft.(type) {
	case *manifest.LoadBalancedWebService:
		if o.targetApp.Domain == "" && t.HasAliases() {
			log.Errorf(aliasUsedWithoutDomainFriendlyText)
			return nil, errors.New("alias specified when application is not associated with a domain")
		}

		var opts []stack.LoadBalancedWebServiceOption

		// TODO: https://github.com/aws/copilot-cli/issues/2918
		// 1. ALB block should not be executed if http is disabled
		if o.targetApp.RequiresDNSDelegation() {
			var appVersionGetter versionGetter
			if appVersionGetter, err = o.newAppVersionGetter(o.appName); err != nil {
				return nil, err
			}
			if err = validateLBSvcAliasAndAppVersion(aws.StringValue(t.Name), t.RoutingRule.Alias, o.targetApp, o.envName, appVersionGetter); err != nil {
				return nil, err
			}
			if err = validateLBSvcAliasAndAppVersion(aws.StringValue(t.Name), t.NLBConfig.Aliases, o.targetApp, o.envName, appVersionGetter); err != nil {
				return nil, err
			}
			opts = append(opts, stack.WithHTTPS())

			var caller identity.Caller
			caller, err = o.identity.Get()
			if err != nil {
				return nil, fmt.Errorf("get identity: %w", err)
			}
			opts = append(opts, stack.WithDNSDelegation(deploy.AppInformation{
				Name:                o.targetEnvironment.App,
				DNSName:             o.targetApp.Domain,
				AccountPrincipalARN: caller.RootUserARN,
			}))
		}
		if !t.NLBConfig.IsEmpty() {
			cidrBlocks, err := o.publicCIDRBlocks()
			if err != nil {
				return nil, err
			}
			opts = append(opts, stack.WithNLB(cidrBlocks))

			if o.targetApp.RequiresDNSDelegation() {
				if err = o.retrieveAppResourcesForEnvRegion(); err != nil {
					return nil, err
				}
				urls, err := uploadNLBWSCustomResources(o.uploadOpts, o.appEnvResources)
				if err != nil {
					return nil, err
				}
				opts = append(opts, stack.WithNLBCustomResources(urls))
			}
		}
		conf, err = stack.NewLoadBalancedWebService(t, o.targetEnvironment.Name, o.targetEnvironment.App, *rc, opts...)
	case *manifest.RequestDrivenWebService:
		if o.targetApp.Domain == "" && t.Alias != nil {
			log.Errorf(aliasUsedWithoutDomainFriendlyText)
			return nil, errors.New("alias specified when application is not associated with a domain")
		}
		o.newSvcUpdater(func(s *session.Session) svcForceUpdater {
			return apprunner.New(s)
		})
		var caller identity.Caller
		caller, err = o.identity.Get()
		if err != nil {
			return nil, fmt.Errorf("get identity: %w", err)
		}
		appInfo := deploy.AppInformation{
			Name:                o.targetEnvironment.App,
			DNSName:             o.targetApp.Domain,
			AccountPrincipalARN: caller.RootUserARN,
		}
		if t.Alias == nil {
			conf, err = stack.NewRequestDrivenWebService(t, o.targetEnvironment.Name, appInfo, *rc)
			break
		}

		o.rdSvcAlias = aws.StringValue(t.Alias)
		var (
			urls             map[string]string
			appVersionGetter versionGetter
		)
		if appVersionGetter, err = o.newAppVersionGetter(o.appName); err != nil {
			return nil, err
		}

		if err = validateRDSvcAliasAndAppVersion(o.name, aws.StringValue(t.Alias), o.envName, o.targetApp, appVersionGetter); err != nil {
			return nil, err
		}

		if err = o.retrieveAppResourcesForEnvRegion(); err != nil {
			return nil, err
		}
		if urls, err = uploadRDWSCustomResources(o.uploadOpts, o.appEnvResources); err != nil {
			return nil, err
		}
		conf, err = stack.NewRequestDrivenWebServiceWithAlias(t, o.targetEnvironment.Name, appInfo, *rc, urls)
	case *manifest.BackendService:
		conf, err = stack.NewBackendService(t, o.targetEnvironment.Name, o.targetEnvironment.App, *rc)
	case *manifest.WorkerService:
		var topics []deploy.Topic
		topics, err = o.snsTopicGetter.ListSNSTopics(o.appName, o.envName)
		if err != nil {
			return nil, fmt.Errorf("get SNS topics for app %s and environment %s: %w", o.appName, o.envName, err)
		}
		var topicARNs []string
		for _, topic := range topics {
			topicARNs = append(topicARNs, topic.ARN())
		}
		type subscriptions interface {
			Subscriptions() []manifest.TopicSubscription
		}

		subscriptionGetter, ok := mft.(subscriptions)
		if !ok {
			return nil, errors.New("manifest does not have required method Subscriptions")
		}
		// Cache the subscriptions for later.
		o.subscriptions = subscriptionGetter.Subscriptions()

		if err = validateTopicsExist(o.subscriptions, topicARNs, o.appName, o.envName); err != nil {
			return nil, err
		}
		conf, err = stack.NewWorkerService(t, o.targetEnvironment.Name, o.targetEnvironment.App, *rc)

	default:
		return nil, fmt.Errorf("unknown manifest type %T while creating the CloudFormation stack", t)
	}
	if err != nil {
		return nil, fmt.Errorf("create stack configuration: %w", err)
	}
	return conf, nil
}

func (o *deploySvcOpts) deploySvc() error {
	conf, err := o.stackConfiguration()
	if err != nil {
		return err
	}
	if err := o.retrieveAppResourcesForEnvRegion(); err != nil {
		return err
	}
	cmdRunAt := o.now()
	if err := o.svcCFN.DeployService(os.Stderr, conf, o.appEnvResources.S3Bucket, awscloudformation.WithRoleARN(o.targetEnvironment.ExecutionRoleARN)); err != nil {
		var errEmptyCS *awscloudformation.ErrChangeSetEmpty
		if !errors.As(err, &errEmptyCS) {
			return fmt.Errorf("deploy service: %w", err)
		}
		if !o.forceNewUpdate {
			log.Warningf("Set --%s to force an update for the service.\n", forceFlag)
			return fmt.Errorf("deploy service: %w", err)
		}
	}
	// Force update the service if --force is set and the service is not updated by the CFN.
	if o.forceNewUpdate {
		lastUpdatedAt, err := o.svcUpdater.LastUpdatedAt(o.appName, o.envName, o.name)
		if err != nil {
			return fmt.Errorf("get the last updated deployment time for %s: %w", o.name, err)
		}
		if cmdRunAt.After(lastUpdatedAt) {
			return o.forceDeploy()
		}
	}
	return nil
}

func (o *deploySvcOpts) forceDeploy() error {
	o.spinner.Start(fmt.Sprintf(fmtForceUpdateSvcStart, color.HighlightUserInput(o.name), color.HighlightUserInput(o.envName)))
	if err := o.svcUpdater.ForceUpdateService(o.appName, o.envName, o.name); err != nil {
		errLog := fmt.Sprintf(fmtForceUpdateSvcFailed, color.HighlightUserInput(o.name),
			color.HighlightUserInput(o.envName), err)
		var terr timeoutError
		if errors.As(err, &terr) {
			errLog = fmt.Sprintf("%s  Run %s to check for the fail reason.\n", errLog,
				color.HighlightCode(fmt.Sprintf("copilot svc status --name %s --env %s", o.name, o.envName)))
		}
		o.spinner.Stop(log.Serror(errLog))
		return fmt.Errorf("force an update for service %s: %w", o.name, err)
	}
	o.spinner.Stop(log.Ssuccessf(fmtForceUpdateSvcComplete, color.HighlightUserInput(o.name), color.HighlightUserInput(o.envName)))
	return nil
}

func buildArgs(name, imageTag, workspacePath string, unmarshaledManifest interface{}) (*dockerengine.BuildArguments, error) {
	type dfArgs interface {
		BuildArgs(rootDirectory string) *manifest.DockerBuildArgs
		ContainerPlatform() string
	}
	mf, ok := unmarshaledManifest.(dfArgs)
	if !ok {
		return nil, fmt.Errorf("%s does not have required methods BuildArgs() and ContainerPlatform()", name)
	}
	var tags []string
	if imageTag != "" {
		tags = append(tags, imageTag)
	}
	args := mf.BuildArgs(workspacePath)
	return &dockerengine.BuildArguments{
		Dockerfile: *args.Dockerfile,
		Context:    *args.Context,
		Args:       args.Args,
		CacheFrom:  args.CacheFrom,
		Target:     aws.StringValue(args.Target),
		Platform:   mf.ContainerPlatform(),
		Tags:       tags,
	}, nil
}

func envFile(unmarshaledManifest interface{}) string {
	type envFile interface {
		EnvFile() string
	}
	mf, ok := unmarshaledManifest.(envFile)
	if ok {
		return mf.EnvFile()
	}
	// If the manifest type doesn't support envFiles, ignore and move forward.
	return ""
}

func validateLBSvcAliasAndAppVersion(svcName string, aliases manifest.Alias, app *config.Application, envName string, appVersionGetter versionGetter) error {
	if aliases.IsEmpty() {
		return nil
	}
	aliasList, err := aliases.ToStringSlice()
	if err != nil {
		return fmt.Errorf(`convert 'http.alias' to string slice: %w`, err)
	}
	if err := validateAppVersion(app.Name, appVersionGetter); err != nil {
		logAppVersionOutdatedError(svcName)
		return err
	}
	for _, alias := range aliasList {
		// Alias should be within either env, app, or root hosted zone.
		var regEnvHostedZone, regAppHostedZone, regRootHostedZone *regexp.Regexp
		var err error
		if regEnvHostedZone, err = regexp.Compile(fmt.Sprintf(`^([^\.]+\.)?%s.%s.%s`, envName, app.Name, app.Domain)); err != nil {
			return err
		}
		if regAppHostedZone, err = regexp.Compile(fmt.Sprintf(`^([^\.]+\.)?%s.%s`, app.Name, app.Domain)); err != nil {
			return err
		}
		if regRootHostedZone, err = regexp.Compile(fmt.Sprintf(`^([^\.]+\.)?%s`, app.Domain)); err != nil {
			return err
		}
		var validAlias bool
		for _, re := range []*regexp.Regexp{regEnvHostedZone, regAppHostedZone, regRootHostedZone} {
			if re.MatchString(alias) {
				validAlias = true
				break
			}
		}
		if validAlias {
			continue
		}
		log.Errorf(`%s must match one of the following patterns:
- %s.%s.%s,
- <name>.%s.%s.%s,
- %s.%s,
- <name>.%s.%s,
- %s,
- <name>.%s
`, color.HighlightCode("http.alias"), envName, app.Name, app.Domain, envName,
			app.Name, app.Domain, app.Name, app.Domain, app.Name,
			app.Domain, app.Domain, app.Domain)
		return fmt.Errorf(`alias "%s" is not supported in hosted zones managed by Copilot`, alias)
	}
	return nil
}

func checkUnsupportedRDSvcAlias(alias, envName string, app *config.Application) error {
	var regEnvHostedZone, regAppHostedZone *regexp.Regexp
	var err error
	// Example: subdomain.env.app.domain, env.app.domain
	if regEnvHostedZone, err = regexp.Compile(fmt.Sprintf(`^([^\.]+\.)?%s.%s.%s`, envName, app.Name, app.Domain)); err != nil {
		return err
	}

	// Example: subdomain.app.domain, app.domain
	if regAppHostedZone, err = regexp.Compile(fmt.Sprintf(`^([^\.]+\.)?%s.%s`, app.Name, app.Domain)); err != nil {
		return err
	}

	if regEnvHostedZone.MatchString(alias) {
		return fmt.Errorf("%s is an environment-level alias, which is not supported yet", alias)
	}

	if regAppHostedZone.MatchString(alias) {
		return fmt.Errorf("%s is an application-level alias, which is not supported yet", alias)
	}

	if alias == app.Domain {
		return fmt.Errorf("%s is a root domain alias, which is not supported yet", alias)
	}

	return nil
}

func validateRDSvcAliasAndAppVersion(svcName, alias, envName string, app *config.Application, appVersionGetter versionGetter) error {
	if alias == "" {
		return nil
	}
	if err := validateAppVersion(app.Name, appVersionGetter); err != nil {
		logAppVersionOutdatedError(svcName)
		return err
	}
	// Alias should be within root hosted zone.
	aliasInvalidLog := fmt.Sprintf(`%s of %s field should match the pattern <subdomain>.%s 
Where <subdomain> cannot be the application name.
`, color.HighlightUserInput(alias), color.HighlightCode("http.alias"), app.Domain)
	if err := checkUnsupportedRDSvcAlias(alias, envName, app); err != nil {
		log.Errorf(aliasInvalidLog)
		return err
	}

	// Example: subdomain.domain
	regRootHostedZone, err := regexp.Compile(fmt.Sprintf(`^([^\.]+\.)%s`, app.Domain))
	if err != nil {
		return err
	}

	if regRootHostedZone.MatchString(alias) {
		return nil
	}

	log.Errorf(aliasInvalidLog)
	return fmt.Errorf("alias is not supported in hosted zones that are not managed by Copilot")
}

func validateAppVersion(appName string, appVersionGetter versionGetter) error {
	appVersion, err := appVersionGetter.Version()
	if err != nil {
		return fmt.Errorf("get version for app %s: %w", appName, err)
	}
	diff := semver.Compare(appVersion, deploy.AliasLeastAppTemplateVersion)
	if diff < 0 {
		return fmt.Errorf(`alias is not compatible with application versions below %s`, deploy.AliasLeastAppTemplateVersion)
	}
	return nil
}

func logAppVersionOutdatedError(name string) {
	log.Errorf(`Cannot deploy service %s because the application version is incompatible.
To upgrade the application, please run %s first (see https://aws.github.io/copilot-cli/docs/credentials/#application-credentials).
`, name, color.HighlightCode("copilot app upgrade"))
}

func newUploadCustomResourcesOpts(opts *deploySvcOpts) *uploadCustomResourcesOpts {
	return &uploadCustomResourcesOpts{
		uploader: template.New(),
		newS3Uploader: func() (uploader, error) {
			envRegion := opts.targetEnvironment.Region
			sess, err := opts.sessProvider.DefaultWithRegion(opts.targetEnvironment.Region)
			if err != nil {
				return nil, fmt.Errorf("create session with region %s: %w", envRegion, err)
			}
			s3Client := s3.New(sess)
			return s3Client, nil
		},
	}
}

func (o *deploySvcOpts) retrieveWorkspacePath() error {
	if o.workspacePath != "" {
		return nil
	}
	workspacePath, err := o.ws.Path()
	if err != nil {
		return fmt.Errorf("get workspace path: %w", err)
	}
	o.workspacePath = workspacePath
	return nil
}

func (o *deploySvcOpts) retrieveAppResourcesForEnvRegion() error {
	if o.appEnvResources != nil {
		return nil
	}
	resources, err := o.appCFN.GetAppResourcesByRegion(o.targetApp, o.targetEnvironment.Region)
	if err != nil {
		return fmt.Errorf("get application %s resources from region %s: %w", o.targetApp.Name, o.targetEnvironment.Region, err)
	}
	o.appEnvResources = resources
	return nil
}

func (o *deploySvcOpts) uriRecommendedActions() ([]string, error) {
	type reachable interface {
		Port() (uint16, bool)
	}
	mft, ok := o.appliedManifest.(reachable)
	if !ok {
		return nil, nil
	}
	if _, ok := mft.Port(); !ok { // No exposed port.
		return nil, nil
	}

	describer, err := describe.NewReachableService(o.appName, o.name, o.store)
	if err != nil {
		return nil, err
	}
	uri, err := describer.URI(o.targetEnvironment.Name)
	if err != nil {
		return nil, fmt.Errorf("get uri for environment %s: %w", o.targetEnvironment.Name, err)
	}

	network := "over the internet."
	if o.targetSvc.Type == manifest.BackendServiceType {
		network = "with service discovery."
	}
	recs := []string{
		fmt.Sprintf("You can access your service at %s %s", color.HighlightResource(uri), network),
	}
	if o.rdSvcAlias != "" {
		recs = append(recs, fmt.Sprintf(`The validation process for https://%s can take more than 15 minutes.
    Please visit %s to check the validation status.`, o.rdSvcAlias, color.Emphasize("https://console.aws.amazon.com/apprunner/home")))
	}
	return recs, nil
}

func (o *deploySvcOpts) subscribeRecommendedActions() []string {
	type subscriber interface {
		Subscriptions() []manifest.TopicSubscription
	}
	if _, ok := o.appliedManifest.(subscriber); !ok {
		return nil
	}
	retrieveEnvVarCode := "const eventsQueueURI = process.env.COPILOT_QUEUE_URI"
	actionRetrieveEnvVar := fmt.Sprintf(
		`Update %s's code to leverage the injected environment variable "COPILOT_QUEUE_URI".
    In JavaScript you can write %s.`,
		o.name,
		color.HighlightCode(retrieveEnvVarCode),
	)
	recs := []string{actionRetrieveEnvVar}
	topicQueueNames := o.buildWorkerQueueNames()
	if topicQueueNames == "" {
		return recs
	}
	retrieveTopicQueueEnvVarCode := fmt.Sprintf("const {%s} = JSON.parse(process.env.COPILOT_TOPIC_QUEUE_URIS)", topicQueueNames)
	actionRetrieveTopicQueues := fmt.Sprintf(
		`You can retrieve topic-specific queues by writing
    %s.`,
		color.HighlightCode(retrieveTopicQueueEnvVarCode),
	)
	recs = append(recs, actionRetrieveTopicQueues)
	return recs
}

func (o *deploySvcOpts) publishRecommendedActions() []string {
	type publisher interface {
		Publish() []manifest.Topic
	}
	mft, ok := o.appliedManifest.(publisher)
	if !ok {
		return nil
	}
	if topics := mft.Publish(); len(topics) == 0 {
		return nil
	}

	return []string{
		fmt.Sprintf(`Update %s's code to leverage the injected environment variable "COPILOT_SNS_TOPIC_ARNS".
    In JavaScript you can write %s.`,
			o.name,
			color.HighlightCode("const {<topicName>} = JSON.parse(process.env.COPILOT_SNS_TOPIC_ARNS)")),
	}
}

func (o *deploySvcOpts) buildWorkerQueueNames() string {
	sb := new(strings.Builder)
	first := true
	for _, subscription := range o.subscriptions {
		if subscription.Queue.IsEmpty() {
			continue
		}
		topicSvc := template.StripNonAlphaNumFunc(aws.StringValue(subscription.Service))
		topicName := template.StripNonAlphaNumFunc(aws.StringValue(subscription.Name))
		subName := fmt.Sprintf("%s%sEventsQueue", topicSvc, strings.Title(topicName))
		if first {
			sb.WriteString(subName)
			first = false
		} else {
			sb.WriteString(fmt.Sprintf(", %s", subName))
		}
	}
	return sb.String()
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
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringToStringVar(&vars.resourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)
	cmd.Flags().BoolVar(&vars.forceNewUpdate, forceFlag, false, forceFlagDescription)

	return cmd
}
