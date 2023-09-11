// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/template/artifactpath"
)

// DeployService deploys a service stack and renders progress updates to out until the deployment is done.
// If the service stack doesn't exist, then it creates the stack.
// If the service stack already exists, it updates the stack.
func (cf CloudFormation) DeployService(conf StackConfiguration, bucketName string, detach bool, opts ...cloudformation.StackOption) error {
	templateURL, err := cf.uploadStackTemplateToS3(bucketName, conf)
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
	return cf.executeAndRenderChangeSet(cf.newUpsertChangeSetInput(cf.console, stack, withEnableInterrupt(), withDetach(detach)))
}

type uploadableStack interface {
	StackName() string
	Template() (string, error)
}

func (cf CloudFormation) uploadStackTemplateToS3(bucket string, stack uploadableStack) (string, error) {
	tmpl, err := stack.Template()
	if err != nil {
		return "", fmt.Errorf("generate template: %w", err)
	}
	url, err := cf.s3Client.Upload(bucket, artifactpath.CFNTemplate(stack.StackName(), []byte(tmpl)), strings.NewReader(tmpl))
	if err != nil {
		return "", err
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
	stackName := fmt.Sprintf("%s-%s-%s", in.AppName, in.EnvName, in.Name)
	description := fmt.Sprintf("Delete stack %s", stackName)
	return cf.deleteAndRenderStack(deleteAndRenderInput{
		stackName:   stackName,
		description: description,
		deleteFn: func() error {
			return cf.cfnClient.DeleteAndWaitWithRoleARN(stackName, in.ExecutionRoleARN)
		},
	})
}
