// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFrom_Parse(t *testing.T) {
	testCases := map[string]struct {
		curr        string
		old         string
		wanted      func() diffNode
		wantedError error
	}{
		"add a map": {
			curr: `Mary:
  Height:
    cm: 168
  Weight:
    kg: 52`,
			old: `Mary:
  Height:
    cm: 168`,
			wanted: func() diffNode {
				/* sentinel -> Mary -> Weight: {new: "kg:52", old: nil} */
				leaf := &node{
					keyValue: "Weight",
					newV:     yamlNode("kg: 52", t),
				}
				return &node{
					childNodes: []diffNode{
						&node{
							keyValue:   "Mary",
							childNodes: []diffNode{leaf},
						},
					},
				}
			},
		},
		"remove a map": {
			curr: `Mary:
  Height:
    cm: 168`,
			old: `Mary:
  Height:
    cm: 168
  Weight:
    kg: 52`,
			wanted: func() diffNode {
				/* sentinel -> Mary -> Weight: {new: nil, old: "kg:52"} */
				leaf := &node{
					keyValue: "Weight",
					oldV:     yamlNode("kg: 52", t),
				}
				return &node{
					childNodes: []diffNode{
						&node{
							keyValue:   "Mary",
							childNodes: []diffNode{leaf},
						},
					},
				}
			},
		},
		"change keyed values": {
			curr: `Mary:
  Height:
    cm: 168
  CanFight: no
  FavoriteWord: peace`,
			old: `Mary:
  Height:
    cm: 190
  CanFight: yes
  FavoriteWord: muscle`,
			wanted: func() diffNode {
				/* sentinel
				   -> Mary
					   -> Height --> cm: {new: 168, old: 190}
					   -> CanFight: {new: no, old: yes}
					   -> FavoriteWord: {new: peace, old: muscle}
				*/
				leafCM := &node{
					keyValue: "cm",
					newV:     yamlScalarNode("168"),
					oldV:     yamlScalarNode("190"),
				}
				leafCanFight := &node{
					keyValue: "CanFight",
					newV:     yamlScalarNode("no"),
					oldV:     yamlScalarNode("yes"),
				}
				leafFavWord := &node{
					keyValue: "FavoriteWord",
					newV:     yamlScalarNode("peace"),
					oldV:     yamlScalarNode("muscle"),
				}
				return &node{
					childNodes: []diffNode{
						&node{
							keyValue: "Mary",
							childNodes: []diffNode{
								leafCanFight,
								leafFavWord,
								&node{
									keyValue:   "Height",
									childNodes: []diffNode{leafCM},
								},
							},
						},
					},
				}
			},
		},
		"list does not change": {
			old:  `Alphabet: [a,b,c,d]`,
			curr: `Alphabet: [a,b,c,d]`,
			wanted: func() diffNode {
				return nil
			},
		},
		"list reordered": {
			old:  `SizeRank: [bear,dog,cat,mouse]`,
			curr: `SizeRank: [bear,cat,dog,mouse]`,
			wanted: func() diffNode {
				/* sentinel
				   -> SizeRank
					   -> {old: dog, new: nil} // Deletion.
					   -> {old: nil, new: dog} // Insertion.
				*/
				leaf1 := &node{
					oldV: yamlScalarNode("dog"),
				}
				leaf2 := &node{
					newV: yamlScalarNode("dog"),
				}
				return &node{
					childNodes: []diffNode{
						&node{
							keyValue:   "SizeRank",
							childNodes: []diffNode{leaf1, leaf2},
						},
					},
				}
			},
		},
		"list with insertion": {
			old:  `DanceCompetition: [dog,bear,cat]`,
			curr: `DanceCompetition: [dog,bear,mouse,cat]`,
			wanted: func() diffNode {
				/* sentinel
				   -> DanceCompetition
					   -> {old: nil, new: mouse} // Insertion.
				*/
				leaf := &node{
					newV: yamlScalarNode("mouse"),
				}
				return &node{
					childNodes: []diffNode{
						&node{
							keyValue:   "DanceCompetition",
							childNodes: []diffNode{leaf},
						},
					},
				}
			},
		},
		"list with deletion": {
			old:  `PotatoChipCommittee: [dog,bear,cat,mouse]`,
			curr: `PotatoChipCommittee: [dog,bear,mouse]`,
			wanted: func() diffNode {
				/* sentinel
				   -> PotatoChipCommittee
					   -> {old: cat, new: nil} // Deletion.
				*/
				leaf := &node{
					oldV: yamlScalarNode("cat"),
				}
				return &node{
					childNodes: []diffNode{
						&node{
							keyValue:   "PotatoChipCommittee",
							childNodes: []diffNode{leaf},
						},
					},
				}
			},
		},
		"list with a scalar value changed": {
			old:  `DogsFavoriteShape: [triangle,circle,rectangle]`,
			curr: `DogsFavoriteShape: [triangle,ellipse,rectangle]`,
			wanted: func() diffNode {
				/* sentinel
				   -> DogsFavoriteShape
					   -> {old: circle, new: ellipse} // Modification.
				*/
				leaf := &node{
					oldV: yamlScalarNode("circle"),
					newV: yamlScalarNode("ellipse"),
				}
				return &node{
					childNodes: []diffNode{
						&node{
							keyValue:   "DogsFavoriteShape",
							childNodes: []diffNode{leaf},
						},
					},
				}
			},
		},
		"list with a map value changed": { // TODO(lou1415926): handle list of maps modification
			old: `StrawberryPopularitySurvey:
- Name: Dog
  LikeStrawberry: ver much
- Name: Bear
  LikeStrawberry: meh
- Name: Cat
  LikeStrawberry: ew`,
			curr: `StrawberryPopularitySurvey:
- Name: Dog
  LikeStrawberry: ver much
- Name: Bear
  LikeStrawberry: ok
- Name: Cat
  LikeStrawberry: ew`,
		},
		"change a map to scalar": {
			curr: `Mary:
  Dialogue: "Said bear: 'I know I'm supposed to keep an eye on you"`,
			old: `Mary:
  Dialogue:
    Bear: "I know I'm supposed to keep an eye on you"`,
			wanted: func() diffNode {
				/* sentinel -> Mary -> Dialogue --> {new: map, old: scalar} */
				leafDialogue := &node{
					keyValue: "Dialogue",
					newV:     yamlScalarNode("Said bear: 'I know I'm supposed to keep an eye on you", withStyle(yaml.DoubleQuotedStyle)),
					oldV:     yamlNode("Bear: \"I know I'm supposed to keep an eye on you\"", t),
				}
				return &node{
					childNodes: []diffNode{
						&node{
							keyValue:   "Mary",
							childNodes: []diffNode{leafDialogue},
						},
					},
				}
			},
		},
		"change a list to scalar": {
			curr: `Mary:
  Dialogue: "Said bear: 'I know I'm supposed to keep an eye on you; Said Dog: 'ikr'"`,
			old: `Mary:
  Dialogue:
    - Bear: "I know I'm supposed to keep an eye on you"
      Tone: disappointed
    - Dog: "ikr"
      Tone: pleased`,
			wanted: func() diffNode {
				/* sentinel -> Mary -> Dialogue --> {new: list, old: scalar} */
				leafDialogue := &node{
					keyValue: "Dialogue",
					newV:     yamlScalarNode("Said bear: 'I know I'm supposed to keep an eye on you; Said Dog: 'ikr'", withStyle(yaml.DoubleQuotedStyle)),
					oldV: yamlNode(`- Bear: "I know I'm supposed to keep an eye on you"
  Tone: disappointed
- Dog: "ikr"
  Tone: pleased`, t),
				}
				return &node{
					childNodes: []diffNode{
						&node{
							keyValue:   "Mary",
							childNodes: []diffNode{leafDialogue},
						},
					},
				}
			},
		},
		"change a map to list": {
			curr: `Mary:
  Dialogue:
    - Bear: "I know I'm supposed to keep an eye on you"
      Tone: disappointed
    - Dog: "ikr"
      Tone: pleased`,
			old: `Mary:
  Dialogue:
    Bear: (disappointed) "I know I'm supposed to keep an eye on you"
    Dog: (pleased) "ikr"`,
			wanted: func() diffNode {
				/* sentinel -> Mary -> Dialogue --> {new: list, old: map} */
				leafDialogue := &node{
					keyValue: "Dialogue",
					newV: yamlNode(`- Bear: "I know I'm supposed to keep an eye on you"
  Tone: disappointed
- Dog: "ikr"
  Tone: pleased`, t),
					oldV: yamlNode(`Bear: (disappointed) "I know I'm supposed to keep an eye on you"
Dog: (pleased) "ikr"`, t),
				}
				return &node{
					childNodes: []diffNode{
						&node{
							keyValue:   "Mary",
							childNodes: []diffNode{leafDialogue},
						},
					},
				}
			},
		},
		"no diff": {
			curr: `Mary:
  Height:
    cm: 190
  CanFight: yes
  FavoriteWord: muscle`,
			old: `Mary:
  Height:
    cm: 190
  CanFight: yes
  FavoriteWord: muscle`,
			wanted: func() diffNode {
				return nil
			},
		},
		"error unmarshalling": {
			curr:        `	!!1?Mary:`,
			wantedError: errors.New("unmarshal current template: yaml: found character that cannot start any token"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := From(tc.old).Parse([]byte(tc.curr))
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			}
			if tc.wanted != nil {
				require.NoError(t, err)
				require.True(t, equalTree(got, Tree{tc.wanted()}, t), "should get the expected tree")
			}
		})
	}
}

