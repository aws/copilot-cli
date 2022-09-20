// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	awscfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/elbv2"
	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy/patch"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	deploycfn "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	cfnstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/dustin/go-humanize/english"
	"golang.org/x/sync/errgroup"
)

type appResourcesGetter interface {
	GetAppResourcesByRegion(app *config.Application, region string) (*cfnstack.AppRegionalResources, error)
}

type environmentDeployer interface {
	UpdateAndRenderEnvironment(conf deploycfn.StackConfiguration, bucketARN string, opts ...cloudformation.StackOption) error
	DeployedEnvironmentParameters(app, env string) ([]*awscfn.Parameter, error)
	ForceUpdateOutputID(app, env string) (string, error)
}

type patcher interface {
	EnsureManagerRoleIsAllowedToUpload(bucketName string) error
}

type prefixListGetter interface {
	CloudFrontManagedPrefixListID() (string, error)
}

type envDescriber interface {
	ValidateCFServiceDomainAliases() error
	Params() (map[string]string, error)
}

type lbDescriber interface {
	ListenerRuleIsRedirect(context.Context, string) (bool, error)
}

type stackDescriber interface {
	Resources() ([]*stack.Resource, error)
}

const (
	svcStackResourceHTTPListenerRuleLogicalID = "HTTPListenerRuleWithDomain"
)

type envDeployer struct {
	app *config.Application
	env *config.Environment

	// Dependencies to upload artifacts.
	templateFS       template.Reader
	s3               uploader
	prefixListGetter prefixListGetter
	// Dependencies to deploy an environment.
	appCFN                   appResourcesGetter
	envDeployer              environmentDeployer
	patcher                  patcher
	newStackSerializer       func(input *deploy.CreateEnvironmentInput, forceUpdateID string, prevParams []*awscfn.Parameter) stackSerializer
	envDescriber             envDescriber
	envManagerSession        *session.Session
	newLBDescriber           func(*session.Session) lbDescriber
	newServiceStackDescriber func(string, *session.Session) stackDescriber

	// Cached variables.
	appRegionalResources *cfnstack.AppRegionalResources
}

// NewEnvDeployerInput contains information needed to construct an environment deployer.
type NewEnvDeployerInput struct {
	App             *config.Application
	Env             *config.Environment
	SessionProvider *sessions.Provider
	ConfigStore     describe.ConfigStoreSvc
}

// NewEnvDeployer constructs an environment deployer.
func NewEnvDeployer(in *NewEnvDeployerInput) (*envDeployer, error) {
	defaultSession, err := in.SessionProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("get default session: %w", err)
	}
	envRegionSession, err := in.SessionProvider.DefaultWithRegion(in.Env.Region)
	if err != nil {
		return nil, fmt.Errorf("get default session in env region %s: %w", in.Env.Region, err)
	}
	envManagerSession, err := in.SessionProvider.FromRole(in.Env.ManagerRoleARN, in.Env.Region)
	if err != nil {
		return nil, fmt.Errorf("get env session: %w", err)
	}
	envDescriber, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
		App:         in.App.Name,
		Env:         in.Env.Name,
		ConfigStore: in.ConfigStore,
	})
	cfnClient := deploycfn.New(envManagerSession, deploycfn.WithProgressTracker(os.Stderr))
	return &envDeployer{
		app: in.App,
		env: in.Env,

		templateFS:       template.New(),
		s3:               s3.New(envManagerSession),
		prefixListGetter: ec2.New(envRegionSession),

		appCFN:      deploycfn.New(defaultSession, deploycfn.WithProgressTracker(os.Stderr)),
		envDeployer: cfnClient,
		patcher: &patch.EnvironmentPatcher{
			Prog:            termprogress.NewSpinner(log.DiagnosticWriter),
			TemplatePatcher: cfnClient,
			Env:             in.Env,
		},
		newStackSerializer: func(in *deploy.CreateEnvironmentInput, lastForceUpdateID string, oldParams []*awscfn.Parameter) stackSerializer {
			return cfnstack.NewEnvConfigFromExistingStack(in, lastForceUpdateID, oldParams)
		},
		envDescriber:      envDescriber,
		envManagerSession: envManagerSession,
		newLBDescriber: func(sess *session.Session) lbDescriber {
			return elbv2.New(sess)
		},
		newServiceStackDescriber: func(svc string, sess *session.Session) stackDescriber {
			return stack.NewStackDescriber(cfnstack.NameForService(in.App.Name, in.Env.Name, svc), sess)
		},
	}, nil
}

