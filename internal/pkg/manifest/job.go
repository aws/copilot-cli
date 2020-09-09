// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to create Manifest files.
package manifest

import (
	"errors"
	"fmt"
	"regexp"
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
type Schedule string

func (s *Schedule) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	if err := unmarshal(&rawString); err != nil {
		return err
	}
	if err := s.parseCron(rawString); err != nil {
		switch err {
		case errStringNotCron:
			break
		default:
			return err
		}
	}

	// If we could successfully parse out a value, return. Otherwise, try
	// parsing it as a rate.
	if s != nil {
		return nil
	}

	if err := s.parseRate(); err != nil {
		return err
	}
	if s != nil {
		return nil
	}

	return errStringNeitherDurationNorCron
}

func (s *Schedule) parseCron(input string) error {
	_, err := cron.ParseStandard(input)
	if err != nil {
		return errStringNotCron
	}
	predefinedMatch := regexpPredefined.FindStringSubmatch(input)
	if len(predefinedMatch) != 0 {
		s = fmt.Sprintf("cron(%s)", predefinedSchedules[predefinedMatch[1]])
		return nil
	}
	everyMatch, err := regexpEvery.FindStringSubmatch(s)
	if err != nil {
		return err
	}
	if len(everyMatch) != 0 {
		var d Duration = everyMatch[1]
		s, err = d.ToRate()
		if err != nil {
			return err
		}
		return nil
	}

	return errStringNotCron
}

// Duration is a string of the form 90m, 30s, 24h.
type Duration string

// ToSeconds converts the duration string into an integer number of seconds
func (d Duration) ToSeconds() (seconds int, err error) {
	duration, err := time.ParseDuration(d)
	if err != nil {
		return 0, err
	}
	seconds = int(duration.Seconds())

	return seconds, nil
}

func (d Duration) ToMinutes() (minutes int, err error) {
	duration, err := time.ParseDuration(d)
	if err != nil {
		return 0, err
	}
	minutes = int(duration.Minutes())

	return minutes, nil
}

// ToRate converts the duration string into a rate string valid for
// Cloudwatch Events schedule expressions
func (d Duration) ToRate() (rate string, err error) {

}
