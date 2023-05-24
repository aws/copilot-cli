// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"fmt"
	"io"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/dustin/go-humanize/english"
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
	if len(s.tree.root.children()) == 0 {
		return s.writeLeaf(s.tree.root, &documentFormatter{})
	}
	for _, child := range s.tree.root.children() {
		if err := s.writeTree(child, 0); err != nil {
			return err
		}
	}
	return nil
}

func (s *treeWriter) writeTree(node diffNode, indent int) error {
	if node == nil {
		return nil
	}
	var formatter formatter
	switch node := node.(type) {
	case *unchangedNode:
		content := fmt.Sprintf("(%s)", english.Plural(node.unchangedCount(), "unchanged item", "unchanged items"))
		content = process(content, indentByFn(indent))
		_, err := s.writer.Write([]byte(color.Faint.Sprint(content + "\n")))
		return err
	case *seqItemNode:
		formatter = &seqItemFormatter{indent}
	default:
		formatter = &keyedFormatter{indent}
	}
	if len(node.children()) == 0 {
		return s.writeLeaf(node, formatter)
	}
	if kn, ok := node.(*keyNode); ok { // Collapse all key nodes with exactly one diff.
		node = joinNodes(kn)
	}
	if _, err := s.writer.Write([]byte(formatter.formatPath(node))); err != nil {
		return err
	}
	for _, child := range node.children() {
		err := s.writeTree(child, formatter.nextIndent())
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *treeWriter) writeLeaf(node diffNode, formatter formatter) error {
	switch {
	case node.oldYAML() != nil && node.newYAML() != nil:
		return s.writeMod(node, formatter)
	case node.oldYAML() != nil:
		return s.writeDel(node, formatter)
	default:
		return s.writeInsert(node, formatter)
	}
}

func (s *treeWriter) writeMod(node diffNode, formatter formatter) error {
	if node.oldYAML().Kind != node.newYAML().Kind {
		if err := s.writeDel(node, formatter); err != nil {
			return err
		}
		return s.writeInsert(node, formatter)
	}
	content, err := formatter.formatMod(node)
	if err != nil {
		return err
	}
	_, err = s.writer.Write([]byte(color.Yellow.Sprint(content + "\n")))
	return err
}

func (s *treeWriter) writeDel(node diffNode, formatter formatter) error {
	content, err := formatter.formatDel(node)
	if err != nil {
		return err
	}
	_, err = s.writer.Write([]byte(color.Red.Sprint(content + "\n")))
	return err
}

func (s *treeWriter) writeInsert(node diffNode, formatter formatter) error {
	content, err := formatter.formatInsert(node)
	if err != nil {
		return err
	}
	_, err = s.writer.Write([]byte(color.Green.Sprint(content + "\n")))
	return err
}

// joinNodes collapses all keyNode on a Tree path into one keyNode, as long as there is only modification under the key.
// For example, if only the `DesiredCount` of an ECS service is changed, then the returned path becomes
// `/Resources/Service/Properties`. If multiple entries of an ECS service is changed, then the returned
// path is `/Resources/Service`.
func joinNodes(curr *keyNode) *keyNode {
	key := curr.key()
	for {
		if len(curr.children()) != 1 {
			break
		}
		peek := curr.children()[0]
		if len(peek.children()) == 0 {
			break
		}
		if _, ok := peek.(*keyNode); !ok {
			break
		}
		key = key + "/" + peek.key()
		curr = peek.(*keyNode)
	}
	return &keyNode{
		keyValue:   key,
		childNodes: curr.children(),
	}
}
