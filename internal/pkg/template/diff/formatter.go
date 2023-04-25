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
	formatInsert(node diffNode) (string, error)
	formatDel(node diffNode) (string, error)
	formatMod(node diffNode) (string, error)
	formatPath(node diffNode) string
	nextIndent() int
}

type seqItemFormatter struct {
	indent int
}

func (f *seqItemFormatter) formatDel(node diffNode) (string, error) {
	raw, err := yaml.Marshal(&yaml.Node{
		Kind:    yaml.SequenceNode,
		Tag:     "!!seq",
		Content: []*yaml.Node{node.oldYAML()},
	})
	if err != nil {
		return "", err
	}
	return processMultiline(string(raw), prefixByFn(prefixDel), indentByFn(f.indent)), nil
}

func (f *seqItemFormatter) formatInsert(node diffNode) (string, error) {
	raw, err := yaml.Marshal(&yaml.Node{
		Kind:    yaml.SequenceNode,
		Tag:     "!!seq",
		Content: []*yaml.Node{node.newYAML()},
	})
	if err != nil {
		return "", err
	}
	return processMultiline(string(raw), prefixByFn(prefixAdd), indentByFn(f.indent)), nil
}

func (f *seqItemFormatter) formatMod(node diffNode) (string, error) {
	oldValue, newValue, err := marshalValues(node)
	if err != nil {
		return "", err
	}
	content := fmt.Sprintf("- %s -> %s", oldValue, newValue)
	return processMultiline(content, prefixByFn(prefixMod), indentByFn(f.indent)), nil
}

func (f *seqItemFormatter) formatPath(node diffNode) string {
	return process(color.Faint.Sprint("- (changed item)"), prefixByFn(prefixMod), indentByFn(f.indent)) + "\n"
}

func (f *seqItemFormatter) nextIndent() int {
	/* A seq item diff should look like:
	   - (item)
	     ~ Field1: a
	     + Field2: b
	   Where "~ Field1: a" and "+ Field2: b" are its children. The indentation should increase by len("- "), which is 2.
	*/
	return f.indent + 2
}

type keyedFormatter struct {
	indent int
}

func (f *keyedFormatter) formatDel(node diffNode) (string, error) {
	raw, err := yaml.Marshal(&yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: node.key(),
			},
			node.oldYAML(),
		},
	})
	if err != nil {
		return "", err
	}
	return processMultiline(string(raw), prefixByFn(prefixDel), indentByFn(f.indent)), nil
}

func (f *keyedFormatter) formatInsert(node diffNode) (string, error) {
	raw, err := yaml.Marshal(&yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: node.key(),
			},
			node.newYAML(),
		},
	})
	if err != nil {
		return "", err
	}
	return processMultiline(string(raw), prefixByFn(prefixAdd), indentByFn(f.indent)), nil
}

func (f *keyedFormatter) formatMod(node diffNode) (string, error) {
	oldValue, newValue, err := marshalValues(node)
	if err != nil {
		return "", err
	}
	content := fmt.Sprintf("%s: %s -> %s", node.key(), oldValue, newValue)
	return processMultiline(content, prefixByFn(prefixMod), indentByFn(f.indent)), nil
}

func (f *keyedFormatter) formatPath(node diffNode) string {
	return process(node.key()+":"+"\n", prefixByFn(prefixMod), indentByFn(f.indent))
}

func (f *keyedFormatter) nextIndent() int {
	return f.indent + indentInc
}

type documentFormatter struct{}

func (f *documentFormatter) formatMod(_ diffNode) (string, error) {
	return "", nil
}

func (f *documentFormatter) formatDel(node diffNode) (string, error) {
	raw, err := yaml.Marshal(node.oldYAML())
	if err != nil {
		return "", err
	}
	return processMultiline(string(raw), prefixByFn(prefixDel), indentByFn(0)), nil
}

func (f *documentFormatter) formatInsert(node diffNode) (string, error) {
	raw, err := yaml.Marshal(node.newYAML())
	if err != nil {
		return "", err
	}
	return processMultiline(string(raw), prefixByFn(prefixAdd), indentByFn(0)), nil
}

func (f *documentFormatter) formatPath(_ diffNode) string {
	return ""
}

func (f *documentFormatter) nextIndent() int {
	return 0
}

func marshalValues(node diffNode) (string, string, error) {
	var oldValue, newValue string
	if v, err := yaml.Marshal(node.oldYAML()); err != nil { // NOTE: Marshal handles YAML tags such as `!Ref` and `!Sub`.
		return "", "", err
	} else {
		oldValue = strings.TrimSuffix(string(v), "\n")
	}
	if v, err := yaml.Marshal(node.newYAML()); err != nil {
		return "", "", err
	} else {
		newValue = strings.TrimSuffix(string(v), "\n")
	}
	return oldValue, newValue, nil
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
