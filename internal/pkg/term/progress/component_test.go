// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoopComponent_Render(t *testing.T) {
	// GIVEN
	buf := new(strings.Builder)
	c := &noopComponent{}

	// WHEN
	nl, err := c.Render(buf)

	// THEN
	require.Equal(t, 0, nl, "expected no lines to be written")
	require.NoError(t, err, "expected err to be nil")
	require.Equal(t, "", buf.String(), "expected the content to be empty")
}

func TestSingleLineComponent_Render(t *testing.T) {
	testCases := map[string]struct {
		inText    string
		inPadding int

		wantedOut string
	}{
		"should print padded text with new line": {
			inText:    "hello world",
			inPadding: 4,

			wantedOut: "    hello world\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			comp := &singleLineComponent{
				Text:    tc.inText,
				Padding: tc.inPadding,
			}
			buf := new(strings.Builder)

			// WHEN
			nl, err := comp.Render(buf)

			// THEN
			require.NoError(t, err)
			require.Equal(t, 1, nl, "expected only a single line to be written by a single line component")
			require.Equal(t, tc.wantedOut, buf.String())
		})
	}
}

func TestTreeComponent_Render(t *testing.T) {
	testCases := map[string]struct {
		inNode     Renderer
		inChildren []Renderer

		wantedNumLines int
		wantedOut      string
	}{
		"should render all the nodes": {
			inNode: &singleLineComponent{
				Text: "is",
			},
			inChildren: []Renderer{
				&singleLineComponent{
					Text: "this",
				},
				&singleLineComponent{
					Text: "working?",
				},
			},

			wantedNumLines: 3,
			wantedOut: `is
this
working?
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			comp := &treeComponent{
				Root:     tc.inNode,
				Children: tc.inChildren,
			}
			buf := new(strings.Builder)

			// WHEN
			nl, err := comp.Render(buf)

			// THEN
			require.NoError(t, err)
			require.Equal(t, tc.wantedNumLines, nl)
			require.Equal(t, tc.wantedOut, buf.String())
		})
	}
}

func TestDynamicTreeComponent_Render(t *testing.T) {
	// GIVEN
	comp := dynamicTreeComponent{
		Root: &mockDynamicRenderer{
			content: "hello",
		},
		Children: []Renderer{
			&mockDynamicRenderer{
				content: " world",
			},
		},
	}
	buf := new(strings.Builder)

	// WHEN
	_, err := comp.Render(buf)

	// THEN
	require.NoError(t, err)
	require.Equal(t, "hello world", buf.String())
}

func TestDynamicTreeComponent_Done(t *testing.T) {
	// GIVEN
	root := &mockDynamicRenderer{
		done: make(chan struct{}),
	}
	child := &mockDynamicRenderer{
		done: make(chan struct{}),
	}
	comp := dynamicTreeComponent{
		Root:     root,
		Children: []Renderer{child, &noopComponent{}},
	}

	// WHEN
	go func() {
		// Close all nodes in the tree.
		close(child.done)
		close(root.done)
	}()

	// THEN
	<-comp.Done() // Should successfully exit instead of hanging.
}

func TestTableComponent_Render(t *testing.T) {
	testCases := map[string]struct {
		inTitle   string
		inHeader  []string
		inRows    [][]string
		inPadding int

		wantedNumLines int
		wantedOut      string
	}{
		"should not write anything if there are no rows": {
			inTitle:  "Fancy table",
			inHeader: []string{"col1", "col2"},

			wantedNumLines: 0,
			wantedOut:      "",
		},
		"should render a sample table": {
			inTitle:  "Deployments",
			inHeader: []string{"", "Revision", "Rollout", "Desired", "Running", "Failed", "Pending"},
			inRows: [][]string{
				{"PRIMARY", "3", "[in progress]", "10", "0", "0", "10"},
				{"ACTIVE", "2", "[completed]", "10", "10", "0", "0"},
			},

			wantedNumLines: 4,
			wantedOut: `Deployments
           Revision  Rollout        Desired  Running  Failed  Pending
  PRIMARY  3         [in progress]  10       0        0       10
  ACTIVE   2         [completed]    10       10       0       0
`,
		},
		"should render a sample table with with padding": {
			inTitle:  "Person",
			inHeader: []string{"First", "Last"},
			inRows: [][]string{
				{"Cookie", "Monster"},
			},
			inPadding: 3,

			wantedNumLines: 3,
			wantedOut: `   Person
     First   Last
     Cookie  Monster
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			buf := new(strings.Builder)
			table := newTableComponent(tc.inTitle, tc.inHeader, tc.inRows)
			table.Padding = tc.inPadding

			// WHEN
			numLines, err := table.Render(buf)

			// THEN
			require.NoError(t, err)
			require.Equal(t, tc.wantedNumLines, numLines, "expected number of lines to match")
			require.Equal(t, tc.wantedOut, buf.String(), "expected table content to match")
		})
	}
}
