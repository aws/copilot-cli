// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/acm"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudfront"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
)

var (
	ecsALBAliasUsedWithoutDomainFriendlyText = fmt.Sprintf(`To use %s, your application must be:
* Associated with a domain:
  %s
* Or, using imported certificates to your environment:
  %s
`,
		color.HighlightCode("http.alias"),
		color.HighlightCode("copilot app init --domain example.com"),
		color.HighlightCode("copilot env init --import-cert-arns arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012"))
	ecsNLBAliasUsedWithoutDomainFriendlyText = fmt.Sprintf("To use %s, your application must be associated with a domain: %s",
		color.HighlightCode("nlb.alias"),
		color.HighlightCode("copilot app init --domain example.com"))
)

type publicCIDRBlocksGetter interface {
	PublicCIDRBlocks() ([]string, error)
}

type lbWebSvcDeployer struct {
	*svcDeployer
	appVersionGetter       versionGetter
	publicCIDRBlocksGetter publicCIDRBlocksGetter
	lbMft                  *manifest.LoadBalancedWebService

	// Overriden in tests.
	newAliasCertValidator func(optionalRegion *string) aliasCertValidator
	newStack              func() cloudformation.StackConfiguration
}

// NewLBWSDeployer is the constructor for lbWebSvcDeployer.
func NewLBWSDeployer(in *WorkloadDeployerInput) (*lbWebSvcDeployer, error) {
	in.customResources = lbwsCustomResources
	svcDeployer, err := newSvcDeployer(in)
	if err != nil {
		return nil, err
	}
	versionGetter, err := describe.NewAppDescriber(in.App.Name)
	if err != nil {
		return nil, fmt.Errorf("new app describer for application %s: %w", in.App.Name, err)
	}
	deployStore, err := deploy.NewStore(in.SessionProvider, svcDeployer.store)
	if err != nil {
		return nil, fmt.Errorf("new deploy store: %w", err)
	}
	envDescriber, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
		App:         in.App.Name,
		Env:         in.Env.Name,
		ConfigStore: svcDeployer.store,
		DeployStore: deployStore,
	})
	if err != nil {
		return nil, fmt.Errorf("create describer for environment %s in application %s: %w", in.Env.Name, in.App.Name, err)
	}
	lbMft, ok := in.Mft.(*manifest.LoadBalancedWebService)
	if !ok {
		return nil, fmt.Errorf("manifest is not of type %s", manifestinfo.LoadBalancedWebServiceType)
	}
	return &lbWebSvcDeployer{
		svcDeployer:            svcDeployer,
		appVersionGetter:       versionGetter,
		publicCIDRBlocksGetter: envDescriber,
		lbMft:                  lbMft,
		newAliasCertValidator: func(optionalRegion *string) aliasCertValidator {
			sess := svcDeployer.envSess.Copy(&aws.Config{
				Region: optionalRegion,
			})
			return acm.New(sess)
		},
	}, nil
}

func lbwsCustomResources(fs template.Reader) ([]*customresource.CustomResource, error) {
	crs, err := customresource.LBWS(fs)
	if err != nil {
		return nil, fmt.Errorf("read custom resources for a %q: %w", manifestinfo.LoadBalancedWebServiceType, err)
	}
	return crs, nil
}

// IsServiceAvailableInRegion checks if service type exist in the given region.
func (lbWebSvcDeployer) IsServiceAvailableInRegion(region string) (bool, error) {
	return partitions.IsAvailableInRegion(awsecs.EndpointsID, region)
}

// UploadArtifacts uploads the deployment artifacts such as the container image, custom resources, addons and env files.
func (d *lbWebSvcDeployer) UploadArtifacts() (*UploadArtifactsOutput, error) {
	return d.uploadArtifacts()
}

// GenerateCloudFormationTemplate generates a CloudFormation template and parameters for a workload.
func (d *lbWebSvcDeployer) GenerateCloudFormationTemplate(in *GenerateCloudFormationTemplateInput) (
	*GenerateCloudFormationTemplateOutput, error) {
	output, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	return d.generateCloudFormationTemplate(output.conf)
}

// DeployWorkload deploys a load balanced web service using CloudFormation.
func (d *lbWebSvcDeployer) DeployWorkload(in *DeployWorkloadInput) (ActionRecommender, error) {
	stackConfigOutput, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	if err := d.deploy(in.Options, *stackConfigOutput); err != nil {
		return nil, err
	}
	return noopActionRecommender{}, nil
}

