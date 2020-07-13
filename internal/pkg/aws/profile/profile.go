// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package profile provides functionality to parse AWS named profiles.
package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/ini"
)

const (
	awsCredentialsDir = ".aws"
	awsConfigFileName = "config"
)

type sectionsGetter interface {
	Sections() []string
}

// Config represents the local AWS config file.
type Config struct {
	// f is the ~/.aws/config INI file.
	f sectionsGetter
}

// NewConfig returns a new parsed Config object from $HOME/.aws/config.
func NewConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	cfgPath := filepath.Join(homeDir, awsCredentialsDir, awsConfigFileName)
	cfg, err := ini.New(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("read AWS config file %s, you might need to run %s first: %w", cfgPath, "aws configure", err)
	}
	return &Config{
		f: cfg,
	}, nil
}

// Names returns a list of profile names available in the user's config file.
// An error is returned if the config file can't be found or parsed.
func (c *Config) Names() []string {
	var profiles []string
	for _, section := range c.f.Sections() {
		// Named profiles created with "aws configure" are formatted as "[profile test]".
		profiles = append(profiles, strings.TrimSpace(strings.TrimPrefix(section, "profile")))
	}
	return profiles
}
