// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to create Manifest files.
package manifest

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/robfig/cron/v3"
)

const (
	// ScheduledJobType is a recurring ECS Fargate task which runs on a schedule.
	ScheduledJobType = "Scheduled Job"
)

const (
	scheduledJobManifestPath = "workloads/jobs/scheduled-job/manifest.yml"
)

// JobTypes holds the valid job "architectures"
var JobTypes = []string{
	ScheduledJobType,
}

var (
	errStringNotDuration            = errors.New("duration must be of the form 90m, 2h, 60s")
	errStringNotCron                = errors.New("string is not a valid cron schedule")
	errStringNeitherDurationNorCron = errors.New("schedule must be either cron expression or duration")
	errDurationTooShort             = errors.New("rate expressions must have duration longer than 1 second")
)

var (
	fmtRateExpression = "rate(%d minutes)"
	fmtCronExpression = "cron(%s)"
)

// ScheduledJob holds the configuration to build a container image that is run
// periodically in a given environment with timeout and retry logic.
type ScheduledJob struct {
	Workload           `yaml:",inline"`
	ScheduledJobConfig `yaml:",inline"`
	Environments       map[string]*ScheduledJobConfig `yaml:",flow"`

	parser template.Parser
}

// ScheduledJobConfig holds the configuration for a scheduled job
type ScheduledJobConfig struct {
	Image          Image `yaml:",flow"`
	TaskConfig     `yaml:",inline"`
	*Logging       `yaml:"logging,flow"`
	Sidecar        `yaml:",inline"`
	ScheduleConfig `yaml:",inline"`
}

// ScheduleConfig holds the fields necessary to describe a scheduled job's execution frequency and error handling.
type ScheduleConfig struct {
	Schedule string `yaml:"schedule"`
	Timeout  string `yaml:"timeout"`
	Retries  int    `yaml:"retries"`
}

var regexpPredefined = regexp.MustCompile(`@(hourly|daily|weekly|monthly|yearly|annually)`)
var regexpEvery = regexp.MustCompile(`@every (\d+.*)`)

// Schedule is a string which can be parsed into either a cron entry or a duration.
// AWS uses a 6-member cron of the format MIN HOUR DOM MON DOW YEAR, so we assume
// the year field is always *
type Schedule struct {
	rawString string
	parsed    bool
	Cron      string
	Rate      string
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the Schedule
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v2) interface.
func (s *Schedule) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&s.rawString); err != nil {
		return err
	}
	if err := s.parseCron(); err != nil {
		switch err {
		case errStringNotCron:
			break
		default:
			return err
		}
	}

	// If we could successfully parse out a value, return. Otherwise, try
	// parsing it as a rate.
	if s.parsed {
		return nil
	}

	if err := s.parseRate(); err != nil {
		return err
	}
	if s.parsed {
		return nil
	}

	return errStringNeitherDurationNorCron
}

// ScheduledJobProps contains properties for creating a new scheduled job manifest.
type ScheduledJobProps struct {
	*WorkloadProps
	Schedule string
	Timeout  string
	Retries  int
}

// LogConfigOpts converts the job's Firelens configuration into a format parsable by the templates pkg.
func (lc *ScheduledJobConfig) LogConfigOpts() *template.LogConfigOpts {
	if lc.Logging == nil {
		return nil
	}
	return lc.logConfigOpts()
}

// newDefaultScheduledJob returns an empty ScheduledJob with only the default values set.
func newDefaultScheduledJob() *ScheduledJob {
	return &ScheduledJob{
		Workload: Workload{
			Type: aws.String(ScheduledJobType),
		},
		ScheduledJobConfig: ScheduledJobConfig{
			Image: Image{},
			TaskConfig: TaskConfig{
				CPU:    aws.Int(256),
				Memory: aws.Int(512),
			},
		},
	}
}

func (s *Schedule) parseCron() error {
	_, err := cron.ParseStandard(s.rawString)
	if err != nil {
		return errStringNotCron
	}
	// check if the string is a directive or a schedule
	if strings.HasPrefix(s.rawString, "@") {
		every := "@every "
		if strings.HasPrefix(s.rawString, every) {
			d := Duration(s.rawString[len(every):])
			s.Rate, err = d.ToRate()
			if err != nil {
				return err
			}
			s.parsed = true
			return nil
		}

		predefinedMatch := regexpPredefined.FindStringSubmatch(s.rawString)
		if len(predefinedMatch) == 0 {
			return errStringNotCron
		}
		s.Cron = fmt.Sprintf(fmtCronExpression, predefinedSchedules[predefinedMatch[1]])
		s.parsed = true
		return nil
	}

	// the string was parseable by cron but did not use a predefined schedule or @every directive
	s.Cron = fmt.Sprintf(fmtCronExpression, s.rawString)
	s.parsed = true
	return nil
}

func (s *Schedule) parseRate() error {
	d := Duration(s.rawString)
	rate, err := d.ToRate()
	if err != nil {
		return err
	}
	s.Rate = rate
	s.parsed = true
	return nil
}

// Duration is a string of the form 90m, 30s, 24h.
type Duration string

// NewScheduledJob creates a new
func NewScheduledJob(props ScheduledJobProps) *ScheduledJob {
	job := newDefaultScheduledJob()
	// Apply overrides.
	job.Name = aws.String(props.Name)
	job.ScheduledJobConfig.Image.Build.BuildArgs.Dockerfile = aws.String(props.Dockerfile)
	job.Schedule = props.Schedule
	job.Retries = props.Retries
	job.Timeout = props.Timeout
	job.parser = template.New()
	return job
}

// MarshalBinary serializes the manifest object into a binary YAML document.
// Implements the encoding.BinaryMarshaler interface.
func (j *ScheduledJob) MarshalBinary() ([]byte, error) {
	content, err := j.parser.Parse(scheduledJobManifestPath, *j)
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// ToMinutes converts the duration string into an integer number of minutes.
func (d Duration) ToMinutes() (minutes int, err error) {
	stringDuration := string(d)
	duration, err := time.ParseDuration(stringDuration)
	if err != nil {
		return 0, err
	}

	minutes = int(duration.Minutes())

	return minutes, nil
}

// ToRate converts the duration string into a rate string valid for
// Cloudwatch Events schedule expressions
func (d Duration) ToRate() (rate string, err error) {
	minutes, err := d.ToMinutes()
	if err != nil {
		return "", err
	}
	if minutes < 1 {
		return "", errDurationTooShort
	}
	return fmt.Sprintf(fmtRateExpression, minutes), nil
}
