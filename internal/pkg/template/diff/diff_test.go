// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"errors"
	"reflect"
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
				leaf := &keyNode{
					keyValue: "Weight",
					newV:     yamlNode("kg: 52", t),
				}
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
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
				leaf := &keyNode{
					keyValue: "Weight",
					oldV:     yamlNode("kg: 52", t),
				}
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
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
				leafCM := &keyNode{
					keyValue: "cm",
					newV:     yamlScalarNode("168"),
					oldV:     yamlScalarNode("190"),
				}
				leafCanFight := &keyNode{
					keyValue: "CanFight",
					newV:     yamlScalarNode("no"),
					oldV:     yamlScalarNode("yes"),
				}
				leafFavWord := &keyNode{
					keyValue: "FavoriteWord",
					newV:     yamlScalarNode("peace"),
					oldV:     yamlScalarNode("muscle"),
				}
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
							keyValue: "Mary",
							childNodes: []diffNode{
								leafCanFight,
								leafFavWord,
								&keyNode{
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
				          -> 1 unchanged item (bear)
					   -> {old: dog, new: nil} // Deletion.
				          -> 1 unchanged item (cat)
					   -> {old: nil, new: dog} // Insertion.
				          -> 1 unchanged item (mouse)
				*/
				leaf1 := &seqItemNode{
					keyNode{oldV: yamlScalarNode("dog")},
				}
				leaf2 := &seqItemNode{
					keyNode{newV: yamlScalarNode("dog")},
				}
				unchangedBear, unchangedCat, unchangedMouse := &unchangedNode{count: 1}, &unchangedNode{count: 1}, &unchangedNode{count: 1}
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
							keyValue:   "SizeRank",
							childNodes: []diffNode{unchangedBear, leaf1, unchangedCat, leaf2, unchangedMouse},
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
				          -> 2 unchanged items (dog, bear)
					   -> {old: nil, new: mouse} // Insertion.
				          -> 1 unchanged item (cat)
				*/
				leaf := &seqItemNode{
					keyNode{newV: yamlScalarNode("mouse")},
				}
				unchangedDogBear, unchangedCat := &unchangedNode{count: 2}, &unchangedNode{count: 1}
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
							keyValue:   "DanceCompetition",
							childNodes: []diffNode{unchangedDogBear, leaf, unchangedCat},
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
					   -> 2 unchanged items (dog, bear)
					   -> {old: cat, new: nil} // Deletion.
					   -> 1 unchanged item (mouse)
				*/
				leaf := &seqItemNode{
					keyNode{oldV: yamlScalarNode("cat")},
				}
				unchangedDobBear, unchangedMouse := &unchangedNode{count: 2}, &unchangedNode{count: 1}
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
							keyValue:   "PotatoChipCommittee",
							childNodes: []diffNode{unchangedDobBear, leaf, unchangedMouse},
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
				leaf := &seqItemNode{
					keyNode{
						oldV: yamlScalarNode("circle"),
						newV: yamlScalarNode("ellipse"),
					},
				}
				unchangedTri, unchangedRec := &unchangedNode{1}, &unchangedNode{1}
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
							keyValue:   "DogsFavoriteShape",
							childNodes: []diffNode{unchangedTri, leaf, unchangedRec},
						},
					},
				}
			},
		},
		"change a map to scalar": {
			curr: `Mary:
  Dialogue: "Said bear: 'I know I'm supposed to keep an eye on you"`,
			old: `Mary:
  Dialogue:
    Bear: "I know I'm supposed to keep an eye on you"`,
			wanted: func() diffNode {
				/* sentinel -> Mary -> Dialogue --> {new: map, old: scalar} */
				leafDialogue := &keyNode{
					keyValue: "Dialogue",
					newV:     yamlScalarNode("Said bear: 'I know I'm supposed to keep an eye on you", withStyle(yaml.DoubleQuotedStyle)),
					oldV:     yamlNode("Bear: \"I know I'm supposed to keep an eye on you\"", t),
				}
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
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
				leafDialogue := &keyNode{
					keyValue: "Dialogue",
					newV:     yamlScalarNode("Said bear: 'I know I'm supposed to keep an eye on you; Said Dog: 'ikr'", withStyle(yaml.DoubleQuotedStyle)),
					oldV: yamlNode(`- Bear: "I know I'm supposed to keep an eye on you"
  Tone: disappointed
- Dog: "ikr"
  Tone: pleased`, t),
				}
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
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
				leafDialogue := &keyNode{
					keyValue: "Dialogue",
					newV: yamlNode(`- Bear: "I know I'm supposed to keep an eye on you"
  Tone: disappointed
- Dog: "ikr"
  Tone: pleased`, t),
					oldV: yamlNode(`Bear: (disappointed) "I know I'm supposed to keep an eye on you"
Dog: (pleased) "ikr"`, t),
				}
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
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
			} else {
				require.NoError(t, err)
				require.True(t, equalTree(got, Tree{tc.wanted()}, t), "should get the expected tree")
			}
		})
	}
}

