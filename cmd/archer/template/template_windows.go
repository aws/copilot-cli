// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package template provides usage templates to render help menus.
package template

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/cli/groups"
)

// RootUsage is the text template for the root command.
var RootUsage = fmt.Sprintf("{{\"Commands\"}}{{ $cmds := .Commands }}{{$groups := mkSlice \"%s\" \"%s\" \"%s\" }}{{range $group := $groups }} \n",
	groups.GettingStarted, groups.Develop, groups.Settings) +
	`  {{$group}}{{range $cmd := $cmds}}{{if isInGroup $cmd $group}}
    {{rpad $cmd.Name $cmd.NamePadding}} {{$cmd.Short}}{{end}}{{end}}
{{end}}{{if .HasAvailableLocalFlags}}
{{"Flags"}}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

{{"Global Flags"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

{{"Examples"}}{{.Example}}{{end}}
`

// Usage is the text template for a single command.
const Usage = `{{"Usage"}}{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]

{{"Available Commands"}}{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{"Flags"}}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

{{"Global Flags"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

{{"Examples"}}{{.Example}}{{end}}
`

func init() {
	cobra.AddTemplateFunc("isInGroup", isInGroup)
	cobra.AddTemplateFunc("mkSlice", mkSlice)
}

func isInGroup(cmd *cobra.Command, group string) bool {
	return cmd.Annotations["group"] == group
}

func mkSlice(args ...interface{}) []interface{} {
	return args
}
