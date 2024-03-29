// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/override"
	"github.com/robfig/cron/v3"
)

// Parameter logical IDs for a scheduled job
const (
	ScheduledJobScheduleParamKey = "Schedule"
)

// ScheduledJob represents the configuration needed to create a Cloudformation stack from a
// scheduled job manfiest.
type ScheduledJob struct {
	*ecsWkld
	manifest *manifest.ScheduledJob

	parser scheduledJobReadParser
}

var (
	fmtRateScheduleExpression = "rate(%d %s)" // rate({duration} {units})
	fmtCronScheduleExpression = "cron(%s)"

	awsScheduleRegexp = regexp.MustCompile(`((?:rate|cron)\(.*\)|none)`) // Validates that an expression is of the form rate(xyz) or cron(abc) or value 'none'
)

const (
	// Cron expressions in AWS Cloudwatch are of the form "M H DoM Mo DoW Y"
	// We use these predefined schedules when a customer specifies "@daily" or "@annually"
	// to fulfill the predefined schedules spec defined at
	// https://godoc.org/github.com/robfig/cron#hdr-Predefined_schedules
	// AWS requires that cron expressions use a ? wildcard for either DoM or DoW
	// so we represent that here.
	//            M H mD Mo wD Y
	cronHourly  = "0 * * * ? *" // at minute 0
	cronDaily   = "0 0 * * ? *" // at midnight
	cronWeekly  = "0 0 ? * 1 *" // at midnight on sunday
	cronMonthly = "0 0 1 * ? *" // at midnight on the first of the month
	cronYearly  = "0 0 1 1 ? *" // at midnight on January 1
)

const (
	hourly   = "@hourly"
	daily    = "@daily"
	midnight = "@midnight"
	weekly   = "@weekly"
	monthly  = "@monthly"
	yearly   = "@yearly"
	annually = "@annually"

	every = "@every "
)

type errScheduleInvalid struct {
	reason error
}

func (e errScheduleInvalid) Error() string {
	return fmt.Sprintf("schedule is not valid cron, rate, or preset: %v", e.reason)
}

type errDurationInvalid struct {
	reason error
}

func (e errDurationInvalid) Error() string {
	return fmt.Sprintf("parse duration: %v", e.reason)
}

// ScheduledJobConfig contains data required to initialize a scheduled job stack.
type ScheduledJobConfig struct {
	App                *config.Application
	Env                string
	Manifest           *manifest.ScheduledJob
	ArtifactBucketName string
	ArtifactKey        string
	RawManifest        string
	RuntimeConfig      RuntimeConfig
	Addons             NestedStackConfigurer
}

// NewScheduledJob creates a new ScheduledJob stack from a manifest file.
func NewScheduledJob(cfg ScheduledJobConfig) (*ScheduledJob, error) {
	crs, err := customresource.ScheduledJob(fs)
	if err != nil {
		return nil, fmt.Errorf("scheduled job custom resources: %w", err)
	}
	cfg.RuntimeConfig.loadCustomResourceURLs(cfg.ArtifactBucketName, uploadableCRs(crs).convert())

	return &ScheduledJob{
		ecsWkld: &ecsWkld{
			wkld: &wkld{
				name:               aws.StringValue(cfg.Manifest.Name),
				env:                cfg.Env,
				app:                cfg.App.Name,
				permBound:          cfg.App.PermissionsBoundary,
				artifactBucketName: cfg.ArtifactBucketName,
				artifactKey:        cfg.ArtifactKey,
				rc:                 cfg.RuntimeConfig,
				image:              cfg.Manifest.ImageConfig.Image,
				rawManifest:        cfg.RawManifest,
				parser:             fs,
				addons:             cfg.Addons,
			},
			sidecars:            cfg.Manifest.Sidecars,
			logging:             cfg.Manifest.Logging,
			tc:                  cfg.Manifest.TaskConfig,
			taskDefOverrideFunc: override.CloudFormationTemplate,
		},
		manifest: cfg.Manifest,

		parser: fs,
	}, nil
}

