// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package customresource provides functionality to upload Copilot custom resources.
package customresource

import (
	"fmt"
	"path"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

// Directory under which all custom resources are minified and packaged.
const customResourcesDir = "custom-resources"

// All custom resources will be copied under this path.
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
func RDWS(templateFS template.Reader) ([]CustomResource, error) {
	var crs []CustomResource
	pathForFn := map[string]string{
		envControllerFnName: envControllerFilePath,
		customDomainFnName:  customDomainAppRunnerFilePath,
	}
	for fn, path := range pathForFn {
		content, err := templateFS.Read(path)
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
