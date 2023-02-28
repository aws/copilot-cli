// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package templatetest provides test doubles for embedded templates.
package templatetest

import (
	"bytes"

	"github.com/aws/copilot-cli/internal/pkg/template"
)

// Stub stubs template.New and simulates successful read and parse calls.
type Stub struct{}

// Read returns a dummy template.Content with "data" in it.
func (fs Stub) Read(_ string) (*template.Content, error) {
	return &template.Content{
		Buffer: bytes.NewBufferString("data"),
	}, nil
}

// Parse returns a dummy template.Content with "data" in it.
func (fs Stub) Parse(_ string, _ interface{}, _ ...template.ParseOption) (*template.Content, error) {
	return &template.Content{
		Buffer: bytes.NewBufferString("data"),
	}, nil
}

// ParseBackendService returns a dummy template.Content with "data" in it.
func (fs Stub) ParseBackendService(_ template.WorkloadOpts) (*template.Content, error) {
	return &template.Content{
		Buffer: bytes.NewBufferString("data"),
	}, nil
}

// ParseEnv returns a dummy template.Content with "data" in it.
func (fs Stub) ParseEnv(_ *template.EnvOpts) (*template.Content, error) {
	return &template.Content{
		Buffer: bytes.NewBufferString("data"),
	}, nil
}

// ParseEnvBootstrap returns a dummy template.Content with "data" in it.
func (fs Stub) ParseEnvBootstrap(data *template.EnvOpts, options ...template.ParseOption) (*template.Content, error) {
	return &template.Content{
		Buffer: bytes.NewBufferString("data"),
	}, nil
}

// ParseLoadBalancedWebService returns a dummy template.Content with "data" in it.
func (fs Stub) ParseLoadBalancedWebService(_ template.WorkloadOpts) (*template.Content, error) {
	return &template.Content{
		Buffer: bytes.NewBufferString("data"),
	}, nil
}

// ParseRequestDrivenWebService returns a dummy template.Content with "data" in it.
func (fs Stub) ParseRequestDrivenWebService(_ template.WorkloadOpts) (*template.Content, error) {
	return &template.Content{
		Buffer: bytes.NewBufferString("data"),
	}, nil
}

// ParseScheduledJob returns a dummy template.Content with "data" in it.
func (fs Stub) ParseScheduledJob(_ template.WorkloadOpts) (*template.Content, error) {
	return &template.Content{
		Buffer: bytes.NewBufferString("data"),
	}, nil
}

// ParseWorkerService returns a dummy template.Content with "data" in it.
func (fs Stub) ParseWorkerService(_ template.WorkloadOpts) (*template.Content, error) {
	return &template.Content{
		Buffer: bytes.NewBufferString("data"),
	}, nil
}

// ParseStaticSite returns a dummy template.Content with "data" in it.
func (fs Stub) ParseStaticSite(_ template.WorkloadOpts) (*template.Content, error) {
	return &template.Content{
		Buffer: bytes.NewBufferString("data"),
	}, nil
}
