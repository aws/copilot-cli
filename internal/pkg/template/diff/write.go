// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"io"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

const (
	prefixAdd = "+"
	prefixDel = "-"
	prefixMod = "~"
)

const indentInc = 4

// treeWriter writes the string representation of a diff tree.
type treeWriter struct {
	tree   Tree
	writer io.Writer
}

// write uses the writer to writeTree the string representation of the diff tree stemmed from the root.
func (s *treeWriter) write() error {
	if s.tree.root == nil {
		return nil // Return without writing anything.
	}
	for _, child := range s.tree.root.children() {
		if err := s.writeTree(child, 0); err != nil {
			return err
		}
	}
	return nil
}

func (s *treeWriter) writeTree(node diffNode, indent int) error {
	var formatter formatter
	switch node.(type) {
	// case *unchangedNode:
	// TODO(lou1425926): handle unchanged. 
	case *seqItemNode:
		formatter = &seqItemFormatter{}
	default:
		formatter = &keyedFormatter{node.key()}
	}
	if len(node.children()) == 0 {
		return s.writeLeaf(node, indent, formatter)
	}
	content := process(formatter.formatPath(node), prefixByFn(prefixMod), indentByFn(indent))
	if _, err := s.writer.Write([]byte(content + "\n")); err != nil {
		return err
	}
	for _, child := range node.children() {
		err := s.writeTree(child, indent+indentInc)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *treeWriter) writeLeaf(node diffNode, indent int, formatter formatter) error {
	switch {
	case node.oldYAML() != nil && node.newYAML() != nil:
		return s.writeMod(node, indent, formatter)
	case node.oldYAML() != nil:
		return s.writeDel(node, indent, formatter)
	default:
		return s.writeInsert(node, indent, formatter)
	}
}

func (s *treeWriter) writeMod(node diffNode, indent int, formatter formatter) error {
	if node.oldYAML().Kind != node.newYAML().Kind {
		if err := s.writeDel(node, indent, formatter); err != nil {
			return err
		}
		return s.writeInsert(node, indent, formatter)
	}
	content := processMultiline(formatter.formatMod(node), prefixByFn(prefixMod), indentByFn(indent))
	_, err := s.writer.Write([]byte(color.Yellow.Sprint(content + "\n")))
	return err
}

func (s *treeWriter) writeDel(node diffNode, indent int, formatter formatter) error {
	raw, err := formatter.formatYAML(node.oldYAML())
	if err != nil {
		return err
	}
	content := processMultiline(string(raw), prefixByFn(prefixDel), indentByFn(indent))
	_, err = s.writer.Write([]byte(color.Red.Sprint(content + "\n")))
	return err
}

func (s *treeWriter) writeInsert(node diffNode, indent int, formatter formatter) error {
	raw, err := formatter.formatYAML(node.newYAML())
	if err != nil {
		return err
	}
	content := processMultiline(string(raw), prefixByFn(prefixAdd), indentByFn(indent))
	_, err = s.writer.Write([]byte(color.Green.Sprint(content + "\n")))
	return err
}
