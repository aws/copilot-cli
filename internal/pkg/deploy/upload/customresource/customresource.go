// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package customresource provides functionality to upload Copilot custom resources.
package customresource

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/template/artifactpath"

	"github.com/aws/copilot-cli/internal/pkg/template"
)

// Directory under which all custom resources are minified and packaged.
const customResourcesDir = "custom-resources"

// All custom resource scripts will be copied under this path in the zip file.
const handlerFileName = "index.js"

// Function names.
const (
	envControllerFnName       = "EnvControllerFunction"
	dynamicDesiredCountFnName = "DynamicDesiredCountFunction"
	backlogPerTaskFnName      = "BacklogPerTaskCalculatorFunction"
	rulePriorityFnName        = "RulePriorityFunction"
	nlbCustomDomainFnName     = "NLBCustomDomainFunction"
	nlbCertValidatorFnName    = "NLBCertValidatorFunction"
	customDomainFnName        = "CustomDomainFunction"
	certValidationFnName      = "CertificateValidationFunction"
	dnsDelegationFnName       = "DNSDelegationFunction"
	bucketCleanerFnName       = "BucketCleanerFunction"
	certReplicatorFnName      = "CertificateReplicatorFunction"
	uniqueJsonValuesFnName    = "UniqueJSONValuesFunction"
	triggerStateMachineFnName = "TriggerStateMachineFunction"
)

// Function source file locations.
var (
	albRulePriorityGeneratorFilePath = path.Join(customResourcesDir, "alb-rule-priority-generator.js")
	backlogPerTaskCalculatorFilePath = path.Join(customResourcesDir, "backlog-per-task-calculator.js")
	customDomainFilePath             = path.Join(customResourcesDir, "custom-domain.js")
	customDomainAppRunnerFilePath    = path.Join(customResourcesDir, "custom-domain-app-runner.js")
	desiredCountDelegationFilePath   = path.Join(customResourcesDir, "desired-count-delegation.js")
	dnsCertValidationFilePath        = path.Join(customResourcesDir, "dns-cert-validator.js")
	certReplicatorFilePath           = path.Join(customResourcesDir, "cert-replicator.js")
	bucketCleanerFilePath            = path.Join(customResourcesDir, "bucket-cleaner.js")
	dnsDelegationFilePath            = path.Join(customResourcesDir, "dns-delegation.js")
	envControllerFilePath            = path.Join(customResourcesDir, "env-controller.js")
	wkldCertValidatorFilePath        = path.Join(customResourcesDir, "wkld-cert-validator.js")
	wkldCustomDomainFilePath         = path.Join(customResourcesDir, "wkld-custom-domain.js")
	uniqueJSONValuesFilePath         = path.Join(customResourcesDir, "unique-json-values.js")
	triggerStateMachineFilePath      = path.Join(customResourcesDir, "trigger-state-machine.js")
)

// CustomResource represents a CloudFormation custom resource backed by a Lambda function.
type CustomResource struct {
	name  string
	files []file

	// Post-initialization cached fields.
	zip *bytes.Buffer
}

// Name returns the name of the custom resource.
func (cr *CustomResource) Name() string {
	return cr.name
}

// ArtifactPath returns the S3 object key where the custom resource should be stored.
func (cr *CustomResource) ArtifactPath() string {
	return artifactpath.CustomResource(strings.ToLower(cr.Name()), cr.zip.Bytes())
}

// zipReader returns a reader for the zip archive from all the files in the custom resource.
func (cr *CustomResource) zipReader() io.Reader {
	return bytes.NewBuffer(cr.zip.Bytes())
}

func (cr *CustomResource) init() error {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	for _, file := range cr.files {
		f, err := w.Create(file.name)
		if err != nil {
			return fmt.Errorf("create zip file %q for custom resource %q: %v", file.name, cr.Name(), err)
		}
		_, err = f.Write(file.content)
		if err != nil {
			return fmt.Errorf("write zip file %q for custom resource %q: %v", file.name, cr.Name(), err)
		}
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close zip file for custom resource %q: %v", cr.Name(), err)
	}
	cr.zip = buf
	return nil
}

type file struct {
	name    string
	content []byte
}

