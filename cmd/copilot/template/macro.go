// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package template provides usage templates to render help menus.
package template

import (
	"strings"

	termcolor "github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func isInGroup(cmd *cobra.Command, group string) bool {
	return cmd.Annotations["group"] == group
}

func filterCmdsByGroup(cmds []*cobra.Command, group string) []*cobra.Command {
	var filtered []*cobra.Command
	for _, cmd := range cmds {
		if isInGroup(cmd, group) {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
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

	var startCodeBlockIdx, codeBlockPadding int
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "/code ") {
			codeIndex := strings.Index(line, "/code ")
			lines[i] = line[:codeIndex] +
				termcolor.HighlightCode(strings.ReplaceAll(line[codeIndex:], "/code ", ""))
		}
		if strings.HasPrefix(strings.TrimSpace(line), "/startcodeblock") {
			startCodeBlockIdx = i
			codeBlockPadding = strings.Index(line, "/startcodeblock")
		}
		if strings.HasPrefix(strings.TrimSpace(line), "/endcodeblock") {
			colored := termcolor.HighlightCodeBlock(strings.Join(lines[startCodeBlockIdx+1:i], "\n"))
			coloredLines := strings.Split(colored, "\n")
			for j, val := range coloredLines {
				padding := ""
				if j == 0 || j == len(coloredLines)-1 {
					padding = strings.Repeat(" ", codeBlockPadding)
				}
				lines[startCodeBlockIdx+j] = padding + val
			}
		}
	}
	return strings.Join(lines, "\n")
}

func mkSlice(args ...interface{}) []interface{} {
	return args
}

func split(s string, sep string) []string {
	return strings.Split(s, sep)
}

func inc(n int) int {
	return n + 1
}
