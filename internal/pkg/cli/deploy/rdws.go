// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/apprunner"
	awsapprunner "github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
)

var rdwsAliasUsedWithoutDomainFriendlyText = fmt.Sprintf("To use %s, your application must be associated with a domain: %s.\n",
	color.HighlightCode("http.alias"),
	color.HighlightCode("copilot app init --domain example.com"))

type rdwsDeployer struct {
	*svcDeployer
	customResourceS3Client uploader
	appVersionGetter       versionGetter
	rdwsMft                *manifest.RequestDrivenWebService
	customResources        customResourcesFunc
}

// NewRDWSDeployer is the constructor for RDWSDeployer.
func NewRDWSDeployer(in *WorkloadDeployerInput) (*rdwsDeployer, error) {
	svcDeployer, err := newSvcDeployer(in)
	if err != nil {
		return nil, err
	}
	svcDeployer.svcUpdater = apprunner.New(svcDeployer.envSess)
	versionGetter, err := describe.NewAppDescriber(in.App.Name)
	if err != nil {
		return nil, fmt.Errorf("new app describer for application %s: %w", in.App.Name, err)
	}
	rdwsMft, ok := in.Mft.(*manifest.RequestDrivenWebService)
	if !ok {
		return nil, fmt.Errorf("manifest is not of type %s", manifest.RequestDrivenWebServiceType)
	}
	return &rdwsDeployer{
		svcDeployer:            svcDeployer,
		customResourceS3Client: s3.New(svcDeployer.defaultSessWithEnvRegion),
		appVersionGetter:       versionGetter,
		rdwsMft:                rdwsMft,
		customResources: func(fs template.Reader) ([]*customresource.CustomResource, error) {
			crs, err := customresource.RDWS(fs)
			if err != nil {
				return nil, fmt.Errorf("read custom resources for a %q: %w", manifest.RequestDrivenWebServiceType, err)
			}
			return crs, nil
		},
	}, nil
}

// IsServiceAvailableInRegion checks if service type exist in the given region.
func (rdwsDeployer) IsServiceAvailableInRegion(region string) (bool, error) {
	return partitions.IsAvailableInRegion(awsapprunner.EndpointsID, region)
}

// UploadArtifacts uploads the deployment artifacts such as the container image, custom resources, addons and env files.
func (d *rdwsDeployer) UploadArtifacts() error {
	return d.uploadArtifacts(d.customResources)
}

type rdwsDeployOutput struct {
	rdwsAlias string
}

// RecommendedActions returns the recommended actions after deployment.
func (d *rdwsDeployOutput) RecommendedActions() []string {
	if d.rdwsAlias == "" {
		return nil
	}
	return []string{fmt.Sprintf(`The validation process for https://%s can take more than 15 minutes.
    Please visit %s to check the validation status.`, d.rdwsAlias, color.Emphasize("https://console.aws.amazon.com/apprunner/home"))}
}

func (d *rdwsDeployer) Stack(in StackRuntimeConfiguration) (Stack, error) {
	rc, err := d.runtimeConfig(in)
	if err != nil {
		return nil, err
	}

	if d.app.Domain == "" && d.rdwsMft.Alias != nil {
		log.Errorf(rdwsAliasUsedWithoutDomainFriendlyText)
		return nil, errors.New("alias specified when application is not associated with a domain")
	}

	conf, err := stack.NewRequestDrivenWebService(stack.RequestDrivenWebServiceConfig{
		App: deploy.AppInformation{
			Name:                d.app.Name,
			Domain:              d.app.Domain,
			PermissionsBoundary: d.app.PermissionsBoundary,
			AccountPrincipalARN: in.RootUserARN,
		},
		Env:           d.env.Name,
		Manifest:      d.rdwsMft,
		RawManifest:   d.rawMft,
		RuntimeConfig: *rc,
		Addons:        d.addons,
	})
	if err != nil {
		return nil, fmt.Errorf("create stack configuration: %w", err)
	}
	if d.rdwsMft.Alias == nil {
		return conf, nil
	}

	if err = validateRDSvcAliasAndAppVersion(d.name,
		aws.StringValue(d.rdwsMft.Alias), d.env.Name, d.app, d.appVersionGetter); err != nil {
		return nil, err
	}
	return conf, nil
}

func (d *rdwsDeployer) RecommendedActions() []string {
	if d.rdwsMft.Alias == nil {
		return nil
	}

	return []string{fmt.Sprintf(`The validation process for https://%s can take more than 15 minutes.
    Please visit %s to check the validation status.`, *d.rdwsMft.Alias, color.Emphasize("https://console.aws.amazon.com/apprunner/home"))}
}

func validateRDSvcAliasAndAppVersion(svcName, alias, envName string, app *config.Application, appVersionGetter versionGetter) error {
	if alias == "" {
		return nil
	}
	if err := validateAppVersionForAlias(app.Name, appVersionGetter); err != nil {
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