// Template returns the CloudFormation template for the scheduled job.
func (j *ScheduledJob) Template() (string, error) {
	addonsParams, err := j.addonsParameters()
	if err != nil {
		return "", err
	}
	addonsOutputs, err := j.addonsOutputs()
	if err != nil {
		return "", err
	}
	exposedPorts, err := j.manifest.ExposedPorts()
	if err != nil {
		return "", fmt.Errorf("parse exposed ports in service manifest %s: %w", j.name, err)
	}
	sidecars, err := convertSidecars(j.manifest.Sidecars, exposedPorts.PortsForContainer, j.rc)
	if err != nil {
		return "", fmt.Errorf("convert the sidecar configuration for job %s: %w", j.name, err)
	}
	publishers, err := convertPublish(j.manifest.Publish(), j.rc.AccountID, j.rc.Region, j.app, j.env, j.name)
	if err != nil {
		return "", fmt.Errorf(`convert "publish" field for job %s: %w`, j.name, err)
	}
	schedule, err := j.awsSchedule()
	if err != nil {
		return "", fmt.Errorf("convert schedule for job %s: %w", j.name, err)
	}
	stateMachine, err := j.stateMachineOpts()
	if err != nil {
		return "", fmt.Errorf("convert retry/timeout config for job %s: %w", j.name, err)
	}
	crs, err := convertCustomResources(j.rc.CustomResourcesURL)
	if err != nil {
		return "", err
	}
	entrypoint, err := convertEntryPoint(j.manifest.EntryPoint)
	if err != nil {
		return "", err
	}
	command, err := convertCommand(j.manifest.Command)
	if err != nil {
		return "", err
	}

	content, err := j.parser.ParseScheduledJob(template.WorkloadOpts{
		SerializedManifest:       string(j.rawManifest),
		Variables:                convertEnvVars(j.manifest.Variables),
		Secrets:                  convertSecrets(j.manifest.Secrets),
		WorkloadType:             manifestinfo.ScheduledJobType,
		NestedStack:              addonsOutputs,
		AddonsExtraParams:        addonsParams,
		Sidecars:                 sidecars,
		ScheduleExpression:       schedule,
		StateMachine:             stateMachine,
		HealthCheck:              convertContainerHealthCheck(j.manifest.ImageConfig.HealthCheck),
		LogConfig:                convertLogging(j.manifest.Logging),
		DockerLabels:             j.manifest.ImageConfig.Image.DockerLabels,
		Storage:                  convertStorageOpts(j.manifest.Name, j.manifest.Storage),
		Network:                  convertNetworkConfig(j.manifest.Network),
		EntryPoint:               entrypoint,
		Command:                  command,
		DependsOn:                convertDependsOn(j.manifest.ImageConfig.Image.DependsOn),
		CredentialsParameter:     aws.StringValue(j.manifest.ImageConfig.Image.Credentials),
		ServiceDiscoveryEndpoint: j.rc.ServiceDiscoveryEndpoint,
		Publish:                  publishers,
		Platform:                 convertPlatform(j.manifest.Platform),
		EnvVersion:               j.rc.EnvVersion,
		Version:                  j.rc.Version,

		CustomResources:     crs,
		PermissionsBoundary: j.permBound,
	})
	if err != nil {
		return "", fmt.Errorf("parse scheduled job template: %w", err)
	}
	overriddenTpl, err := j.taskDefOverrideFunc(convertTaskDefOverrideRules(j.manifest.TaskDefOverrides), content.Bytes())
	if err != nil {
		return "", fmt.Errorf("apply task definition overrides: %w", err)
	}
	return string(overriddenTpl), nil
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (j *ScheduledJob) Parameters() ([]*cloudformation.Parameter, error) {
	wkldParams, err := j.ecsWkld.Parameters()
	if err != nil {
		return nil, err
	}
	schedule, err := j.awsSchedule()
	if err != nil {
		return nil, err
	}
	return append(wkldParams, []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(ScheduledJobScheduleParamKey),
			ParameterValue: aws.String(schedule),
		},
	}...), nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized to a JSON document.
func (j *ScheduledJob) SerializedParameters() (string, error) {
	return serializeTemplateConfig(j.wkld.parser, j)
}

