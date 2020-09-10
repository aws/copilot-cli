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

	"github.com/robfig/cron/v3"
)

const (
	// ScheduledJobType is a recurring ECS Fargate task which runs on a schedule.
	ScheduledJobType = "Scheduled Job"
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

const (
	cronHourly  = "0 * * * * *" // at minute 0
	cronDaily   = "0 0 * * * *" // at midnight
	cronWeekly  = "0 0 * * 0 *" // at midnight on sunday
	cronMonthly = "0 0 1 * * *" // at midnight on the first of the month
	cronYearly  = "0 0 1 1 * *" // at midnight on January 1
)

var predefinedSchedules = map[string]string{
	"hourly":   cronHourly,
	"daily":    cronDaily,
	"weekly":   cronWeekly,
	"monthly":  cronMonthly,
	"yearly":   cronYearly,
	"annually": cronYearly,
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

// ToSeconds converts the duration string into an integer number of seconds
func (d Duration) ToSeconds() (seconds int, err error) {
	stringDuration := string(d)
	duration, err := time.ParseDuration(stringDuration)
	if err != nil {
		return 0, err
	}
	seconds = int(duration.Seconds())

	return seconds, nil
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
