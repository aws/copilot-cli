// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package list

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
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
	JobNames() ([]string, error)
	ServiceNames() ([]string, error)
}

// JobListWriter holds all the metadata needed to
type JobListWriter struct {
	ShowLocalJobs bool
	OutputJSON    bool

	Store Store
	Ws    Workspace
	W     io.Writer
}

// SvcListWriter holds all the metadata needed to write local or remote services to a writer
type SvcListWriter struct {
	ShowLocalSvcs bool
	OutputJSON    bool

	Store Store
	Ws    Workspace
	W     io.Writer
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
		localWklds, err := l.Ws.JobNames()
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
		fmt.Fprint(l.W, data)
	} else {
		humanOutput(wklds, l.W)
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
		localWklds, err := l.Ws.ServiceNames()
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
		fmt.Fprint(l.W, data)
	} else {
		humanOutput(wklds, l.W)
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
		if _, ok := isWanted[wkld.Name]; !ok {
			continue
		}
		filtered = append(filtered, wkld)
	}
	return filtered
}

func humanOutput(wklds []*config.Workload, w io.Writer) {
	writer := tabwriter.NewWriter(w, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, "%s\t%s\n", "Name", "Type")
	nameLengthMax := len("Name")
	typeLengthMax := len("Type")
	for _, svc := range wklds {
		nameLengthMax = int(math.Max(float64(nameLengthMax), float64(len(svc.Name))))
		typeLengthMax = int(math.Max(float64(typeLengthMax), float64(len(svc.Type))))
	}
	fmt.Fprintf(writer, "%s\t%s\n", strings.Repeat("-", nameLengthMax), strings.Repeat("-", typeLengthMax))
	for _, wkld := range wklds {
		fmt.Fprintf(writer, "%s\t%s\n", wkld.Name, wkld.Type)
	}
	writer.Flush()
}

func (l *SvcListWriter) jsonOutputSvcs(svcs []*config.Workload) (string, error) {
	type out struct {
		Services []*config.Workload `json:"services"`
	}
	b, err := json.Marshal(out{Services: svcs})
	if err != nil {
		return "", fmt.Errorf("marshal services: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

func (l *JobListWriter) jsonOutputJobs(jobs []*config.Workload) (string, error) {
	type out struct {
		Jobs []*config.Workload `json:"jobs"`
	}
	b, err := json.Marshal(out{Jobs: jobs})
	if err != nil {
		return "", fmt.Errorf("marshal jobs: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}
