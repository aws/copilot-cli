// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

// Template rendering configuration common across workloads.
const (
	wkldParamsTemplatePath = "workloads/params.json.tmpl"
)

// Parameter logical IDs common across workloads.
const (
	WorkloadAppNameParamKey           = "AppName"
	WorkloadEnvNameParamKey           = "EnvName"
	WorkloadNameParamKey              = "WorkloadName"
	WorkloadContainerImageParamKey    = "ContainerImage"
	WorkloadContainerPortParamKey     = "ContainerPort"
	WorkloadAddonsTemplateURLParamKey = "AddonsTemplateURL"
)

// Parameter logical IDs for workloads on ECS.
const (
	WorkloadTaskCPUParamKey      = "TaskCPU"
	WorkloadTaskMemoryParamKey   = "TaskMemory"
	WorkloadTaskCountParamKey    = "TaskCount"
	WorkloadLogRetentionParamKey = "LogRetention"
)

// Parameter logical IDs for workloads on App Runner.
const (
	RDWkldImageRepositoryType                   = "ImageRepositoryType"
	RDWkldInstanceCPUParamKey                   = "InstanceCPU"
	RDWkldInstanceMemoryParamKey                = "InstanceMemory"
	RDWkldInstanceRoleParamKey                  = "InstanceRole"
	RDWkldHealthCheckPathParamKey               = "HealthCheckPath"
	RDWkldHealthCheckIntervalParamKey           = "HealthCheckInterval"
	RDWkldHealthCheckTimeoutParamKey            = "HealthCheckTimeout"
	RDWkldHealthCheckHealthyThresholdParamKey   = "HealthCheckHealthyThreshold"
	RDWkldHealthCheckUnhealthyThresholdParamKey = "HealthCheckUnhealthyThreshold"
)

// Matches alphanumeric characters and -._
var pathRegexp = regexp.MustCompile(`^[a-zA-Z0-9\-\.\_/]+$`)

// Max path length in EFS is 255 bytes.
// https://docs.aws.amazon.com/efs/latest/ug/troubleshooting-efs-fileop-errors.html#filenametoolong
const maxEFSPathLength = 255

// In docker containers, max path length is 242.
// https://github.com/moby/moby/issues/1413
const maxDockerContainerPathLength = 242

// RuntimeConfig represents configuration that's defined outside of the manifest file
// that is needed to create a CloudFormation stack.
type RuntimeConfig struct {
	Image                    *ECRImage         // Optional. Image location in an ECR repository.
	AddonsTemplateURL        string            // Optional. S3 object URL for the addons template.
	AdditionalTags           map[string]string // AdditionalTags are labels applied to resources in the workload stack.
	ServiceDiscoveryEndpoint string            // Endpoint for the service discovery namespace in the environment.
	AccountID                string            // Account ID for constructing ARNs
	Region                   string            // Region for constructing ARNs
}

// ECRImage represents configuration about the pushed ECR image that is needed to
// create a CloudFormation stack.
type ECRImage struct {
	RepoURL  string // RepoURL is the ECR repository URL the container image should be pushed to.
	ImageTag string // Tag is the container image's unique tag.
	Digest   string // The image digest.
}

// GetLocation returns the ECR image URI.
// If a tag is provided by the user or discovered from git then prioritize referring to the image via the tag.
// Otherwise, each image after a push to ECR will get a digest and we refer to the image via the digest.
// Finally, if no digest or tag is present, this occurs with the "package" commands, we default to the "latest" tag.
func (i ECRImage) GetLocation() string {
	if i.ImageTag != "" {
		return fmt.Sprintf("%s:%s", i.RepoURL, i.ImageTag)
	}
	if i.Digest != "" {
		return fmt.Sprintf("%s@%s", i.RepoURL, i.Digest)
	}
	return fmt.Sprintf("%s:%s", i.RepoURL, "latest")
}

type templater interface {
	Template() (string, error)
}

type location interface {
	GetLocation() string
}

// wkld represents a generic containerized workload.
// A workload can be a long-running service, an ephemeral task, or a periodic task.
type wkld struct {
	name  string
	env   string
	app   string
	rc    RuntimeConfig
	image location

	parser template.Parser
	addons templater
}