// awsSchedule converts the Schedule string to the format required by Cloudwatch Events
// https://docs.aws.amazon.com/lambda/latest/dg/services-cloudwatchevents-expressions.html
// Cron expressions must have an sixth "year" field, and must contain at least one ? (either-or)
// in either day-of-month or day-of-week.
// Day-of-week expressions are zero-indexed in Golang but one-indexed in AWS.
// @every cron definition strings are converted to rates.
// All others become cron expressions.
// Exception is made for strings of the form "rate( )" or "cron( )". These are accepted as-is and
// validated server-side by CloudFormation.
func (j *ScheduledJob) awsSchedule() (string, error) {
	schedule := aws.StringValue(j.manifest.On.Schedule)
	if schedule == "" {
		return "", fmt.Errorf(`missing required field "schedule" in manifest for job %s`, j.name)
	}
	// If the schedule uses default CloudWatch Events syntax, pass it through for server-side validation.
	if match := awsScheduleRegexp.FindStringSubmatch(schedule); match != nil {
		return aws.StringValue(j.manifest.On.Schedule), nil
	}
	// Try parsing the string as a cron expression to validate it.
	if _, err := cron.ParseStandard(schedule); err != nil {
		return "", errScheduleInvalid{reason: err}
	}
	var scheduleExpression string
	var err error
	switch {
	case strings.HasPrefix(schedule, every):
		scheduleExpression, err = toRate(schedule[len(every):])
		if err != nil {
			return "", fmt.Errorf("parse fixed interval: %w", err)
		}
	case strings.HasPrefix(schedule, "@"):
		scheduleExpression, err = toFixedSchedule(schedule)
		if err != nil {
			return "", fmt.Errorf("parse preset schedule: %w", err)
		}
	case schedule == "none":
		scheduleExpression = schedule // Keep expression as "none" when the job is disabled.
	default:
		scheduleExpression, err = toAWSCron(schedule)
		if err != nil {
			return "", fmt.Errorf("parse cron schedule: %w", err)
		}
	}
	return scheduleExpression, nil
}

// toRate converts a cron "@every" directive to a rate expression defined in minutes.
// example input: @every 1h30m
//
//	output: rate(90 minutes)
func toRate(duration string) (string, error) {
	d, err := time.ParseDuration(duration)
	if err != nil {
		return "", errDurationInvalid{reason: err}
	}
	// Check that rates are not specified in units smaller than minutes
	if d != d.Truncate(time.Minute) {
		return "", fmt.Errorf("duration must be a whole number of minutes or hours")
	}

	if d < time.Minute*1 {
		return "", errors.New("duration must be greater than or equal to 1 minute")
	}

	minutes := int(d.Minutes())
	if minutes == 1 {
		return fmt.Sprintf(fmtRateScheduleExpression, minutes, "minute"), nil
	}
	return fmt.Sprintf(fmtRateScheduleExpression, minutes, "minutes"), nil
}

// toFixedSchedule converts cron predefined schedules into AWS-flavored cron expressions.
// (https://godoc.org/github.com/robfig/cron#hdr-Predefined_schedules)
// Example input: @daily
//
//	output: cron(0 0 * * ? *)
//	 input: @annually
//	output: cron(0 0 1 1 ? *)
func toFixedSchedule(schedule string) (string, error) {
	switch {
	case strings.HasPrefix(schedule, hourly):
		return fmt.Sprintf(fmtCronScheduleExpression, cronHourly), nil
	case strings.HasPrefix(schedule, midnight):
		fallthrough
	case strings.HasPrefix(schedule, daily):
		return fmt.Sprintf(fmtCronScheduleExpression, cronDaily), nil
	case strings.HasPrefix(schedule, weekly):
		return fmt.Sprintf(fmtCronScheduleExpression, cronWeekly), nil
	case strings.HasPrefix(schedule, monthly):
		return fmt.Sprintf(fmtCronScheduleExpression, cronMonthly), nil
	case strings.HasPrefix(schedule, annually):
		fallthrough
	case strings.HasPrefix(schedule, yearly):
		return fmt.Sprintf(fmtCronScheduleExpression, cronYearly), nil
	default:
		return "", fmt.Errorf("unrecognized preset schedule %s", schedule)
	}
}

