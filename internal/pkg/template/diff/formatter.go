// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type formatter interface {
	formatYAML(*yaml.Node) ([]byte, error)
	formatMod(node diffNode) string
	formatPath(node diffNode) string
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
	return fmt.Sprintf("- %s -> %s", node.oldValue().Value, node.newValue().Value)
}

func (f *seqItemFormatter) formatPath(_ diffNode) string {
	return ""
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
	return fmt.Sprintf("%s: %s -> %s", node.key(), node.oldValue().Value, node.newValue().Value)
}

func (f *keyedFormatter) formatPath(node diffNode) string {
	return node.key() + ":"
}

func prefixBy(prefix string) func(line string) string {
	return func(line string) string {
		return fmt.Sprintf("%s %s", prefix, line)
	}
}

func indentBy(count int) func(line string) string {
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