// StackName returns the name of the stack.
func (w *wkld) StackName() string {
	return NameForService(w.app, w.env, w.name)
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (w *wkld) Parameters() ([]*cloudformation.Parameter, error) {
	var img string
	if w.image != nil {
		img = w.image.GetLocation()
	}
	if w.rc.Image != nil {
		img = w.rc.Image.GetLocation()
	}
	return []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(WorkloadAppNameParamKey),
			ParameterValue: aws.String(w.app),
		},
		{
			ParameterKey:   aws.String(WorkloadEnvNameParamKey),
			ParameterValue: aws.String(w.env),
		},
		{
			ParameterKey:   aws.String(WorkloadNameParamKey),
			ParameterValue: aws.String(w.name),
		},
		{
			ParameterKey:   aws.String(WorkloadContainerImageParamKey),
			ParameterValue: aws.String(img),
		},
		{
			ParameterKey:   aws.String(WorkloadAddonsTemplateURLParamKey),
			ParameterValue: aws.String(w.rc.AddonsTemplateURL),
		},
	}, nil
}

// Tags returns the list of tags to apply to the CloudFormation stack.
func (w *wkld) Tags() []*cloudformation.Tag {
	return mergeAndFlattenTags(w.rc.AdditionalTags, map[string]string{
		deploy.AppTagKey:     w.app,
		deploy.EnvTagKey:     w.env,
		deploy.ServiceTagKey: w.name,
	})
}

type templateConfigurer interface {
	Parameters() ([]*cloudformation.Parameter, error)
	Tags() []*cloudformation.Tag
}

func (w *wkld) templateConfiguration(tc templateConfigurer) (string, error) {
	params, err := tc.Parameters()
	if err != nil {
		return "", err
	}
	doc, err := w.parser.Parse(wkldParamsTemplatePath, struct {
		Parameters []*cloudformation.Parameter
		Tags       []*cloudformation.Tag
	}{
		Parameters: params,
		Tags:       tc.Tags(),
	}, template.WithFuncs(map[string]interface{}{
		"inc": template.IncFunc,
	}))
	if err != nil {
		return "", err
	}
	return doc.String(), nil
}

func (w *wkld) addonsOutputs() (*template.WorkloadNestedStackOpts, error) {
	stack, err := w.addons.Template()
	if err != nil {
		var notFoundErr *addon.ErrAddonsNotFound
		if !errors.As(err, &notFoundErr) {
			return nil, fmt.Errorf("generate addons template for %s: %w", w.name, err)
		}
		return nil, nil // No addons found, so there are no outputs and error.
	}

	out, err := addon.Outputs(stack)
	if err != nil {
		return nil, fmt.Errorf("get addons outputs for %s: %w", w.name, err)
	}
	return &template.WorkloadNestedStackOpts{
		StackName:            addon.StackName,
		VariableOutputs:      envVarOutputNames(out),
		SecretOutputs:        secretOutputNames(out),
		PolicyOutputs:        managedPolicyOutputNames(out),
		SecurityGroupOutputs: securityGroupOutputNames(out),
	}, nil
}

func securityGroupOutputNames(outputs []addon.Output) []string {
	var securityGroups []string
	for _, out := range outputs {
		if out.IsSecurityGroup {
			securityGroups = append(securityGroups, out.Name)
		}
	}
	return securityGroups
}

func secretOutputNames(outputs []addon.Output) []string {
	var secrets []string
	for _, out := range outputs {
		if out.IsSecret {
			secrets = append(secrets, out.Name)
		}
	}
	return secrets
}

func managedPolicyOutputNames(outputs []addon.Output) []string {
	var policies []string
	for _, out := range outputs {
		if out.IsManagedPolicy {
			policies = append(policies, out.Name)
		}
	}
	return policies
}

func envVarOutputNames(outputs []addon.Output) []string {
	var envVars []string
	for _, out := range outputs {
		if !out.IsSecret && !out.IsManagedPolicy {
			envVars = append(envVars, out.Name)
		}
	}
	return envVars
}

type ecsWkld struct {
	*wkld
	tc manifest.TaskConfig
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (w *ecsWkld) Parameters() ([]*cloudformation.Parameter, error) {
	wkldParameters, err := w.wkld.Parameters()
	if err != nil {
		return nil, err
	}
	desiredCount, err := w.tc.Count.Desired()
	if err != nil {
		return nil, err
	}
	return append(wkldParameters, []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(WorkloadTaskCPUParamKey),
			ParameterValue: aws.String(strconv.Itoa(aws.IntValue(w.tc.CPU))),
		},
		{
			ParameterKey:   aws.String(WorkloadTaskMemoryParamKey),
			ParameterValue: aws.String(strconv.Itoa(aws.IntValue(w.tc.Memory))),
		},
		{
			ParameterKey:   aws.String(WorkloadTaskCountParamKey),
			ParameterValue: aws.String(strconv.Itoa(*desiredCount)),
		},
		{
			ParameterKey:   aws.String(WorkloadLogRetentionParamKey),
			ParameterValue: aws.String("30"),
		},
	}...), nil
}

