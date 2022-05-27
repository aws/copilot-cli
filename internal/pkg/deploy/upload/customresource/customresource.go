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

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
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
)

// Function source file locations.
var (
	albRulePriorityGeneratorFilePath = path.Join(customResourcesDir, "alb-rule-priority-generator.js")
	backlogPerTaskCalculatorFilePath = path.Join(customResourcesDir, "backlog-per-task-calculator.js")
	customDomainFilePath             = path.Join(customResourcesDir, "custom-domain.js")
	customDomainAppRunnerFilePath    = path.Join(customResourcesDir, "custom-domain-app-runner.js")
	desiredCountDelegationFilePath   = path.Join(customResourcesDir, "desired-count-delegation.js")
	dnsCertValidationFilePath        = path.Join(customResourcesDir, "dns-cert-validator.js")
	dnsDelegationFilePath            = path.Join(customResourcesDir, "dns-delegation.js")
	envControllerFilePath            = path.Join(customResourcesDir, "env-controller.js")
	nlbCertValidatorFilePath         = path.Join(customResourcesDir, "nlb-cert-validator.js")
	nlbCustomDomainFilePath          = path.Join(customResourcesDir, "nlb-custom-domain.js")
)

// CustomResource represents a CloudFormation custom resource backed by a Lambda function.
type CustomResource struct {
	name  string
	files []file
}

// FunctionName is the name of the Lambda function.
func (cr CustomResource) FunctionName() string {
	return cr.name
}

// Files returns the collection of files that need to be zipped together to form the deployment package for the function.
func (cr CustomResource) Files() []s3.NamedBinary {
	files := make([]s3.NamedBinary, len(cr.files))
	for i := range cr.files {
		files[i] = &cr.files[i]
	}
	return files
}

func (cr CustomResource) zip() (io.Reader, error) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	for _, file := range cr.files {
		f, err := w.Create(file.Name())
		if err != nil {
			return nil, fmt.Errorf("create zip file %q for custom resource %q: %v", file.Name(), cr.FunctionName(), err)
		}
		_, err = f.Write(file.Content())
		if err != nil {
			return nil, fmt.Errorf("write zip file %q for custom resource %q: %v", file.Name(), cr.FunctionName(), err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close zip file for custom resource %q: %v", cr.FunctionName(), err)
	}
	return buf, nil
}

// file implements the s3.NamedBinary interface.
type file struct {
	name    string
	content []byte
}

// Name returns the name of the file. The name can be a relative path.
func (f *file) Name() string {
	return f.name
}

// Content is the data in the file.
func (f *file) Content() []byte {
	return f.content
}

// RDWS returns the custom resources for a request-driven web service.
func RDWS(fs template.Reader) ([]CustomResource, error) {
	return buildCustomResources(fs, map[string]string{
		envControllerFnName: envControllerFilePath,
		customDomainFnName:  customDomainAppRunnerFilePath,
	})
}

// LBWS returns the custom resources for a load-balanced web service.
func LBWS(fs template.Reader) ([]CustomResource, error) {
	return buildCustomResources(fs, map[string]string{
		dynamicDesiredCountFnName: desiredCountDelegationFilePath,
		envControllerFnName:       envControllerFilePath,
		rulePriorityFnName:        albRulePriorityGeneratorFilePath,
		nlbCustomDomainFnName:     nlbCustomDomainFilePath,
		nlbCertValidatorFnName:    nlbCertValidatorFilePath,
	})
}

// Worker returns the custom resources for a worker service.
func Worker(fs template.Reader) ([]CustomResource, error) {
	return buildCustomResources(fs, map[string]string{
		dynamicDesiredCountFnName: desiredCountDelegationFilePath,
		backlogPerTaskFnName:      backlogPerTaskCalculatorFilePath,
		envControllerFnName:       envControllerFilePath,
	})
}

// Backend returns the custom resources for a backend service.
func Backend(fs template.Reader) ([]CustomResource, error) {
	return buildCustomResources(fs, map[string]string{
		dynamicDesiredCountFnName: desiredCountDelegationFilePath,
		rulePriorityFnName:        albRulePriorityGeneratorFilePath,
		envControllerFnName:       envControllerFilePath,
	})
}

// ScheduledJob returns the custom resources for a scheduled job.
func ScheduledJob(fs template.Reader) ([]CustomResource, error) {
	return buildCustomResources(fs, map[string]string{
		envControllerFnName: envControllerFilePath,
	})
}

// Env returns the custom resources for an environment.
func Env(fs template.Reader) ([]CustomResource, error) {
	return buildCustomResources(fs, map[string]string{
		certValidationFnName: dnsCertValidationFilePath,
		customDomainFnName:   customDomainFilePath,
		dnsDelegationFnName:  dnsDelegationFilePath,
	})
}

// UploadFunc is the function signature to upload contents under a key within a S3 bucket.
type UploadFunc func(key string, contents io.Reader) (url string, err error)

// Upload zips all the Files for each CustomResource and uploads the zip files individually to S3.
// Returns a map of the CustomResource FunctionName to the S3 URL where the zip file is stored.
func Upload(upload UploadFunc, crs []CustomResource) (map[string]string, error) {
	urls := make(map[string]string)
	for _, cr := range crs {
		zipFile, err := cr.zip()
		if err != nil {
			return nil, err
		}
		out, err := io.ReadAll(zipFile)
		if err != nil {
			return nil, fmt.Errorf("read content of zip file for custom resource %q: %v", cr.FunctionName(), err)
		}
		url, err := upload(artifactpath.CustomResource(strings.ToLower(cr.FunctionName()), out), zipFile)
		if err != nil {
			return nil, fmt.Errorf("upload custom resource %q: %w", cr.FunctionName(), err)
		}
		urls[cr.FunctionName()] = url
	}
	return urls, nil
}

func buildCustomResources(fs template.Reader, pathForFn map[string]string) ([]CustomResource, error) {
	var crs []CustomResource
	for fn, path := range pathForFn {
		content, err := fs.Read(path)
		if err != nil {
			return nil, fmt.Errorf("read custom resource %s at path %s: %v", fn, path, err)
		}
		crs = append(crs, CustomResource{
			name: fn,
			files: []file{
				{
					name:    handlerFileName,
					content: content.Bytes(),
				},
			},
		})
	}
	return crs, nil
}
