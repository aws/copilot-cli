// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"io"
)

const (
	prefixAdd = "+"
	prefixDel = "-"
	prefixMod = "~"
)

const indentInc = 4

// Writer writes the string representation of a diff tree.
type Writer struct {
	tree   Tree
	writer io.Writer
}

// Write uses the writer to write the string representation of the diff tree stemmed from the root.
func (s *Writer) Write() error {
	if s.tree.root == nil {
		_, err := s.writer.Write([]byte("No changes.\n"))
		return err
	}
	for _, child := range s.tree.root.children() {
		if err := s.write(child, 0); err != nil {
			return err
		}
	}
	return nil
}

func (s *Writer) write(node diffNode, indent int) error {
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
	content := process(formatter.formatPath(node), prefixBy(prefixMod), indentBy(indent))
	if _, err := s.writer.Write([]byte(content + "\n")); err != nil {
		return err
	}
	for _, child := range node.children() {
		err := s.write(child, indent+indentInc)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Writer) writeLeaf(node diffNode, indent int, formatter formatter) error {
	switch {
	case node.oldValue() != nil && node.newValue() != nil:
		return s.writeMod(node, indent, formatter)
	case node.oldValue() != nil:
		return s.writeDel(node, indent, formatter)
	default:
		return s.writeInsert(node, indent, formatter)
	}
}

func (s *Writer) writeMod(node diffNode, indent int, formatter formatter) error {
	if node.oldValue().Kind != node.newValue().Kind {
		if err := s.writeDel(node, indent, formatter); err != nil {
			return err
		}
		return s.writeInsert(node, indent, formatter)
	}
	content := processMultiline(formatter.formatMod(node), prefixBy(prefixMod), indentBy(indent))
	_, err := s.writer.Write([]byte(content + "\n"))
	return err
}

func (s *Writer) writeDel(node diffNode, indent int, formatter formatter) error {
	raw, err := formatter.formatYAML(node.oldValue())
	if err != nil {
		return err
	}
	content := processMultiline(string(raw), prefixBy(prefixDel), indentBy(indent))
	_, err = s.writer.Write([]byte(content + "\n"))
	return err
}

func (s *Writer) writeInsert(node diffNode, indent int, formatter formatter) error {
	raw, err := formatter.formatYAML(node.newValue())
	if err != nil {
		return err
	}
	content := processMultiline(string(raw), prefixBy(prefixAdd), indentBy(indent))
	_, err = s.writer.Write([]byte(content + "\n"))
	return err
}
