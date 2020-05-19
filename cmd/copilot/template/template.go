// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
)

// RootUsage is the text template for the root command.
var RootUsage = fmt.Sprintf("{{h1 \"Commands\"}}{{ $cmds := .Commands }}{{$groups := mkSlice \"%s\" \"%s\" \"%s\" \"%s\" }}{{range $group := $groups }} \n",
	group.GettingStarted, group.Develop, group.Release, group.Settings) +
	`  {{h2 $group}}{{$groupCmds := (filterCmdsByGroup $cmds $group)}}
{{- range $j, $cmd := $groupCmds}}{{$lines := split $cmd.Short "\n"}}
{{- range $i, $line := $lines}}
    {{if eq $i 0}}{{rpad $cmd.Name $cmd.NamePadding}} {{$line}}
    {{- else}}{{rpad "" $cmd.NamePadding}} {{$line}}
{{- end}}{{end}}{{if and (gt (len $lines) 1) (ne (inc $j) (len $groupCmds))}}
{{end}}{{end}}
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
	cobra.AddTemplateFunc("filterCmdsByGroup", filterCmdsByGroup)
	cobra.AddTemplateFunc("h1", h1)
	cobra.AddTemplateFunc("h2", h2)
	cobra.AddTemplateFunc("code", code)
	cobra.AddTemplateFunc("mkSlice", mkSlice)
	cobra.AddTemplateFunc("split", split)
	cobra.AddTemplateFunc("inc", inc)
}
