// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

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
	"github.com/aws/copilot-cli/internal/pkg/term/color"
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
	DescribeRule(context.Context, string) (elbv2.Rule, error)
}

type stackDescriber interface {
	Resources() ([]*stack.Resource, error)
}

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
	lbDescriber              lbDescriber
	newServiceStackDescriber func(string) stackDescriber

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
	if err != nil {
		return nil, fmt.Errorf("get env describer: %w", err)
	}
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
		lbDescriber:       elbv2.New(envManagerSession),
		newServiceStackDescriber: func(svc string) stackDescriber {
			return stack.NewStackDescriber(cfnstack.NameForService(in.App.Name, in.Env.Name, svc), envManagerSession)
		},
	}, nil
}

// Validate returns an error if the environment manifest is incompatible with services and application configurations.
func (d *envDeployer) Validate(ctx context.Context, mft *manifest.Environment, output io.Writer) error {
	isManagedCDNEnabled := mft.CDNEnabled() && !mft.HasImportedPublicALBCerts() && d.app.Domain != ""
	if isManagedCDNEnabled {
		// With managed domain, if the customer isn't using `alias` the A-records are inserted in the service stack as each service domain is unique.
		// However, when clients enable CloudFront, they would need to update all their existing records to now point to the distribution.
		// Hence, we force users to use `alias` and let the records be written in the environment stack instead.
		if err := d.envDescriber.ValidateCFServiceDomainAliases(); err != nil {
			return err
		}
	}

	if mft.CDNEnabled() && mft.CDNDoesTLSTermination() && mft.HasImportedPublicALBCerts() {
		err := d.validateALBWorkloadsDontRedirect(ctx)
		var redirErr *errEnvHasPublicServicesWithRedirect
		switch {
		case errors.As(err, &redirErr) && mft.ALBIngressRestrictedToCDN():
			return err
		case errors.As(err, &redirErr):
			fmt.Fprintf(output, redirErr.Warning())
		case err != nil:
			return fmt.Errorf("can't enable TLS termination on CDN: %w", err)
		}
	}

	return nil
}

type errEnvHasPublicServicesWithRedirect struct {
	services []string
}

func (e *errEnvHasPublicServicesWithRedirect) Error() string {
	return e.message()
}

func (e *errEnvHasPublicServicesWithRedirect) message() string {
	n := len(e.services)
	quoted := make([]string, len(e.services))
	for i := range e.services {
		quoted[i] = strconv.Quote(e.services[i])
	}

	return fmt.Sprintf(`%s %s %s HTTP traffic to HTTPS.
%s
To fix this, set the following field in %s manifest:
%s
and run %s.
	`,
		english.PluralWord(n, "Service", "Services"),
		english.OxfordWordSeries(quoted, "and"),
		english.PluralWord(n, "redirects", "redirect"),
		color.Emphasize(english.PluralWord(n, "This service", "These services")+" will not be reachable through the CDN."),
		english.PluralWord(n, "its", "each"),
		color.HighlightCodeBlock("http:\n  redirect_to_https: true"),
		color.HighlightCode("copilot svc deploy"),
	)

}

func (e *errEnvHasPublicServicesWithRedirect) Warning() string {
	return fmt.Sprintf(`%s
If you'd like to use %s without a CDN, ensure %s A record is pointed to the ALB.
`,
		e.message(),
		english.PluralWord(len(e.services), "this service", "these services"),
		english.PluralWord(len(e.services), "its", "each service's"),
	)
}

// validateALBWorkloadsDontRedirect verifies that none of the public ALB Workloads
// in this environment have a redirect in their HTTPWithDomain listener.
// If any services redirect, an error is returned.
func (d *envDeployer) validateALBWorkloadsDontRedirect(ctx context.Context) error {
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
		return &errEnvHasPublicServicesWithRedirect{
			services: badServices,
		}
	}

	return nil
}

// lbServiceRedirects returns true if svc's HTTP listener rule redirects. We only check
// HTTPListenerRuleWithDomain because HTTPListenerRule:
// a) doesn't ever redirect
// b) won't work with cloudfront anyways (can't point ALB default DNS to CF)
func (d *envDeployer) lbServiceRedirects(ctx context.Context, svc string) (bool, error) {
	stackDescriber := d.newServiceStackDescriber(svc)
	resources, err := stackDescriber.Resources()
	if err != nil {
		return false, fmt.Errorf("get stack resources: %w", err)
	}

	var ruleARN string
	for _, res := range resources {
		if res.LogicalID == template.LogicalIDHTTPListenerRuleWithDomain {
			ruleARN = res.PhysicalID
			break
		}
	}
	if ruleARN == "" {
		return false, fmt.Errorf("http listener not found on service %q", svc)
	}

	rule, err := d.lbDescriber.DescribeRule(ctx, ruleARN)
	if err != nil {
		return false, fmt.Errorf("get listener rule %q: %w", ruleARN, err)
	}
	return rule.HasRedirectAction(), nil
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
	PermissionsBoundary string
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
		PermissionsBoundary:  in.PermissionsBoundary,
		Version:              deploy.LatestEnvTemplateVersion,
	}, nil
}