func awsCronFieldSpecified(input string) bool {
	return !strings.ContainsAny(input, "*?")
}

// toAWSCron converts "standard" 5-element crons into the AWS preferred syntax
// cron(* * * * ? *)
// MIN HOU DOM MON DOW YEA
// EITHER DOM or DOW must be specified as ? (either-or operator)
// BOTH DOM and DOW cannot be specified
// DOW numbers run 1-7, not 0-6
// Example input: 0 9 * * 1-5 (at 9 am, Monday-Friday)
//
//	: cron(0 9 ? * 2-6 *) (adds required ? operator, increments DOW to 1-index, adds year)
func toAWSCron(schedule string) (string, error) {
	const (
		MIN = iota
		HOU
		DOM
		MON
		DOW
	)

	// Split the cron into its components. We can do this because it'll already have been validated.
	// Use https://golang.org/pkg/strings/#Fields since it handles consecutive whitespace.
	sched := strings.Fields(schedule)

	// Check whether the Day of Week and Day of Month fields have a ?
	// Possible conversion:
	// * * * * * ==> * * * * ?
	// 0 9 * * 1 ==> 0 9 ? * 1
	// 0 9 1 * * ==> 0 9 1 * ?
	switch {
	// If both are unspecified, convert DOW to a ? and DOM to *
	case !awsCronFieldSpecified(sched[DOM]) && !awsCronFieldSpecified(sched[DOW]):
		sched[DOW] = "?"
		sched[DOM] = "*"
	// If DOM is * or ? and DOW is specified, convert DOM to ?
	case !awsCronFieldSpecified(sched[DOM]) && awsCronFieldSpecified(sched[DOW]):
		sched[DOM] = "?"
	// If DOW is * or ? and DOM is specified, convert DOW to ?
	case !awsCronFieldSpecified(sched[DOW]) && awsCronFieldSpecified(sched[DOM]):
		sched[DOW] = "?"
	// Error if both DOM and DOW are specified
	default:
		return "", errors.New("cannot specify both DOW and DOM in cron expression")
	}

	// Increment the DOW by one if specified as a number
	// https://play.golang.org/p/_1uxt0zJneb
	var newDOW []rune
	for _, c := range sched[DOW] {
		if unicode.IsDigit(c) {
			newDOW = append(newDOW, c+1)
		} else {
			newDOW = append(newDOW, c)
		}
	}
	// We don't need to use a string builder here because this will only ever have a max of
	// about 50 characters (SUN-MON,MON-TUE,TUE-WED,... is the longest possible string here)
	sched[DOW] = string(newDOW)

	// Add "every year" to 5-element crons to comply with AWS
	sched = append(sched, "*")

	return fmt.Sprintf(fmtCronScheduleExpression, strings.Join(sched, " ")), nil
}

// StateMachine converts the Timeout and Retries fields to an instance of template.StateMachineOpts
// It also performs basic validations to provide a fast feedback loop to the customer.
func (j *ScheduledJob) stateMachineOpts() (*template.StateMachineOpts, error) {
	var timeoutSeconds *int
	if inTimeout := aws.StringValue(j.manifest.Timeout); inTimeout != "" {
		parsedTimeout, err := time.ParseDuration(inTimeout)
		if err != nil {
			return nil, errDurationInvalid{reason: err}
		}
		if parsedTimeout < 1*time.Second {
			return nil, errors.New("timeout must be greater than or equal to 1 second")
		}
		if parsedTimeout != parsedTimeout.Truncate(time.Second) {
			return nil, errors.New("timeout must be a whole number of seconds, minutes, or hours")
		}
		timeoutSeconds = aws.Int(int(parsedTimeout.Seconds()))
	}

	var retries *int
	if inRetries := aws.IntValue(j.manifest.Retries); inRetries != 0 {
		if inRetries < 0 {
			return nil, errors.New("number of retries cannot be negative")
		}
		retries = aws.Int(inRetries)
	}
	return &template.StateMachineOpts{
		Timeout: timeoutSeconds,
		Retries: retries,
	}, nil
}