// Verify checks that the manifest configuration is valid to deploy alongside
// the currently deployed resources.
func (d *envDeployer) Verify(ctx context.Context, mft *manifest.Environment) error {
	if mft.CDNEnabled() && mft.HTTPConfig.Public.Certificates == nil && d.app.Domain != "" {
		// With managed domain, if the customer isn't using `alias` the A-records are inserted in the service stack as each service domain is unique.
		// However, when clients enable CloudFront, they would need to update all their existing records to now point to the distribution.
		// Hence, we force users to use `alias` and let the records be written in the environment stack instead.
		if err := d.envDescriber.ValidateCFServiceDomainAliases(); err != nil {
			return err
		}
	}

	if mft.CDNEnabled() && aws.BoolValue(mft.CDNConfig.Config.TerminateTLS) {
		// ensure all services _are not_ doing http->https redirect
		if err := d.verifyALBWorkloadsDontRedirect(ctx); err != nil {
			return fmt.Errorf("can't enable TLS termination on CDN: %w", err)
		}
	}

	return nil
}

// verifyALBWorkloadsDontRedirect verifies that none of the ALB Workloads
// in this environment have a redirect in their HTTPWithDomain listener.
// If any services redirect, an error is returned.
func (d *envDeployer) verifyALBWorkloadsDontRedirect(ctx context.Context) error {
	params, err := d.envDescriber.Params()
	if err != nil {
		return fmt.Errorf("get env params: %w", err)
	}

	services := strings.Split(params[cfnstack.EnvParamALBWorkloadsKey], ",")
	g, ctx := errgroup.WithContext(ctx)

	var badServices []string
	var badServicesMu sync.Mutex

	for i := range services {
		svc := services[i]
		g.Go(func() error {
			redirects, err := d.lbServiceRedirects(ctx, svc)
			switch {
			case err != nil:
				return fmt.Errorf("verify service %q: %w", svc, err)
			case redirects:
				badServicesMu.Lock()
				defer badServicesMu.Unlock()
				badServices = append(badServices, svc)
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}
	if len(badServices) > 0 {
		sort.Strings(badServices)
		return fmt.Errorf("HTTP traffic redirects to HTTPS in %v %v.\nSet http.redirect_to_https to false for %v and redeploy %v.",
			english.PluralWord(len(badServices), "service", "services"),
			english.OxfordWordSeries(badServices, "and"),
			english.PluralWord(len(badServices), "that service", "those services"),
			english.PluralWord(len(badServices), "it", "them"),
		)
	}

	return nil
}

// lbServiceRedirects returns true if svc's HTTP listener rule redirects.
func (d *envDeployer) lbServiceRedirects(ctx context.Context, svc string) (bool, error) {
	stackDescriber := d.newServiceStackDescriber(svc, d.envManagerSession)
	resources, err := stackDescriber.Resources()
	if err != nil {
		return false, fmt.Errorf("get stack resources: %w", err)
	}

	ruleARN := ""
	for _, res := range resources {
		if res.LogicalID == svcStackResourceHTTPListenerRuleLogicalID {
			ruleARN = res.PhysicalID
		}
	}
	if ruleARN == "" {
		return false, fmt.Errorf("resource %q not present", svcStackResourceHTTPListenerRuleLogicalID)
	}

	lbDescriber := d.newLBDescriber(d.envManagerSession)
	isRedirect, err := lbDescriber.ListenerRuleIsRedirect(ctx, ruleARN)
	if err != nil {
		return false, fmt.Errorf("verify http listener doesn't redirect: %w", err)
	}
	return isRedirect, nil
}

// UploadArtifacts uploads the deployment artifacts for the environment.
func (d *envDeployer) UploadArtifacts() (map[string]string, error) {
	resources, err := d.getAppRegionalResources()
	if err != nil {
		return nil, err
	}
	if err := d.patcher.EnsureManagerRoleIsAllowedToUpload(resources.S3Bucket); err != nil {
		return nil, fmt.Errorf("ensure env manager role has permissions to upload: %w", err)
	}
	return d.uploadCustomResources(resources.S3Bucket)
}

func (d *envDeployer) uploadCustomResources(bucket string) (map[string]string, error) {
	crs, err := customresource.Env(d.templateFS)
	if err != nil {
		return nil, fmt.Errorf("read custom resources for environment %s: %w", d.env.Name, err)
	}
	urls, err := customresource.Upload(func(key string, dat io.Reader) (url string, err error) {
		return d.s3.Upload(bucket, key, dat)
	}, crs)
	if err != nil {
		return nil, fmt.Errorf("upload custom resources to bucket %s: %w", bucket, err)
	}
	return urls, nil
}

func (d *envDeployer) cidrPrefixLists(in *DeployEnvironmentInput) ([]string, error) {
	var cidrPrefixListIDs []string

	// Check if ingress is allowed from cloudfront
	if in.Manifest == nil || !in.Manifest.IsIngressRestrictedToCDN() {
		return nil, nil
	}
	cfManagedPrefixListID, err := d.cfManagedPrefixListID()
	if err != nil {
		return nil, err
	}
	cidrPrefixListIDs = append(cidrPrefixListIDs, cfManagedPrefixListID)

	return cidrPrefixListIDs, nil
}

func (d *envDeployer) cfManagedPrefixListID() (string, error) {
	id, err := d.prefixListGetter.CloudFrontManagedPrefixListID()
	if err != nil {
		return "", fmt.Errorf("retrieve CloudFront managed prefix list id: %w", err)
	}

	return id, nil
}

// DeployEnvironmentInput contains information used to deploy the environment.
type DeployEnvironmentInput struct {
	RootUserARN         string
	CustomResourcesURLs map[string]string
	Manifest            *manifest.Environment
	ForceNewUpdate      bool
	RawManifest         []byte
}

// GenerateCloudFormationTemplate returns the environment stack's template and parameter configuration.
func (d *envDeployer) GenerateCloudFormationTemplate(in *DeployEnvironmentInput) (*GenerateCloudFormationTemplateOutput, error) {
	stackInput, err := d.buildStackInput(in)
	if err != nil {
		return nil, err
	}
	oldParams, err := d.envDeployer.DeployedEnvironmentParameters(d.app.Name, d.env.Name)
	if err != nil {
		return nil, fmt.Errorf("describe environment stack parameters: %w", err)
	}
	lastForceUpdateID, err := d.envDeployer.ForceUpdateOutputID(d.app.Name, d.env.Name)
	if err != nil {
		return nil, fmt.Errorf("retrieve environment stack force update ID: %w", err)
	}
	stack := d.newStackSerializer(stackInput, lastForceUpdateID, oldParams)
	tpl, err := stack.Template()
	if err != nil {
		return nil, fmt.Errorf("generate stack template: %w", err)
	}
	params, err := stack.SerializedParameters()
	if err != nil {
		return nil, fmt.Errorf("generate stack template parameters: %w", err)
	}
	return &GenerateCloudFormationTemplateOutput{
		Template:   tpl,
		Parameters: params,
	}, nil
}

// DeployEnvironment deploys an environment using CloudFormation.
func (d *envDeployer) DeployEnvironment(in *DeployEnvironmentInput) error {
	stackInput, err := d.buildStackInput(in)
	if err != nil {
		return err
	}
	oldParams, err := d.envDeployer.DeployedEnvironmentParameters(d.app.Name, d.env.Name)
	if err != nil {
		return fmt.Errorf("describe environment stack parameters: %w", err)
	}
	lastForceUpdateID, err := d.envDeployer.ForceUpdateOutputID(d.app.Name, d.env.Name)
	if err != nil {
		return fmt.Errorf("retrieve environment stack force update ID: %w", err)
	}
	conf := cfnstack.NewEnvConfigFromExistingStack(stackInput, lastForceUpdateID, oldParams)
	return d.envDeployer.UpdateAndRenderEnvironment(conf, stackInput.ArtifactBucketARN, cloudformation.WithRoleARN(d.env.ExecutionRoleARN))
}

func (d *envDeployer) getAppRegionalResources() (*cfnstack.AppRegionalResources, error) {
	if d.appRegionalResources != nil {
		return d.appRegionalResources, nil
	}
	resources, err := d.appCFN.GetAppResourcesByRegion(d.app, d.env.Region)
	if err != nil {
		return nil, fmt.Errorf("get app resources in region %s: %w", d.env.Region, err)
	}
	if resources.S3Bucket == "" {
		return nil, fmt.Errorf("cannot find the S3 artifact bucket in region %s", d.env.Region)
	}
	return resources, nil
}

func (d *envDeployer) buildStackInput(in *DeployEnvironmentInput) (*deploy.CreateEnvironmentInput, error) {
	resources, err := d.getAppRegionalResources()
	if err != nil {
		return nil, err
	}
	partition, err := partitions.Region(d.env.Region).Partition()
	if err != nil {
		return nil, err
	}
	cidrPrefixListIDs, err := d.cidrPrefixLists(in)
	if err != nil {
		return nil, err
	}
	return &deploy.CreateEnvironmentInput{
		Name: d.env.Name,
		App: deploy.AppInformation{
			Name:                d.app.Name,
			Domain:              d.app.Domain,
			AccountPrincipalARN: in.RootUserARN,
		},
		AdditionalTags:       d.app.Tags,
		CustomResourcesURLs:  in.CustomResourcesURLs,
		ArtifactBucketARN:    s3.FormatARN(partition.ID(), resources.S3Bucket),
		ArtifactBucketKeyARN: resources.KMSKeyARN,
		CIDRPrefixListIDs:    cidrPrefixListIDs,
		Mft:                  in.Manifest,
		ForceUpdate:          in.ForceNewUpdate,
		RawMft:               in.RawManifest,
		Version:              deploy.LatestEnvTemplateVersion,
	}, nil
}