type nodeModifier func(node *yaml.Node)

func withStyle(style yaml.Style) nodeModifier {
	return func(node *yaml.Node) {
		node.Style = style
	}
}

func yamlNode(content string, t *testing.T) *yaml.Node {
	var node yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(content), &node), "should be able to unmarshal the wanted content")
	// The root YAML node is a document node. We want the first content node.
	return node.Content[0]
}

func yamlScalarNode(value string, opts ...nodeModifier) *yaml.Node {
	node := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: value,
	}
	for _, opt := range opts {
		opt(node)
	}
	return node
}

func equalTree(a, b Tree, t *testing.T) bool {
	if a.root == nil || b.root == nil {
		return a.root == nil && b.root == nil
	}
	return equalSubTree(a.root, b.root, t)
}

func equalSubTree(a, b diffNode, t *testing.T) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a.key() != b.key() || len(a.children()) != len(b.children()) {
		return false
	}
	if len(a.children()) == 0 {
		return equalLeaves(a, b, t)
	}
	for idx, _ := range a.children() {
		if equal := equalSubTree(a.children()[idx], b.children()[idx], t); !equal {
			return false
		}
	}
	return true
}

func equalLeaves(a, b diffNode, t *testing.T) bool {
	aNew, err := yaml.Marshal(a.newYAML())
	require.NoError(t, err)
	bNew, err := yaml.Marshal(b.newYAML())
	require.NoError(t, err)
	aOld, err := yaml.Marshal(a.oldYAML())
	require.NoError(t, err)
	bOld, err := yaml.Marshal(b.oldYAML())
	require.NoError(t, err)
	return string(aNew) == string(bNew) && string(aOld) == string(bOld)
}
