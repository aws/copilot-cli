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
		wanted      func() *Node
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
			wanted: func() *Node {
				/* sentinel -> Mary -> Weight: {new: "kg:52", old: nil} */
				leaf := &Node{
					key:      "Weight",
					newValue: yamlNode("kg: 52", t),
				}
				return &Node{
					children: map[string]*Node{
						"Mary": {
							key: "Mary",
							children: map[string]*Node{
								"Weight": leaf,
							},
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
			wanted: func() *Node {
				/* sentinel -> Mary -> Weight: {new: nil, old: "kg:52"} */
				leaf := &Node{
					key:      "Weight",
					oldValue: yamlNode("kg: 52", t),
				}
				return &Node{
					children: map[string]*Node{
						"Mary": {
							key: "Mary",
							children: map[string]*Node{
								"Weight": leaf,
							},
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
			wanted: func() *Node {
				/* sentinel
				   -> Mary
					   -> Height --> cm: {new: 168, old: 190}
					   -> CanFight: {new: no, old: yes}
					   -> FavoriteWord: {new: peace, old: muscle}
				*/
				leafCM := &Node{
					key:      "cm",
					newValue: yamlScalarNode("168"),
					oldValue: yamlScalarNode("190"),
				}
				leafCanFight := &Node{
					key:      "CanFight",
					newValue: yamlScalarNode("no"),
					oldValue: yamlScalarNode("yes"),
				}
				leafFavWord := &Node{
					key:      "FavoriteWord",
					newValue: yamlScalarNode("peace"),
					oldValue: yamlScalarNode("muscle"),
				}
				return &Node{
					children: map[string]*Node{
						"Mary": {
							key: "Mary",
							children: map[string]*Node{
								"CanFight":     leafCanFight,
								"FavoriteWord": leafFavWord,
								"Height": {
									key: "Height",
									children: map[string]*Node{
										"cm": leafCM,
									},
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
			wanted: func() *Node {
				return nil
			},
		},
		"list reordered": {
			old:  `SizeRank: [bear,dog,cat,mouse]`,
			curr: `SizeRank: [bear,cat,dog,mouse]`,
			wanted: func() *Node {
				/* sentinel
				   -> SizeRank
					   -> {old: dog, new: nil} // Deletion.
					   -> {old: nil, new: dog} // Insertion.
				*/
				leaf1 := &Node{
					oldValue: yamlScalarNode("dog"),
				}
				leaf2 := &Node{
					newValue: yamlScalarNode("dog"),
				}
				return &Node{
					children: map[string]*Node{
						"SizeRank": {
							key: "SizeRank",
							children: map[string]*Node{
								"1,1": leaf1,
								"3,2": leaf2,
							},
						},
					},
				}
			},
		},
		"list with insertion": {
			old:  `DanceCompetition: [dog,bear,cat]`,
			curr: `DanceCompetition: [dog,bear,mouse,cat]`,
			wanted: func() *Node {
				/* sentinel
				   -> DanceCompetition
					   -> {old: nil, new: mouse} // Insertion.
				*/
				leaf := &Node{
					newValue: yamlScalarNode("mouse"),
				}
				return &Node{
					children: map[string]*Node{
						"DanceCompetition": {
							key: "DanceCompetition",
							children: map[string]*Node{
								"2,2": leaf,
							},
						},
					},
				}
			},
		},
		"list with deletion": {
			old:  `PotatoChipCommittee: [dog,bear,cat,mouse]`,
			curr: `PotatoChipCommittee: [dog,bear,mouse]`,
			wanted: func() *Node {
				/* sentinel
				   -> PotatoChipCommittee
					   -> {old: cat, new: nil} // Deletion.
				*/
				leaf := &Node{
					oldValue: yamlScalarNode("cat"),
				}
				return &Node{
					children: map[string]*Node{
						"PotatoChipCommittee": {
							key: "PotatoChipCommittee",
							children: map[string]*Node{
								"2,2": leaf,
							},
						},
					},
				}
			},
		},
		"list with a scalar value changed": {
			old:  `DogsFavoriteShape: [triangle,circle,rectangle]`,
			curr: `DogsFavoriteShape: [triangle,ellipse,rectangle]`,
			wanted: func() *Node {
				/* sentinel
				   -> DogsFavoriteShape
					   -> {old: circle, new: ellipse} // Modification.
				*/
				leaf := &Node{
					oldValue: yamlScalarNode("circle"),
					newValue: yamlScalarNode("ellipse"),
				}
				return &Node{
					children: map[string]*Node{
						"DogsFavoriteShape": {
							key: "DogsFavoriteShape",
							children: map[string]*Node{
								"1,1": leaf,
							},
						},
					},
				}
			},
		},
		"list with a map value changed": {
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
			wanted: func() *Node {
				/* sentinel -> Mary -> Dialogue --> {new: map, old: scalar} */
				leafDialogue := &Node{
					key:      "Dialogue",
					newValue: yamlScalarNode("Said bear: 'I know I'm supposed to keep an eye on you", withStyle(yaml.DoubleQuotedStyle)),
					oldValue: yamlNode("Bear: \"I know I'm supposed to keep an eye on you\"", t),
				}
				return &Node{
					children: map[string]*Node{
						"Mary": {
							key: "Mary",
							children: map[string]*Node{
								"Dialogue": leafDialogue,
							},
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
			wanted: func() *Node {
				/* sentinel -> Mary -> Dialogue --> {new: list, old: scalar} */
				leafDialogue := &Node{
					key:      "Dialogue",
					newValue: yamlScalarNode("Said bear: 'I know I'm supposed to keep an eye on you; Said Dog: 'ikr'", withStyle(yaml.DoubleQuotedStyle)),
					oldValue: yamlNode(`- Bear: "I know I'm supposed to keep an eye on you"
  Tone: disappointed
- Dog: "ikr"
  Tone: pleased`, t),
				}
				return &Node{
					children: map[string]*Node{
						"Mary": {
							key: "Mary",
							children: map[string]*Node{
								"Dialogue": leafDialogue,
							},
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
			wanted: func() *Node {
				/* sentinel -> Mary -> Dialogue --> {new: list, old: map} */
				leafDialogue := &Node{
					key: "Dialogue",
					newValue: yamlNode(`- Bear: "I know I'm supposed to keep an eye on you"
  Tone: disappointed
- Dog: "ikr"
  Tone: pleased`, t),
					oldValue: yamlNode(`Bear: (disappointed) "I know I'm supposed to keep an eye on you"
Dog: (pleased) "ikr"`, t),
				}
				return &Node{
					children: map[string]*Node{
						"Mary": {
							key: "Mary",
							children: map[string]*Node{
								"Dialogue": leafDialogue,
							},
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
			wanted: func() *Node {
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
				require.True(t, equalTree(got, tc.wanted(), t), "should get the expected tree")
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

func equalLeaves(a, b *Node, t *testing.T) bool {
	aNew, err := yaml.Marshal(a.newValue)
	require.NoError(t, err)
	bNew, err := yaml.Marshal(b.newValue)
	require.NoError(t, err)
	aOld, err := yaml.Marshal(a.oldValue)
	require.NoError(t, err)
	bOld, err := yaml.Marshal(b.oldValue)
	require.NoError(t, err)
	return string(aNew) == string(bNew) && string(aOld) == string(bOld)
}

func equalTree(a, b *Node, t *testing.T) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a.key != b.key || len(a.children) != len(b.children) {
		return false
	}
	if len(a.children) == 0 {
		return equalLeaves(a, b, t)
	}
	for k := range a.children {
		if equal := equalTree(a.children[k], b.children[k], t); !equal {
			return false
		}
	}
	return true
}
