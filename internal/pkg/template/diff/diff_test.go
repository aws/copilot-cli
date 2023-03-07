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
		"add a list": {
			curr: `Mary:
  Pulitzer: true
  Works:
    - Dog Songs
    - Others`,
			old: `Mary:
  Pulitzer: true`,
			wanted: func() *Node {
				/* sentinel -> Mary -> Works: {new: seq, old: nil} */
				leaf := &Node{
					key:      "Works",
					newValue: yamlNode("- Dog Songs\n- Others", t),
				}
				return &Node{
					children: map[string]*Node{
						"Mary": {
							key: "Mary",
							children: map[string]*Node{
								"Works": leaf,
							},
						},
					},
				}
			},
		},
		"remove a list": {
			curr: `Mary:
  Pulitzer: true`,
			old: `Mary:
  Pulitzer: true
  Works:
    - Dog Songs
    - Others`,
			wanted: func() *Node {
				/* sentinel -> Mary -> Works: {new: nil, old: seq} */
				leaf := &Node{
					key:      "Works",
					oldValue: yamlNode("- Dog Songs\n- Others", t),
				}
				return &Node{
					children: map[string]*Node{
						"Mary": {
							key: "Mary",
							children: map[string]*Node{
								"Works": leaf,
							},
						},
					},
				}
			},
		},
		"add a scalar item to a list": {
			curr: `Mary:
- <Dog Songs>
- <Kingdom of Dogs>`,
			old: `Mary:
- <Dog Songs>`,
			wanted: func() *Node {
				/* sentinel -> Mary -> {new: "<Kingdom of Dogs>", old: nil} */
				leaf := &Node{
					key:      "0",
					newValue: yamlScalarNode("<Kingdom of Dogs>"),
				}
				return &Node{
					children: map[string]*Node{
						"Mary": {
							key: "Mary",
							children: map[string]*Node{
								"0": leaf,
							},
						},
					},
				}
			},
		},
		"remove a scalar item from a list": {
			curr: `Mary:
- <Dog Songs>`,
			old: `Mary:
- <Dog Songs>
- <Kingdom of Dogs>`,
			wanted: func() *Node {
				/* sentinel -> Mary -> {new: nil, old: "<Kingdom of Dogs>"} */
				leaf := &Node{
					key:      "0",
					oldValue: yamlScalarNode("<Kingdom of Dogs>"),
				}
				return &Node{
					children: map[string]*Node{
						"Mary": {
							key: "Mary",
							children: map[string]*Node{
								"0": leaf,
							},
						},
					},
				}
			},
		},
		"add repeated items to a list": {
			curr: `Mary:
- <Dog Songs>
- <Dog Songs>
- <Dog Songs>`,
			old: `Mary:
- <Dog Songs>`,
			wanted: func() *Node {
				/* sentinel
				-> Mary
					-> {new: "<Dog Songs>", old: nil}
					-> {new: "<Dog Songs>", old: nil}
				*/
				leaf0 := &Node{
					key:      "0",
					newValue: yamlScalarNode("<Dog Songs>"),
				}
				leaf1 := &Node{
					key:      "1",
					newValue: yamlScalarNode("<Dog Songs>"),
				}
				return &Node{
					children: map[string]*Node{
						"Mary": {
							key: "Mary",
							children: map[string]*Node{
								"0": leaf0,
								"1": leaf1,
							},
						},
					},
				}
			},
		},
		"remove repeated items to a list": {
			curr: `Mary:
- <Dog Songs>`,
			old: `Mary:
- <Dog Songs>
- <Dog Songs>
- <Dog Songs>`,
			wanted: func() *Node {
				/* sentinel -> Mary -> {new: nil, old: "<Dog Songs>"} */
				leaf0 := &Node{
					key:      "0",
					oldValue: yamlScalarNode("<Dog Songs>"),
				}
				leaf1 := &Node{
					key:      "1",
					oldValue: yamlScalarNode("<Dog Songs>"),
				}
				return &Node{
					children: map[string]*Node{
						"Mary": {
							key: "Mary",
							children: map[string]*Node{
								"0": leaf0,
								"1": leaf1,
							},
						},
					},
				}
			},
		},
		"add a map item to a list": {
			curr: `Mary:
- <Dog Songs>
- Name: Kingdom of Dogs
  Type: Book`,
			old: `Mary:
- <Dog Songs>`,
			wanted: func() *Node {
				/* sentinel -> Mary -> {new: map, old: nil} */
				leaf := &Node{
					key:      "0",
					newValue: yamlNode("Name: Kingdom of Dogs\nType: Book", t),
				}
				return &Node{
					children: map[string]*Node{
						"Mary": {
							key: "Mary",
							children: map[string]*Node{
								"0": leaf,
							},
						},
					},
				}
			},
		},
		"remove a map item from a list": {
			curr: `Mary:
- <Dog Songs>`,
			old: `Mary:
- <Dog Songs>
- Name: Kingdom of Dogs
  Type: Book`,
			wanted: func() *Node {
				/* sentinel -> Mary -> {new: nil, old: map} */
				leaf := &Node{
					key:      "0",
					oldValue: yamlNode("Name: Kingdom of Dogs\nType: Book", t),
				}
				return &Node{
					children: map[string]*Node{
						"Mary": {
							key: "Mary",
							children: map[string]*Node{
								"0": leaf,
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
		"change a list item value": {},
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
				require.True(t, equalTree(got, tc.wanted(), t))
			}
		})
	}
}

func yamlNode(content string, t *testing.T) *yaml.Node {
	var node yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(content), &node), "should be able to unmarshal the wanted content")
	// The root YAML node is a document node. We want the first content node.
	return node.Content[0]
}

func yamlScalarNode(value string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: value,
	}
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
	if a == nil && b == nil {
		return true
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
