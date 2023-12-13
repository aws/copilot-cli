// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/elbv2"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/acm"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudfront"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
	"github.com/aws/copilot-cli/internal/pkg/config"
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
	"github.com/aws/copilot-cli/internal/pkg/version"
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

type elbGetter interface {
	LoadBalancer(nameOrARN string) (*elbv2.LoadBalancer, error)
}

type lbWebSvcDeployer struct {
	*svcDeployer
	appVersionGetter versionGetter
	elbGetter        elbGetter
	lbMft            *manifest.LoadBalancedWebService

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

	if err != nil {
		return nil, fmt.Errorf("create describer for environment %s in application %s: %w", in.Env.Name, in.App.Name, err)
	}
	lbMft, ok := in.Mft.(*manifest.LoadBalancedWebService)
	if !ok {
		return nil, fmt.Errorf("manifest is not of type %s", manifestinfo.LoadBalancedWebServiceType)
	}
	return &lbWebSvcDeployer{
		svcDeployer:      svcDeployer,
		appVersionGetter: versionGetter,
		elbGetter:        elbv2.New(svcDeployer.envSess),
		lbMft:            lbMft,
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
	return d.uploadArtifacts(d.buildAndPushContainerImages, d.uploadArtifactsToS3, d.uploadCustomResources)
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
	if d.lbMft.HTTPOrBool.ImportedALB != nil {
		lb, err := d.elbGetter.LoadBalancer(aws.StringValue(d.lbMft.HTTPOrBool.ImportedALB))
		if err != nil {
			return nil, err
		}
		opts = append(opts, stack.WithImportedALB(lb))
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
			ArtifactKey:        d.resources.KMSKeyARN,
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
	if d.lbMft.HTTPOrBool.Disabled() {
		return nil
	}

	if err := d.validateImportedALBConfig(); err != nil {
		return fmt.Errorf(`validate imported ALB configuration for "http": %w`, err)
	}

	if err := d.validateRuntimeRoutingRule(d.lbMft.HTTPOrBool.Main); err != nil {
		return fmt.Errorf(`validate ALB runtime configuration for "http": %w`, err)
	}

	for idx, rule := range d.lbMft.HTTPOrBool.AdditionalRoutingRules {
		if err := d.validateRuntimeRoutingRule(rule); err != nil {
			return fmt.Errorf(`validate ALB runtime configuration for "http.additional_rule[%d]": %w`, idx, err)
		}
	}
	return nil
}

func (d *lbWebSvcDeployer) validateImportedALBConfig() error {
	if d.lbMft.HTTPOrBool.ImportedALB == nil {
		return nil
	}
	alb, err := d.elbGetter.LoadBalancer(aws.StringValue(d.lbMft.HTTPOrBool.ImportedALB))
	if err != nil {
		return fmt.Errorf(`retrieve load balancer %q: %w`, aws.StringValue(d.lbMft.HTTPOrBool.ImportedALB), err)
	}
	if len(alb.Listeners) == 0 || len(alb.Listeners) > 2 {
		return fmt.Errorf(`imported ALB %q must have either one or two listeners`, alb.ARN)
	}
	if len(alb.Listeners) == 1 {
		return nil
	}
	var isHTTP, isHTTPS bool
	for _, listener := range alb.Listeners {
		if listener.Protocol == "HTTP" {
			isHTTP = true
		} else if listener.Protocol == "HTTPS" {
			isHTTPS = true
		}
	}
	if !(isHTTP && isHTTPS) {
		return fmt.Errorf("imported ALB must have listeners of protocol HTTP and HTTPS")
	}
	return nil
}

func (d *lbWebSvcDeployer) validateRuntimeRoutingRule(rule manifest.RoutingRule) error {
	hasALBCerts := len(d.envConfig.HTTPConfig.Public.Certificates) != 0
	hasCDNCerts := d.envConfig.CDNConfig.Config.Certificate != nil
	hasImportedCerts := hasALBCerts || hasCDNCerts
	if rule.RedirectToHTTPS != nil && d.app.Domain == "" && !hasImportedCerts {
		return fmt.Errorf("cannot configure http to https redirect without having a domain associated with the app %q or importing any certificates in env %q", d.app.Name, d.env.Name)
	}
	if rule.Alias.IsEmpty() {
		if hasImportedCerts && d.lbMft.HTTPOrBool.ImportedALB == nil {
			return &errSvcWithNoALBAliasDeployingToEnvWithImportedCerts{
				name:    d.name,
				envName: d.env.Name,
			}
		}
		return nil
	}
	importedHostedZones := rule.Alias.HostedZones()
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
		aliases, err := rule.Alias.ToStringSlice()
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
		err := validateMinAppVersion(d.app.Name, aws.StringValue(d.lbMft.Name), d.appVersionGetter, version.AppTemplateMinAlias)
		if err != nil {
			return fmt.Errorf("alias not supported: %w", err)
		}
		if err := validateLBWSAlias(rule.Alias, d.app, d.env.Name); err != nil {
			return fmt.Errorf(`validate 'alias': %w`, err)
		}
		return nil
	}
	log.Errorf(ecsALBAliasUsedWithoutDomainFriendlyText)
	return fmt.Errorf(`cannot specify "alias" when application is not associated with a domain and env %s doesn't import one or more certificates`, d.env.Name)
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
	err := validateMinAppVersion(d.app.Name, aws.StringValue(d.lbMft.Name), d.appVersionGetter, version.AppTemplateMinAlias)
	if err != nil {
		return fmt.Errorf("alias not supported: %w", err)
	}
	if err := validateLBWSAlias(d.lbMft.NLBConfig.Aliases, d.app, d.env.Name); err != nil {
		return fmt.Errorf(`validate 'nlb.alias': %w`, err)
	}
	return nil
}

func validateLBWSAlias(alias manifest.Alias, app *config.Application, envName string) error {
	if alias.IsEmpty() {
		return nil
	}

	aliases, err := alias.ToStringSlice()
	if err != nil {
		return err
	}

	return validateAliases(app, envName, aliases...)
}
