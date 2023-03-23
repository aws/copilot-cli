// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"gopkg.in/yaml.v3"
)

type formatter interface {
	formatYAML(*yaml.Node) ([]byte, error)
	formatMod(node diffNode) string
	formatPath(node diffNode) string
	childIndent(curr int) int
}

type seqItemFormatter struct{}

func (f *seqItemFormatter) formatYAML(node *yaml.Node) ([]byte, error) {
	wrapped := &yaml.Node{
		Kind:    yaml.SequenceNode,
		Tag:     "!!seq",
		Content: []*yaml.Node{node},
	}
	return yaml.Marshal(wrapped)
}

func (f *seqItemFormatter) formatMod(node diffNode) string {
	return fmt.Sprintf("- %s -> %s", node.oldYAML().Value, node.newYAML().Value)
}

func (f *seqItemFormatter) formatPath(_ diffNode) string {
	return color.Faint.Sprint("- (changed item)")
}

func (f *seqItemFormatter) childIndent(curr int) int {
	/* A seq item diff should look like:
	   - (item)
	     ~ Field1: a
	     + Field2: b
	   Where "~ Filed1: a" and "+ Field2: b" are its children. The indentation should increase by len("- "), which is 2.
	*/
	return curr + 2
}

type keyedFormatter struct {
	key string
}

func (f *keyedFormatter) formatYAML(node *yaml.Node) ([]byte, error) {
	wrapped := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: f.key,
			},
			node,
		},
	}
	return yaml.Marshal(wrapped)
}

func (f *keyedFormatter) formatMod(node diffNode) string {
	return fmt.Sprintf("%s: %s -> %s", node.key(), node.oldYAML().Value, node.newYAML().Value)
}

func (f *keyedFormatter) formatPath(node diffNode) string {
	return node.key() + ":"
}

func (f *keyedFormatter) childIndent(curr int) int {
	return curr + indentInc
}

type documentFormatter struct{}

func (f *documentFormatter) formatYAML(node *yaml.Node) ([]byte, error) {
	return yaml.Marshal(node)
}

func (f *documentFormatter) formatMod(_ diffNode) string {
	return ""
}

func (f *documentFormatter) formatPath(_ diffNode) string {
	return ""
}

func (f *documentFormatter) childIndent(curr int) int {
	return curr + indentInc
}

func prefixByFn(prefix string) func(line string) string {
	return func(line string) string {
		return fmt.Sprintf("%s %s", prefix, line)
	}
}

func indentByFn(count int) func(line string) string {
	return func(line string) string {
		return fmt.Sprintf("%s%s", strings.Repeat(" ", count), line)
	}
}

func process(line string, fn ...func(line string) string) string {
	for _, f := range fn {
		line = f(line)
	}
	return line
}

func processMultiline(multiline string, fn ...func(line string) string) string {
	var processed []string
	for _, line := range strings.Split(strings.TrimRight(multiline, "\n"), "\n") {
		processed = append(processed, process(line, fn...))
	}
	return strings.Join(processed, "\n")
}
