// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ini provides functionality to parse and read properties from INI files.
package ini

import (
	"fmt"

	"gopkg.in/ini.v1"
)

type sectionsParser interface {
	Sections() []*ini.Section
}

// INI represents a parsed INI file in memory.
type INI struct {
	cfg sectionsParser
}

// New returns an INI file given a path to the file.
// An error is returned if the file can't be parsed.
func New(path string) (*INI, error) {
	cfg, err := ini.Load(path)
	if err != nil {
		return nil, fmt.Errorf("load ini file %s: %w", path, err)
	}
	return &INI{
		cfg: cfg,
	}, nil
}

// Sections returns the names of **non-empty** sections in the file.
//
// For example, the method returns ["paths", "servers"] if the file's content is:
//  app_mode = development
//  [paths]
//  data = /home/git/grafana
//  [server]
//  protocol = http
//  http_port = 9999
func (i *INI) Sections() []string {
	var names []string
	for _, section := range i.cfg.Sections() {
		if len(section.Keys()) == 0 {
			continue
		}
		names = append(names, section.Name())
	}
	return names
}
