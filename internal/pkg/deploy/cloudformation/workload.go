// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/term/progress"
)

const fmtWorkloadCFNTemplateName = "manual/templates/%s/%x.yml"

// DeployService deploys a service stack and renders progress updates to out until the deployment is done.
// If the service stack doesn't exist, then it creates the stack.
// If the service stack already exists, it updates the stack.
func (cf CloudFormation) DeployService(out progress.FileWriter, conf StackConfiguration, bucketName string, opts ...cloudformation.StackOption) error {
	templateURL, err := cf.pushWorkloadTemplateToS3Bucket(bucketName, conf)
	if err != nil {
		return err
	}
	stack, err := toStackFromS3(conf, templateURL)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(stack)
	}
	return cf.renderStackChanges(cf.newRenderWorkloadInput(out, stack))
}

func (cf CloudFormation) pushWorkloadTemplateToS3Bucket(bucket string, config StackConfiguration) (string, error) {
	template, err := config.Template()
	if err != nil {
		return "", fmt.Errorf("generate template: %w", err)
	}
	reader := strings.NewReader(template)
	url, err := cf.appS3Client.Upload(bucket, fmt.Sprintf(fmtWorkloadCFNTemplateName, config.StackName(), sha256.Sum256([]byte(template))), reader)
	if err != nil {
		return "", fmt.Errorf("upload workload template to S3 bucket %s: %w", bucket, err)
	}
	return url, nil
}

func (cf CloudFormation) handleStackError(stackName string, err error) error {
	if err == nil {
		return nil
	}
	reasons, describeErr := cf.errorEvents(stackName)
	if describeErr != nil {
		return fmt.Errorf("%w: describe stack: %v", err, describeErr)
	}
	if len(reasons) == 0 {
		return err
	}
	return fmt.Errorf("%w: %s", err, reasons[0])
}

// DeleteWorkload removes the CloudFormation stack of a deployed workload.
func (cf CloudFormation) DeleteWorkload(in deploy.DeleteWorkloadInput) error {
	return cf.cfnClient.DeleteAndWait(fmt.Sprintf("%s-%s-%s", in.AppName, in.EnvName, in.Name))
}