func TestFrom_ParseWithCFNOverriders(t *testing.T) {
	testCases := map[string]struct {
		curr        string
		old         string
		wanted      func() diffNode
		wantedError error
	}{
		"diff in metadata manifest is ignored": {
			old: `Description: CloudFormation environment template for infrastructure shared among Copilot workloads.
Metadata:
  Version: v1.26.0
  Manifest: I don't see any difference.`,
			curr: `Description: CloudFormation environment template for infrastructure shared among Copilot workloads.
Metadata:
  Version: v1.27.0
  Manifest: There is definitely a difference.`,
			wanted: func() diffNode {
				/* sentinel -> Metadata -> Version*/
				leaf := &keyNode{
					keyValue: "Version",
					oldV:     yamlScalarNode("v1.26.0"),
					newV:     yamlScalarNode("v1.27.0"),
				}
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
							keyValue:   "Metadata",
							childNodes: []diffNode{leaf},
						},
					},
				}
			},
		},
		"no diff between full/short form intrinsic func": {
			curr: `Value: !Sub 'blah'
AvailabilityZone: !Select [0, !GetAZs '']
SecurityGroups:
  - !GetAtt InternalLoadBalancerSecurityGroup.GroupId
StringsEquals:
  iam:ResourceTag/copilot-application: !Sub '${AppName}'
Properties:
  GroupDescription: !Join ['', [!Ref AppName, '-', !Ref EnvironmentName, EnvironmentSecurityGroup]]
`,
			old: `Value:
  Fn::Sub: 'blah'
AvailabilityZone:
  Fn::Select:
    - 0
    - Fn::GetAZs: ""
SecurityGroups:
  - Fn::GetAtt: InternalLoadBalancerSecurityGroup.GroupId
StringsEquals:
  iam:ResourceTag/copilot-application:
    Fn::Sub: ${AppName}
Properties:
  GroupDescription:
     Fn::Join:
        - ""
        - - Ref: AppName
          - "-"
          - Ref: EnvironmentName
          - EnvironmentSecurityGroup
`,
			wanted: func() diffNode {
				return nil
			},
		},
		"no diff": {
			old: `Description: CloudFormation environment template for infrastructure shared among Copilot workloads.
Metadata:
  Manifest: I don't see any difference.`,
			curr: `Description: CloudFormation environment template for infrastructure shared among Copilot workloads.
Metadata:
  Manifest: There is definitely a difference.`,
			wanted: func() diffNode {
				return nil
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := From(tc.old).ParseWithCFNOverriders([]byte(tc.curr))
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
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
	if a.key() != b.key() || len(a.children()) != len(b.children()) || reflect.TypeOf(a) != reflect.TypeOf(b) {
		return false
	}
	if len(a.children()) == 0 {
		return equalLeaves(a, b, t)
	}
	for idx := range a.children() {
		if equal := equalSubTree(a.children()[idx], b.children()[idx], t); !equal {
			return false
		}
	}
	return true
}

func equalLeaves(a, b diffNode, t *testing.T) bool {
	if _, ok := a.(*unchangedNode); ok {
		return a.(*unchangedNode).unchangedCount() == b.(*unchangedNode).unchangedCount()
	}
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