func (d *lbWebSvcDeployer) stackConfiguration(in *StackRuntimeConfiguration) (*svcStackConfigurationOutput, error) {
	rc, err := d.runtimeConfig(in)
	if err != nil {
		return nil, err
	}
	if err := d.validateALBRuntime(); err != nil {
		return nil, err
	}
	if err := d.validateNLBRuntime(); err != nil {
		return nil, err
	}
	var opts []stack.LoadBalancedWebServiceOption
	if !d.lbMft.NLBConfig.IsEmpty() {
		cidrBlocks, err := d.publicCIDRBlocksGetter.PublicCIDRBlocks()
		if err != nil {
			return nil, fmt.Errorf("get public CIDR blocks information from the VPC of environment %s: %w", d.env.Name, err)
		}
		opts = append(opts, stack.WithNLB(cidrBlocks))
	}

	var conf cloudformation.StackConfiguration
	switch {
	case d.newStack != nil:
		conf = d.newStack()
	default:
		conf, err = stack.NewLoadBalancedWebService(stack.LoadBalancedWebServiceConfig{
			App:                d.app,
			EnvManifest:        d.envConfig,
			Manifest:           d.lbMft,
			RawManifest:        d.rawMft,
			ArtifactBucketName: d.resources.S3Bucket,
			RuntimeConfig:      *rc,
			RootUserARN:        in.RootUserARN,
			Addons:             d.addons,
		}, opts...)
		if err != nil {
			return nil, fmt.Errorf("create stack configuration: %w", err)
		}
	}

	return &svcStackConfigurationOutput{
		conf: cloudformation.WrapWithTemplateOverrider(conf, d.overrider),
		svcUpdater: d.newSvcUpdater(func(s *session.Session) serviceForceUpdater {
			return ecs.New(s)
		}),
	}, nil
}

func (d *lbWebSvcDeployer) validateALBRuntime() error {
	hasALBCerts := len(d.envConfig.HTTPConfig.Public.Certificates) != 0
	hasCDNCerts := d.envConfig.CDNConfig.Config.Certificate != nil
	hasImportedCerts := hasALBCerts || hasCDNCerts
	if d.lbMft.RoutingRule.RedirectToHTTPS != nil && d.app.Domain == "" && !hasImportedCerts {
		return fmt.Errorf("cannot configure http to https redirect without having a domain associated with the app %q or importing any certificates in env %q", d.app.Name, d.env.Name)
	}
	if d.lbMft.RoutingRule.Alias.IsEmpty() {
		if hasImportedCerts {
			return &errSvcWithNoALBAliasDeployingToEnvWithImportedCerts{
				name:    d.name,
				envName: d.env.Name,
			}
		}
		return nil
	}
	importedHostedZones := d.lbMft.RoutingRule.Alias.HostedZones()
	if len(importedHostedZones) != 0 {
		if !hasImportedCerts {
			return fmt.Errorf("cannot specify alias hosted zones %v when no certificates are imported in environment %q", importedHostedZones, d.env.Name)
		}
		if d.envConfig.CDNEnabled() {
			return &errSvcWithALBAliasHostedZoneWithCDNEnabled{
				envName: d.env.Name,
			}
		}
	}
	if hasImportedCerts {
		aliases, err := d.lbMft.RoutingRule.Alias.ToStringSlice()
		if err != nil {
			return fmt.Errorf("convert aliases to string slice: %w", err)
		}

		if hasALBCerts {
			albCertValidator := d.newAliasCertValidator(nil)
			if err := albCertValidator.ValidateCertAliases(aliases, d.envConfig.HTTPConfig.Public.Certificates); err != nil {
				return fmt.Errorf("validate aliases against the imported public ALB certificate for env %s: %w", d.env.Name, err)
			}
		}
		if hasCDNCerts {
			cfCertValidator := d.newAliasCertValidator(aws.String(cloudfront.CertRegion))
			if err := cfCertValidator.ValidateCertAliases(aliases, []string{*d.envConfig.CDNConfig.Config.Certificate}); err != nil {
				return fmt.Errorf("validate aliases against the imported CDN certificate for env %s: %w", d.env.Name, err)
			}
		}
		return nil
	}
	if d.app.Domain != "" {
		if err := validateAppVersionForAlias(d.app.Name, d.appVersionGetter); err != nil {
			logAppVersionOutdatedError(aws.StringValue(d.lbMft.Name))
			return err
		}
		return validateLBWSAlias(d.lbMft.RoutingRule.Alias, d.app, d.env.Name)
	}
	log.Errorf(ecsALBAliasUsedWithoutDomainFriendlyText)
	return fmt.Errorf("cannot specify http.alias when application is not associated with a domain and env %s doesn't import one or more certificates", d.env.Name)
}

func (d *lbWebSvcDeployer) validateNLBRuntime() error {
	if d.lbMft.NLBConfig.Aliases.IsEmpty() {
		return nil
	}

	hasImportedCerts := len(d.envConfig.HTTPConfig.Public.Certificates) != 0
	if hasImportedCerts {
		return fmt.Errorf("cannot specify nlb.alias when env %s imports one or more certificates", d.env.Name)
	}
	if d.app.Domain == "" {
		log.Errorf(ecsNLBAliasUsedWithoutDomainFriendlyText)
		return fmt.Errorf("cannot specify nlb.alias when application is not associated with a domain")
	}
	if err := validateAppVersionForAlias(d.app.Name, d.appVersionGetter); err != nil {
		logAppVersionOutdatedError(aws.StringValue(d.lbMft.Name))
		return err
	}
	return validateLBWSAlias(d.lbMft.NLBConfig.Aliases, d.app, d.env.Name)
}

func validateLBWSAlias(aliases manifest.Alias, app *config.Application, envName string) error {
	if aliases.IsEmpty() {
		return nil
	}
	aliasList, err := aliases.ToStringSlice()
	if err != nil {
		return fmt.Errorf(`convert 'http.alias' to string slice: %w`, err)
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
