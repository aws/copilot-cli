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
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

// Template rendering configuration common across workloads.
const (
	wkldParamsTemplatePath = "workloads/params.json.tmpl"

	svcWlType = "svc"
	jobWlType = "job"
)

// Parameter logical IDs common across workloads.
const (
	WorkloadAppNameParamKey           = "AppName"
	WorkloadEnvNameParamKey           = "EnvName"
	WorkloadNameParamKey              = "WorkloadName"
	WorkloadContainerImageParamKey    = "ContainerImage"
	WorkloadTaskCPUParamKey           = "TaskCPU"
	WorkloadTaskMemoryParamKey        = "TaskMemory"
	WorkloadTaskCountParamKey         = "TaskCount"
	WorkloadLogRetentionParamKey      = "LogRetention"
	WorkloadAddonsTemplateURLParamKey = "AddonsTemplateURL"
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
	Image             *ECRImage         // Optional. Image location in an ECR repository.
	AddonsTemplateURL string            // Optional. S3 object URL for the addons template.
	AdditionalTags    map[string]string // AdditionalTags are labels applied to resources in the workload stack.
}

// ECRImage represents configuration about the pushed ECR image that is needed to
// create a CloudFormation stack.
type ECRImage struct {
	RepoURL  string // RepoURL is the ECR repository URL the container image should be pushed to.
	ImageTag string // Tag is the container image's unique tag.
}

// GetLocation returns the location of the ECR image.
func (i ECRImage) GetLocation() string {
	return fmt.Sprintf("%s:%s", i.RepoURL, i.ImageTag)
}

type templater interface {
	Template() (string, error)
}

type location interface {
	GetLocation() string
}

// wkld represents a containerized workload running on Amazon ECS.
// A workload can be a long-running service, an ephemeral task, or a periodic task.
type wkld struct {
	name  string
	env   string
	app   string
	tc    manifest.TaskConfig
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
	desiredCount := w.tc.Count.Value
	// If auto scaling is configured, override the desired count value.
	if !w.tc.Count.Autoscaling.IsEmpty() {
		min, _, err := w.tc.Count.Autoscaling.Range.Parse()
		if err != nil {
			return nil, fmt.Errorf("parse task count value %s: %w", aws.StringValue((*string)(w.tc.Count.Autoscaling.Range)), err)
		}
		desiredCount = aws.Int(min)
	}
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
		var noAddonsErr *addon.ErrAddonsDirNotExist
		if !errors.As(err, &noAddonsErr) {
			return nil, fmt.Errorf("generate addons template for %s: %w", w.name, err)
		}
		return nil, nil // Addons directory does not exist, so there are no outputs and error.
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

// Validate that paths contain only an approved set of characters to guard against command injection.
// We can accept 0-9A-Za-z-_.
func validatePath(input string, maxLength int) error {
	if len(input) > maxLength {
		return fmt.Errorf("path must be less than %d bytes in length", maxLength)
	}
	if len(input) == 0 {
		return nil
	}
	m := pathRegexp.FindStringSubmatch(input)
	if len(m) == 0 {
		return fmt.Errorf("paths can only contain the characters a-zA-Z0-9.-_/")
	}
	return nil
}

func validateStorageConfig(in *manifest.Storage) error {
	if in == nil {
		return nil
	}
	return validateVolumes(in.Volumes)
}

func validateVolumes(in map[string]manifest.Volume) error {
	for name, v := range in {
		if err := validateVolume(name, v); err != nil {
			return err
		}
	}
	return nil
}

func validateVolume(name string, in manifest.Volume) error {
	if err := validateMountPointConfig(in); err != nil {
		return fmt.Errorf("validate container configuration for volume %s: %w", name, err)
	}
	if err := validateEFSConfig(in); err != nil {
		return fmt.Errorf("validate EFS configuration for volume %s: %w", name, err)
	}
	return nil
}

func validateMountPointConfig(in manifest.Volume) error {
	// containerPath must be specified.
	path := aws.StringValue(in.ContainerPath)
	if path == "" {
		return errNoContainerPath
	}
	if err := validateContainerPath(path); err != nil {
		return fmt.Errorf("validate container path %s: %w", path, err)
	}
	return nil
}

func validateSidecarMountPoints(in []manifest.SidecarMountPoint) error {
	if in == nil {
		return nil
	}
	for _, mp := range in {
		if aws.StringValue(mp.ContainerPath) == "" {
			return errNoContainerPath
		}
		if aws.StringValue(mp.SourceVolume) == "" {
			return errNoSourceVolume
		}
	}
	return nil
}

func validateEFSConfig(in manifest.Volume) error {
	// EFS is implicitly disabled.
	if in.EFS == nil {
		return nil
	}
	// This should never happen but error when EFS is Enabled with a non-empty configuration.
	if aws.BoolValue(in.EFS.Enabled) && !in.EFS.Config.IsEmpty() {
		return errInvalidEFSConfig
	}

	// If EFS is disabled explicitly, return nil.
	if in.EFS.Enabled != nil && !aws.BoolValue(in.EFS.Enabled) {
		return nil
	}

	// UID and GID are mutually exclusive with any other fields.
	if !in.EFS.Config.EmptyBYOConfig() && !in.EFS.Config.EmptyUIDConfig() {
		return errUIDWithNonManagedFS
	}

	// Check that required fields for BYO EFS are satisfied.
	if !in.EFS.Config.EmptyBYOConfig() && !in.EFS.Config.IsEmpty() {
		if aws.StringValue(in.EFS.Config.FileSystemID) == "" {
			return errNoFSID
		}
	}

	if err := validateRootDirPath(aws.StringValue(in.EFS.Config.RootDirectory)); err != nil {
		return err
	}

	if err := validateAuthConfig(in.EFS.Config); err != nil {
		return err
	}

	if err := validateUIDGID(in.EFS.Config.UID, in.EFS.Config.GID); err != nil {
		return err
	}

	return nil
}

func validateAuthConfig(in manifest.EFSVolumeConfiguration) error {
	if in.AuthConfig == nil {
		return nil
	}
	rd := aws.StringValue(in.RootDirectory)
	if !(rd == "" || rd == "/") && in.AuthConfig.AccessPointID != nil {
		return errAccessPointWithRootDirectory
	}

	if in.AuthConfig.AccessPointID != nil && !aws.BoolValue(in.AuthConfig.IAM) {
		return errAccessPointWithoutIAM
	}

	return nil
}

func validateUIDGID(uid, gid *uint32) error {
	if uid == nil && gid == nil {
		return nil
	}
	if uid != nil && gid == nil {
		return errInvalidUIDGIDConfig
	}
	if uid == nil && gid != nil {
		return errInvalidUIDGIDConfig
	}
	// Check for root UID.
	if aws.Uint32Value(uid) == 0 {
		return errReservedUID
	}
	return nil
}

func validateRootDirPath(input string) error {
	return validatePath(input, maxEFSPathLength)
}

func validateContainerPath(input string) error {
	return validatePath(input, maxDockerContainerPathLength)
}