type appRunnerWkld struct {
	*wkld
	instanceConfig    manifest.AppRunnerInstanceConfig
	imageConfig       manifest.ImageWithPort
	healthCheckConfig manifest.HealthCheckArgsOrString
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (w *appRunnerWkld) Parameters() ([]*cloudformation.Parameter, error) {
	wkldParameters, err := w.wkld.Parameters()
	if err != nil {
		return nil, err
	}
	var img string
	if w.image != nil {
		img = w.image.GetLocation()
	}
	if w.rc.Image != nil {
		img = w.rc.Image.GetLocation()
	}

	imageRepositoryType, err := apprunner.DetermineImageRepositoryType(img)
	if err != nil {
		return nil, fmt.Errorf("determining image repository type: %w", err)
	}

	if w.imageConfig.Port == nil {
		return nil, fmt.Errorf("field `image.port` is required for Request Driven Web Services")
	}

	if w.instanceConfig.CPU == nil {
		return nil, fmt.Errorf("field `cpu` is required for Request Driven Web Services")
	}

	if w.instanceConfig.Memory == nil {
		return nil, fmt.Errorf("field `memory` is required for Request Driven Web Services")
	}

	appRunnerParameters := []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(RDWkldImageRepositoryType),
			ParameterValue: aws.String(imageRepositoryType),
		},
		{
			ParameterKey:   aws.String(WorkloadContainerPortParamKey),
			ParameterValue: aws.String(strconv.Itoa(int(aws.Uint16Value(w.imageConfig.Port)))),
		},
		{
			ParameterKey:   aws.String(RDWkldInstanceCPUParamKey),
			ParameterValue: aws.String(strconv.Itoa(aws.IntValue(w.instanceConfig.CPU))),
		},
		{
			ParameterKey:   aws.String(RDWkldInstanceMemoryParamKey),
			ParameterValue: aws.String(strconv.Itoa(aws.IntValue(w.instanceConfig.Memory))),
		},
	}

	// Optional HealthCheckPath parameter
	if w.healthCheckConfig.Path() != nil {
		appRunnerParameters = append(appRunnerParameters, &cloudformation.Parameter{
			ParameterKey:   aws.String(RDWkldHealthCheckPathParamKey),
			ParameterValue: aws.String(*w.healthCheckConfig.Path()),
		})
	}

	// Optional HealthCheckInterval parameter
	if w.healthCheckConfig.HealthCheckArgs.Interval != nil {
		appRunnerParameters = append(appRunnerParameters, &cloudformation.Parameter{
			ParameterKey:   aws.String(RDWkldHealthCheckIntervalParamKey),
			ParameterValue: aws.String(strconv.Itoa(int(w.healthCheckConfig.HealthCheckArgs.Interval.Seconds()))),
		})
	}

	// Optional HealthCheckTimeout parameter
	if w.healthCheckConfig.HealthCheckArgs.Timeout != nil {
		appRunnerParameters = append(appRunnerParameters, &cloudformation.Parameter{
			ParameterKey:   aws.String(RDWkldHealthCheckTimeoutParamKey),
			ParameterValue: aws.String(strconv.Itoa(int(w.healthCheckConfig.HealthCheckArgs.Timeout.Seconds()))),
		})
	}

	// Optional HealthCheckHealthyThreshold parameter
	if w.healthCheckConfig.HealthCheckArgs.HealthyThreshold != nil {
		appRunnerParameters = append(appRunnerParameters, &cloudformation.Parameter{
			ParameterKey:   aws.String(RDWkldHealthCheckHealthyThresholdParamKey),
			ParameterValue: aws.String(strconv.Itoa(int(*w.healthCheckConfig.HealthCheckArgs.HealthyThreshold))),
		})
	}

	// Optional HealthCheckUnhealthyThreshold parameter
	if w.healthCheckConfig.HealthCheckArgs.UnhealthyThreshold != nil {
		appRunnerParameters = append(appRunnerParameters, &cloudformation.Parameter{
			ParameterKey:   aws.String(RDWkldHealthCheckUnhealthyThresholdParamKey),
			ParameterValue: aws.String(strconv.Itoa(int(*w.healthCheckConfig.HealthCheckArgs.UnhealthyThreshold))),
		})
	}

	return append(wkldParameters, appRunnerParameters...), nil
}
