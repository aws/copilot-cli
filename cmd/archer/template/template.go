// +build !windows

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package template provides usage templates to render help menus.
package template

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/cli/groups"
)

// RootUsage is the text template for the root command.
var RootUsage = fmt.Sprintf("{{h1 \"Commands\"}}{{ $cmds := .Commands }}{{$groups := mkSlice \"%s\" \"%s\" \"%s\" }}{{range $group := $groups }} \n",
	groups.GettingStarted, groups.Develop, groups.Settings) +
	`  {{h2 $group}}{{range $cmd := $cmds}}{{if isInGroup $cmd $group}}
    {{rpad $cmd.Name $cmd.NamePadding}} {{$cmd.Short}}{{end}}{{end}}
{{end}}{{if .HasAvailableLocalFlags}}
{{h1 "Flags"}}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

{{h1 "Global Flags"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

{{h1 "Examples"}}{{code .Example}}{{end}}
`

// Usage is the text template for a single command.
const Usage = `{{h1 "Usage"}}{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]

{{h1 "Available Commands"}}{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{h1 "Flags"}}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

{{h1 "Global Flags"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

{{h1 "Examples"}}{{code .Example}}{{end}}
`

func init() {
	cobra.AddTemplateFunc("isInGroup", isInGroup)
	cobra.AddTemplateFunc("h1", h1)
	cobra.AddTemplateFunc("h2", h2)
	cobra.AddTemplateFunc("code", code)
	cobra.AddTemplateFunc("mkSlice", mkSlice)
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

func code(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "$") {
			// code sample
			lines[i] = color.HiBlackString(line)
		}
	}
	return strings.Join(lines, "\n")
}

func mkSlice(args ...interface{}) []interface{} {
	return args
}
