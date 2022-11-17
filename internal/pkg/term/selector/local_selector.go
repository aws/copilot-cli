// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package selector

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/lnquy/cron"
	"github.com/spf13/afero"
)

const (
	dockerfileName   = "dockerfile"
	dockerignoreName = ".dockerignore"
)

type configurationSelector struct {
	prompt prompter
}

// NewConfigurationSelector constructs a configurationSelector.
func NewConfigurationSelector(prompt prompter) *configurationSelector {
	return &configurationSelector{
		prompt: prompt,
	}
}

// Schedule asks the user to select either a rate, preset cron, or custom cron.
func (s *configurationSelector) Schedule(scheduleTypePrompt, scheduleTypeHelp string, scheduleValidator, rateValidator prompt.ValidatorFunc) (string, error) {
	scheduleType, err := s.prompt.SelectOne(
		scheduleTypePrompt,
		scheduleTypeHelp,
		scheduleTypes,
		prompt.WithFinalMessage("Schedule type:"),
	)
	if err != nil {
		return "", fmt.Errorf("get schedule type: %w", err)
	}
	switch scheduleType {
	case rate:
		return s.askRate(rateValidator)
	case fixedSchedule:
		return s.askCron(scheduleValidator)
	default:
		return "", fmt.Errorf("unrecognized schedule type %s", scheduleType)
	}
}

func (s *configurationSelector) askRate(rateValidator prompt.ValidatorFunc) (string, error) {
	rateInput, err := s.prompt.Get(
		ratePrompt,
		rateHelp,
		rateValidator,
		prompt.WithDefaultInput("1h30m"),
		prompt.WithFinalMessage("Rate:"),
	)
	if err != nil {
		return "", fmt.Errorf("get schedule rate: %w", err)
	}
	return fmt.Sprintf(every, rateInput), nil
}

func (s *configurationSelector) askCron(scheduleValidator prompt.ValidatorFunc) (string, error) {
	cronInput, err := s.prompt.SelectOption(
		schedulePrompt,
		scheduleHelp,
		presetSchedules,
		prompt.WithFinalMessage("Fixed schedule:"),
	)
	if err != nil {
		return "", fmt.Errorf("get preset schedule: %w", err)
	}
	if cronInput != custom {
		return presetScheduleToDefinitionString(cronInput), nil
	}
	var customSchedule, humanCron string
	cronDescriptor, err := cron.NewDescriptor()
	if err != nil {
		return "", fmt.Errorf("get custom schedule: %w", err)
	}
	for {
		customSchedule, err = s.prompt.Get(
			customSchedulePrompt,
			customScheduleHelp,
			scheduleValidator,
			prompt.WithDefaultInput("0 * * * *"),
			prompt.WithFinalMessage("Custom schedule:"),
		)
		if err != nil {
			return "", fmt.Errorf("get custom schedule: %w", err)
		}

		// Break if the customer has specified an easy to read cron definition string
		if strings.HasPrefix(customSchedule, "@") {
			break
		}

		humanCron, err = cronDescriptor.ToDescription(customSchedule, cron.Locale_en)
		if err != nil {
			return "", fmt.Errorf("convert cron to human string: %w", err)
		}

		log.Infoln(fmt.Sprintf("Your job will run at the following times: %s", humanCron))

		ok, err := s.prompt.Confirm(
			humanReadableCronConfirmPrompt,
			humanReadableCronConfirmHelp,
		)
		if err != nil {
			return "", fmt.Errorf("confirm cron schedule: %w", err)
		}
		if ok {
			break
		}
	}

	return customSchedule, nil
}

// localFileSelector selects from a local file system where a workspace does not necessarily exist.
type localFileSelector struct {
	prompt        prompter
	fs            *afero.Afero
	workingDirAbs string
}

// NewLocalFileSelector constructs a localFileSelector.
func NewLocalFileSelector(prompt prompter, fs afero.Fs) (*localFileSelector, error) {
	workingDirAbs, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	return &localFileSelector{
		prompt:        prompt,
		fs:            &afero.Afero{Fs: fs},
		workingDirAbs: workingDirAbs,
	}, nil
}

// Dockerfile asks the user to select from a list of Dockerfiles in the current
// directory or one level down. If no dockerfiles are found, it asks for a custom path.
func (s *localFileSelector) Dockerfile(selPrompt, notFoundPrompt, selHelp, notFoundHelp string, pathValidator prompt.ValidatorFunc) (string, error) {
	dockerfiles, err := s.listDockerfiles()
	if err != nil {
		return "", err
	}
	var sel string
	dockerfiles = append(dockerfiles, []string{dockerfilePromptUseCustom, DockerfilePromptUseImage}...)
	sel, err = s.prompt.SelectOne(
		selPrompt,
		selHelp,
		dockerfiles,
		prompt.WithFinalMessage(dockerfileFinalMsg),
	)
	if err != nil {
		return "", fmt.Errorf("select Dockerfile: %w", err)
	}
	if sel != dockerfilePromptUseCustom {
		return sel, nil
	}
	sel, err = s.prompt.Get(
		notFoundPrompt,
		notFoundHelp,
		pathValidator,
		prompt.WithFinalMessage(dockerfileFinalMsg))
	if err != nil {
		return "", fmt.Errorf("get custom Dockerfile path: %w", err)
	}
	return sel, nil
}

// ListDockerfiles returns the list of Dockerfiles within the current
// working directory and a subdirectory level below. If an error occurs while
// reading directories, or no Dockerfiles found returns the error.
func (s *localFileSelector) listDockerfiles() ([]string, error) {
	wdFiles, err := s.fs.ReadDir(s.workingDirAbs)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}
	var dockerfiles = make([]string, 0)
	for _, wdFile := range wdFiles {
		// Add current file if it is a Dockerfile and not a directory; otherwise continue.
		if !wdFile.IsDir() {
			fname := wdFile.Name()
			if strings.Contains(strings.ToLower(fname), dockerfileName) && !strings.HasSuffix(strings.ToLower(fname), dockerignoreName) {
				path := filepath.Dir(fname) + "/" + fname
				dockerfiles = append(dockerfiles, path)
			}
			continue
		}

		// Add sub-directories containing a Dockerfile one level below current directory.
		subFiles, err := s.fs.ReadDir(wdFile.Name())
		if err != nil {
			// swallow errors for unreadable directories
			continue
		}
		for _, f := range subFiles {
			// NOTE: ignore directories in sub-directories.
			if f.IsDir() {
				continue
			}
			fname := f.Name()
			if strings.Contains(strings.ToLower(fname), dockerfileName) && !strings.HasSuffix(strings.ToLower(fname), dockerignoreName) {
				path := wdFile.Name() + "/" + f.Name()
				dockerfiles = append(dockerfiles, path)
			}
		}
	}
	sort.Strings(dockerfiles)
	return dockerfiles, nil
}

func presetScheduleToDefinitionString(input string) string {
	return fmt.Sprintf("@%s", strings.ToLower(input))
}