// RDWS returns the custom resources for a request-driven web service.
func RDWS(fs template.Reader) ([]*CustomResource, error) {
	return buildCustomResources(fs, map[string]string{
		envControllerFnName: envControllerFilePath,
		customDomainFnName:  customDomainAppRunnerFilePath,
	})
}

// LBWS returns the custom resources for a load-balanced web service.
func LBWS(fs template.Reader) ([]*CustomResource, error) {
	return buildCustomResources(fs, map[string]string{
		dynamicDesiredCountFnName: desiredCountDelegationFilePath,
		envControllerFnName:       envControllerFilePath,
		rulePriorityFnName:        albRulePriorityGeneratorFilePath,
		nlbCustomDomainFnName:     wkldCustomDomainFilePath,
		nlbCertValidatorFnName:    wkldCertValidatorFilePath,
	})
}

// Worker returns the custom resources for a worker service.
func Worker(fs template.Reader) ([]*CustomResource, error) {
	return buildCustomResources(fs, map[string]string{
		dynamicDesiredCountFnName: desiredCountDelegationFilePath,
		backlogPerTaskFnName:      backlogPerTaskCalculatorFilePath,
		envControllerFnName:       envControllerFilePath,
	})
}

// Backend returns the custom resources for a backend service.
func Backend(fs template.Reader) ([]*CustomResource, error) {
	return buildCustomResources(fs, map[string]string{
		dynamicDesiredCountFnName: desiredCountDelegationFilePath,
		rulePriorityFnName:        albRulePriorityGeneratorFilePath,
		envControllerFnName:       envControllerFilePath,
	})
}

// StaticSite returns the custom resources for a static site service.
func StaticSite(fs template.Reader) ([]*CustomResource, error) {
	return buildCustomResources(fs, map[string]string{
		triggerStateMachineFnName: triggerStateMachineFilePath,
		certValidationFnName:      wkldCertValidatorFilePath,
		customDomainFnName:        wkldCustomDomainFilePath,
	})
}

// ScheduledJob returns the custom resources for a scheduled job.
func ScheduledJob(fs template.Reader) ([]*CustomResource, error) {
	return buildCustomResources(fs, map[string]string{
		envControllerFnName: envControllerFilePath,
	})
}

// Env returns the custom resources for an environment.
func Env(fs template.Reader) ([]*CustomResource, error) {
	return buildCustomResources(fs, map[string]string{
		certValidationFnName:   dnsCertValidationFilePath,
		customDomainFnName:     customDomainFilePath,
		dnsDelegationFnName:    dnsDelegationFilePath,
		certReplicatorFnName:   certReplicatorFilePath,
		bucketCleanerFnName:    bucketCleanerFilePath,
		uniqueJsonValuesFnName: uniqueJSONValuesFilePath,
	})
}

// UploadFunc is the function signature to upload contents under a key within a S3 bucket.
type UploadFunc func(key string, contents io.Reader) (url string, err error)

// Upload zips all the Files for each CustomResource and uploads the zip files individually to S3.
// Returns a map of the CustomResource FunctionName to the S3 URL where the zip file is stored.
func Upload(upload UploadFunc, crs []*CustomResource) (map[string]string, error) {
	urls := make(map[string]string)
	for _, cr := range crs {
		url, err := upload(cr.ArtifactPath(), cr.zipReader())
		if err != nil {
			return nil, fmt.Errorf("upload custom resource %q: %w", cr.Name(), err)
		}
		urls[cr.Name()] = url
	}
	return urls, nil
}

func buildCustomResources(fs template.Reader, pathForFn map[string]string) ([]*CustomResource, error) {
	var idx int
	crs := make([]*CustomResource, len(pathForFn))
	for fn, path := range pathForFn {
		content, err := fs.Read(path)
		if err != nil {
			return nil, fmt.Errorf("read custom resource %s at path %s: %v", fn, path, err)
		}
		crs[idx] = &CustomResource{
			name: fn,
			files: []file{
				{
					name:    handlerFileName,
					content: content.Bytes(),
				},
			},
		}
		if err := crs[idx].init(); err != nil {
			return nil, err
		}
		idx += 1
	}
	return crs, nil
}
