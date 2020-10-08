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

var workloadTypes = []string{jobWorkloadType, svcWorkloadType}

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

// Lister handles
type Lister struct {
	store Store
	ws    Workspace
	w     io.Writer
}

// NewLister returns a new Lister structure, which can be used to list Jobs or Services
// from both the config store and workspace.
func NewLister(ws Workspace, store Store, writer io.Writer) *Lister {
	return &Lister{
		store: store,
		ws:    ws,
		w:     writer,
	}
}

// Jobs lists all jobs, either locally or in the workspace, and writes the output to a writer.
func (l *Lister) Jobs(appName string, showLocal bool, writeJSON bool) error {
	if _, err := l.store.GetApplication(appName); err != nil {
		return fmt.Errorf("get application: %w", err)
	}
	wklds, err := l.store.ListJobs(appName)
	if err != nil {
		return fmt.Errorf("get %s names: %w", jobWorkloadType, err)
	}
	if showLocal {
		localWklds, err := l.ws.JobNames()
		if err != nil {
			return fmt.Errorf("get local %s names: %w", jobWorkloadType, err)
		}
		wklds = filterByName(wklds, localWklds)
	}
	if writeJSON {
		data, err := l.jsonOutputJobs(wklds)
		if err != nil {
			return err
		}
		fmt.Fprint(l.w, data)
	} else {
		l.humanOutput(wklds)
	}
	return nil
}

// Services lists all services, either locally or in the workspace, and writes the output to a writer.
func (l *Lister) Services(appName string, showLocal bool, writeJSON bool) error {
	if _, err := l.store.GetApplication(appName); err != nil {
		return fmt.Errorf("get application: %w", err)
	}
	wklds, err := l.store.ListServices(appName)
	if err != nil {
		return fmt.Errorf("get %s names: %w", svcWorkloadType, err)
	}
	if showLocal {
		localWklds, err := l.ws.ServiceNames()
		if err != nil {
			return fmt.Errorf("get local %s names: %w", svcWorkloadType, err)
		}
		wklds = filterByName(wklds, localWklds)
	}
	if writeJSON {
		data, err := l.jsonOutputSvcs(wklds)
		if err != nil {
			return err
		}
		fmt.Fprint(l.w, data)
	} else {
		l.humanOutput(wklds)
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

func (l *Lister) humanOutput(wklds []*config.Workload) {
	writer := tabwriter.NewWriter(l.w, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
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

func (l *Lister) jsonOutputSvcs(svcs []*config.Workload) (string, error) {
	type out struct {
		Services []*config.Workload `json:"services"`
	}
	b, err := json.Marshal(out{Services: svcs})
	if err != nil {
		return "", fmt.Errorf("marshal services: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

func (l *Lister) jsonOutputJobs(jobs []*config.Workload) (string, error) {
	type out struct {
		Jobs []*config.Workload `json:"jobs"`
	}
	b, err := json.Marshal(out{Jobs: jobs})
	if err != nil {
		return "", fmt.Errorf("marshal jobs: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}
