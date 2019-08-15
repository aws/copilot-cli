// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package template provides usage templates to render help menus.
package template

import (
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// RootUsage is the text template for the root command.
const RootUsage = `{{h1 "Commands"}}
  {{h2 "Getting Started"}}{{range .Commands}}{{if isInGroup . "Getting Started"}}
    {{rpad .Name .NamePadding}} {{.Short}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{h1 "Flags"}}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

{{h1 "Global Flags"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

{{h1 "Examples"}}{{.Example}}{{end}}
`

// Usage is the text template for a single command.
const Usage = `{{h1 "Usage"}}{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if .HasAvailableLocalFlags}}

{{h1 "Flags"}}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

{{h1 "Global Flags"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

{{h1 "Examples"}}{{.Example}}{{end}}
`

func init() {
	cobra.AddTemplateFunc("isInGroup", isInGroup)
	cobra.AddTemplateFunc("h1", h1)
	cobra.AddTemplateFunc("h2", h2)
}

func isInGroup(cmd *cobra.Command, group string) bool {
	return cmd.Annotations["group"] == group
}

func h1(text string) string {
	var s strings.Builder
	color.New(color.Bold, color.Underline).Fprintf(&s, text)
	return s.String()
}

func h2(text string) string {
	var s strings.Builder
	color.New(color.Bold).Fprintf(&s, text)
	return s.String()
}
