// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package list

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/aws/copilot-cli/internal/pkg/config"
)

const (
	// Display settings.
	minCellWidth           = 20  // minimum number of characters in a table's cell.
	tabWidth               = 4   // number of characters in between columns.
	cellPaddingWidth       = 2   // number of padding characters added by default to a cell.
	paddingChar            = ' ' // character in between columns.
	noAdditionalFormatting = 0

	// Workload types.
	jobWorkloadType = "job"
	svcWorkloadType = "service"
)

// Store wraps the methods required for interacting with config stores.
type Store interface {
	GetApplication(appName string) (*config.Application, error)
	ListJobs(appName string) ([]*config.Workload, error)
	ListServices(appName string) ([]*config.Workload, error)
}

// Workspace wraps the methods required to interact with a local workspace.
type Workspace interface {
	ListJobs() ([]string, error)
	ListServices() ([]string, error)
}

// JobListWriter holds all the metadata and clients needed to list all jobs in a given
// workspace or app in a human- or machine-readable format.
type JobListWriter struct {
	// Output configuration options.
	ShowLocalJobs bool
	OutputJSON    bool

	Store Store     // Client to retrieve application configuration and job metadata.
	Ws    Workspace // Client to retrieve local jobs.
	Out   io.Writer // The writer where output will be written.
}

// SvcListWriter holds all the metadata and clients needed to list all services in a given
// workspace or app in a human- or machine-readable format.
type SvcListWriter struct {
	ShowLocalSvcs bool
	OutputJSON    bool

	Store Store     // Client to retrieve application configuration and service metadata.
	Ws    Workspace // Client to retrieve local jobs.
	Out   io.Writer // The writer where output will be written.
}

// ServiceJSONOutput is the output struct for service list.
type ServiceJSONOutput struct {
	Services []*config.Workload `json:"services"`
}

// JobJSONOutput is the output struct for job list.
type JobJSONOutput struct {
	Jobs []*config.Workload `json:"jobs"`
}

// Jobs lists all jobs, either locally or in the workspace, and writes the output to a writer.
func (l *JobListWriter) Write(appName string) error {
	if _, err := l.Store.GetApplication(appName); err != nil {
		return fmt.Errorf("get application: %w", err)
	}
	wklds, err := l.Store.ListJobs(appName)
	if err != nil {
		return fmt.Errorf("get %s names: %w", jobWorkloadType, err)
	}
	if l.ShowLocalJobs {
		localWklds, err := l.Ws.ListJobs()
		if err != nil {
			return fmt.Errorf("get local %s names: %w", jobWorkloadType, err)
		}
		wklds = filterByName(wklds, localWklds)
	}
	if l.OutputJSON {
		data, err := l.jsonOutputJobs(wklds)
		if err != nil {
			return err
		}
		fmt.Fprint(l.Out, data)
	} else {
		humanOutput(wklds, l.Out)
	}
	return nil
}

// Write lists all services, either locally or in the workspace, and writes the output to a writer.
func (l *SvcListWriter) Write(appName string) error {
	if _, err := l.Store.GetApplication(appName); err != nil {
		return fmt.Errorf("get application: %w", err)
	}
	wklds, err := l.Store.ListServices(appName)
	if err != nil {
		return fmt.Errorf("get %s names: %w", svcWorkloadType, err)
	}
	if l.ShowLocalSvcs {
		localWklds, err := l.Ws.ListServices()
		if err != nil {
			return fmt.Errorf("get local %s names: %w", svcWorkloadType, err)
		}
		wklds = filterByName(wklds, localWklds)
	}
	if l.OutputJSON {
		data, err := l.jsonOutputSvcs(wklds)
		if err != nil {
			return err
		}
		fmt.Fprint(l.Out, data)
	} else {
		humanOutput(wklds, l.Out)
	}
	return nil
}

func filterByName(wklds []*config.Workload, wantedNames []string) []*config.Workload {
	isWanted := make(map[string]bool)
	for _, name := range wantedNames {
		isWanted[name] = true
	}
	var filtered []*config.Workload
	for _, wkld := range wklds {
		if isWanted[wkld.Name] {
			filtered = append(filtered, wkld)
		}
	}
	return filtered
}

func underline(headings []string) []string {
	var lines []string
	for _, heading := range headings {
		line := strings.Repeat("-", len(heading))
		lines = append(lines, line)
	}
	return lines
}

func humanOutput(wklds []*config.Workload, w io.Writer) {
	writer := tabwriter.NewWriter(w, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	headers := []string{"Name", "Type"}
	fmt.Fprintf(writer, "%s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "%s\n", strings.Join(underline(headers), "\t"))
	for _, wkld := range wklds {
		fmt.Fprintf(writer, "%s\t%s\n", wkld.Name, wkld.Type)
	}
	writer.Flush()
}

func (l *SvcListWriter) jsonOutputSvcs(svcs []*config.Workload) (string, error) {
	b, err := json.Marshal(ServiceJSONOutput{Services: svcs})
	if err != nil {
		return "", fmt.Errorf("marshal services: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

func (l *JobListWriter) jsonOutputJobs(jobs []*config.Workload) (string, error) {
	b, err := json.Marshal(JobJSONOutput{Jobs: jobs})
	if err != nil {
		return "", fmt.Errorf("marshal jobs: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}
