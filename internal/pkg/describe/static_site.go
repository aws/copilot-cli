// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	awsS3 "github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	s3 "github.com/aws/copilot-cli/internal/pkg/s3"

	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/dustin/go-humanize/english"
)

const (
	staticSiteOutputCFDomainName    = "CloudFrontDistributionDomainName"
	staticSiteOutputCFAltDomainName = "CloudFrontDistributionAlternativeDomainName"
)

// StaticSiteDescriber retrieves information about a static site service.
type StaticSiteDescriber struct {
	app string
	svc string

	enableResources        bool
	store                  DeployedEnvServicesLister
	initWkldStackDescriber func(string) (workloadDescriber, error)
	wkldDescribers         map[string]workloadDescriber
	initS3Client           func(string) (bucketDescriber, bucketNameGetter, error)
}

// NewStaticSiteDescriber instantiates a static site service describer.
func NewStaticSiteDescriber(opt NewServiceConfig) (*StaticSiteDescriber, error) {
	describer := &StaticSiteDescriber{
		app:             opt.App,
		svc:             opt.Svc,
		enableResources: opt.EnableResources,
		store:           opt.DeployStore,
		wkldDescribers:  make(map[string]workloadDescriber),
	}
	describer.initWkldStackDescriber = func(env string) (workloadDescriber, error) {
		if describer, ok := describer.wkldDescribers[env]; ok {
			return describer, nil
		}
		svcDescr, err := NewWorkloadStackDescriber(NewWorkloadConfig{
			App:         opt.App,
			Env:         env,
			Name:        opt.Svc,
			ConfigStore: opt.ConfigStore,
		})
		if err != nil {
			return nil, err
		}
		describer.wkldDescribers[env] = svcDescr
		return svcDescr, nil
	}
	describer.initS3Client = func(env string) (bucketDescriber, bucketNameGetter, error) {
		environment, err := opt.ConfigStore.GetEnvironment(opt.App, env)
		if err != nil {
			return nil, nil, fmt.Errorf("get environment %s: %w", env, err)
		}
		sess, err := sessions.ImmutableProvider().FromRole(environment.ManagerRoleARN, environment.Region)
		if err != nil {
			return nil, nil, err
		}
		return awsS3.New(sess), s3.New(sess), nil
	}
	return describer, nil
}

// URI returns the public accessible URI of a static site service.
func (d *StaticSiteDescriber) URI(envName string) (URI, error) {
	wkldDescr, err := d.initWkldStackDescriber(envName)
	if err != nil {
		return URI{}, err
	}
	outputs, err := wkldDescr.Outputs()
	if err != nil {
		return URI{}, fmt.Errorf("get stack output for service %q: %w", d.svc, err)
	}
	uri := accessURI{
		HTTPS:    true,
		DNSNames: []string{outputs[staticSiteOutputCFDomainName]},
	}
	if outputs[staticSiteOutputCFAltDomainName] != "" {
		uri.DNSNames = append(uri.DNSNames, outputs[staticSiteOutputCFAltDomainName])
	}
	return URI{
		URI:        english.OxfordWordSeries(uri.strings(), "or"),
		AccessType: URIAccessTypeInternet,
	}, nil
}

// Describe returns info of a static site.
func (d *StaticSiteDescriber) Describe() (HumanJSONStringer, error) {
	environments, err := d.store.ListEnvironmentsDeployedTo(d.app, d.svc)
	if err != nil {
		return nil, fmt.Errorf("list deployed environments for service %q: %w", d.svc, err)
	}
	var routes []*WebServiceRoute
	var objects []*S3ObjectTree
	for _, env := range environments {
		bucketDescriber, bucketNameDescriber, err := d.initS3Client(env)
		if err != nil {
			return nil, err
		}
		uri, err := d.URI(env)
		if err != nil {
			return nil, fmt.Errorf("retrieve service URI: %w", err)
		}
		if uri.AccessType == URIAccessTypeInternet {
			routes = append(routes, &WebServiceRoute{
				Environment: env,
				URL:         uri.URI,
			})
		}
		bucketName, err := bucketNameDescriber.BucketName(d.app, env, d.svc)
		if err != nil {
			return nil, fmt.Errorf("get bucket name for %q env: %w", env, err)
		}
		tree, err := bucketDescriber.BucketTree(bucketName)
		if err != nil {
			return nil, fmt.Errorf("get tree representation of bucket contents: %w", err)
		}
		if tree != "" {
			objects = append(objects, &S3ObjectTree{
				Environment: env,
				Tree:        tree,
			})
		}
	}

	resources := make(map[string][]*stack.Resource)
	if d.enableResources {
		for _, env := range environments {
			svcDescr, err := d.initWkldStackDescriber(env)
			if err != nil {
				return nil, err
			}
			stackResources, err := svcDescr.StackResources()
			if err != nil {
				return nil, fmt.Errorf("retrieve service resources: %w", err)
			}
			resources[env] = stackResources
		}
	}
	return &staticSiteDesc{
		Service:   d.svc,
		Type:      manifestinfo.StaticSiteType,
		App:       d.app,
		Routes:    routes,
		Resources: resources,
		Objects:   objects,

		environments: environments,
	}, nil
}

// Manifest returns the contents of the manifest used to deploy a static site stack.
// If the Manifest metadata doesn't exist in the stack template, then returns ErrManifestNotFoundInTemplate.
func (d *StaticSiteDescriber) Manifest(env string) ([]byte, error) {
	cfn, err := d.initWkldStackDescriber(env)
	if err != nil {
		return nil, err
	}
	return cfn.Manifest()
}

// S3ObjectTree contains serialized parameters for an S3 object tree.
type S3ObjectTree struct {
	Environment string
	Tree        string
}

// staticSiteDesc contains serialized parameters for a static site.
type staticSiteDesc struct {
	Service   string               `json:"service"`
	Type      string               `json:"type"`
	App       string               `json:"application"`
	Routes    []*WebServiceRoute   `json:"routes"`
	Objects   []*S3ObjectTree      `json:"objects,omitempty"`
	Resources deployedSvcResources `json:"resources,omitempty"`

	environments []string `json:"-"`
}

// JSONString returns the stringified backendService struct with json format.
func (w *staticSiteDesc) JSONString() (string, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return "", fmt.Errorf("marshal static site description: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified backendService struct with human readable format.
func (w *staticSiteDesc) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprint(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Application", w.App)
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", w.Service)
	fmt.Fprintf(writer, "  %s\t%s\n", "Type", w.Type)
	if len(w.Routes) > 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nRoutes\n\n"))
		writer.Flush()
		headers := []string{"Environment", "URL"}
		fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
		fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
		for _, route := range w.Routes {
			fmt.Fprintf(writer, "  %s\t%s\n", route.Environment, route.URL)
		}
	}
	if len(w.Objects) != 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nS3 Bucket Objects\n"))
		writer.Flush()
		for _, object := range w.Objects {
			fmt.Fprintf(writer, "\n  %s\t%s\n", "Environment", object.Environment)
			fmt.Fprintf(writer, object.Tree)
		}
		writer.Flush()
	}
	if len(w.Resources) != 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nResources\n"))
		writer.Flush()

		w.Resources.humanStringByEnv(writer, w.environments)
	}
	writer.Flush()
	return b.String()
}
