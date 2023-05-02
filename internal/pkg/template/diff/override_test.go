// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestIntrinsicFuncFullShortFormConverter(t *testing.T) {
	testCases := map[string]struct {
		curr   string
		old    string
		wanted func() diffNode
	}{
		"no diff in Ref vs !Ref": {
			old: `Value: !Sub 'blah'`,
			curr: `Value:
  Fn::Sub: 'blah'`,
		},
		"detect diff in Ref and !Ref": {
			old: `
hummingbird:
  Ref: pineapple1`,
			curr: `hummingbird: !Ref pineapple2`,
			wanted: func() diffNode {
				leaf := &node{
					keyValue: "Ref",
					oldV:     yamlScalarNode("pineapple1"),
					newV:     yamlScalarNode("pineapple2"),
				}
				return &node{
					childNodes: []diffNode{&node{
						keyValue:   "hummingbird",
						childNodes: []diffNode{leaf},
					}},
				}
			},
		},
		"no diff in Fn::Base64 vs !Base64": {
			old: `Value:
  Fn::Base64: "AWS CloudFormation"`,
			curr: `Value: !Base64 AWS CloudFormation`,
		},
		"detect diff in Fn::Base64 amd !Base64": {
			old: `Basement:
  Fn::Base64: 1`,
			curr: `Basement: !Base64 2`,
			wanted: func() diffNode {
				leaf := &node{
					keyValue: "Fn::Base64",
					oldV:     yamlScalarNode("1"),
					newV:     yamlScalarNode("2"),
				}
				return &node{
					childNodes: []diffNode{&node{
						keyValue:   "Basement",
						childNodes: []diffNode{leaf},
					}},
				}
			},
		},
		"no diff in Fn::Cidr vs !Cidr": {
			old: `CidrBlock:
  Fn::Cidr:
    - 192.168.0.0/24
    - 6
    - 5`,
			curr: `CidrBlock: !Cidr ["192.168.0.0/24", 6, 5 ]`,
		},
		"detect diff in Fn::Cidr vs !Cidr": {
			old: `cedar: !Cidr ["192.168.0.0/16", 6, 4 ]`,
			curr: `cedar:
  Fn::Cidr:
    - 192.168.0.0/24
    - 6
    - 5`,
			wanted: func() diffNode {
				changedCIDR := &seqItemNode{
					node: node{
						oldV: yamlScalarNode("192.168.0.0/16", withStyle(yaml.DoubleQuotedStyle)),
						newV: yamlScalarNode("192.168.0.0/24"),
					},
				}
				unchanged := &unchangedNode{count: 1}
				changedNum := &seqItemNode{
					node: node{
						oldV: yamlScalarNode("4"),
						newV: yamlScalarNode("5"),
					},
				}
				return &node{
					childNodes: []diffNode{&node{
						keyValue: "cedar",
						childNodes: []diffNode{&node{
							keyValue:   "Fn::Cidr",
							childNodes: []diffNode{changedCIDR, unchanged, changedNum},
						}},
					}},
				}
			},
		},
		"no diff in Fn::FindInMap vs !FindInMap": {
			old: `
ImageId:
  Fn::FindInMap:
   - RegionMap
   - !Ref 'AWS::Region'
   - HVM64`,
			curr: `
ImageId: !FindInMap
 - RegionMap
 - !Ref 'AWS::Region'
 - HVM64`,
		},
		"detect diff in Fn::FindInMap vs !FindInMap": {
			old: `
ImageId:
  Fn::FindInMap:
   - RegionMap
   - !Ref 'AWS::Region'
   - HVM64`,
			curr: `
ImageId: !FindInMap
 - Chizu
 - !Ref 'AWS::Region'
 - HVM64`,
			wanted: func() diffNode {
				changedMapName := &seqItemNode{
					node: node{
						oldV: yamlScalarNode("RegionMap"),
						newV: yamlScalarNode("Chizu"),
					},
				}
				unchanged := &unchangedNode{count: 2}
				leaf := &node{
					keyValue: "ImageId",
					childNodes: []diffNode{&node{
						keyValue:   "Fn::FindInMap",
						childNodes: []diffNode{changedMapName, unchanged},
					}},
				}
				return &node{
					childNodes: []diffNode{leaf},
				}
			},
		},
		// 	"no diff in Fn::GetAtt vs !GetAtt": { // TODO(lou1415926)
		// 		old: `SourceSecurityGroupOwnerId:
		// Fn::GetAtt:
		//   - myELB
		//   - SourceSecurityGroup.OwnerAlias`,
		// 		curr: `SourceSecurityGroupOwnerId: !GetAtt myELB.SourceSecurityGroup.OwnerAlias`,
		// 	},
		"no diff in Fn::GetAZs vs !GetAZ": {
			old: `AvailabilityZone: !GetAZs ""`,
			curr: `AvailabilityZone:
  Fn::GetAZs: ""`,
		},
		"detect diff in Fn::GetAZs vs !GetAZ": {
			old: `AvailabilityZone:
  Fn::Select:
    - 0
    - Fn::GetAZs: "amazon"`,
			curr: `AvailabilityZone: !Select [0, !GetAZs 'arizona']`,
			wanted: func() diffNode {
				unchanged := &unchangedNode{count: 1}
				changedAZName := &seqItemNode{
					node{
						childNodes: []diffNode{
							&node{
								keyValue: "Fn::GetAZs",
								oldV:     yamlScalarNode("amazon", withStyle(yaml.DoubleQuotedStyle)),
								newV:     yamlScalarNode("arizona", withStyle(yaml.SingleQuotedStyle)),
							},
						},
					},
				}
				leaf := &node{
					keyValue: "AvailabilityZone",
					childNodes: []diffNode{&node{
						keyValue:   "Fn::Select",
						childNodes: []diffNode{unchanged, changedAZName},
					}},
				}
				return &node{
					childNodes: []diffNode{leaf},
				}
			},
		},
		"no diff in Fn::ImportValue vs !ImportValue": {
			old: `
V:
  Fn::ImportValue: sharedValueToImport`,
			curr: `
V: !ImportValue sharedValueToImport`,
		},
		"detect diff in Fn::ImportValue vs !ImportValue": {
			old: `
TestImportValue:
  Fn::ImportValue: pineapple1`,
			curr: `
TestImportValue: !ImportValue pineapple2`,
			wanted: func() diffNode {
				leaf := &node{
					keyValue: "Fn::ImportValue",
					oldV:     yamlScalarNode("pineapple1"),
					newV:     yamlScalarNode("pineapple2"),
				}
				return &node{
					childNodes: []diffNode{
						&node{
							keyValue:   "TestImportValue",
							childNodes: []diffNode{leaf},
						},
					},
				}
			},
		},
		"no diff in Fn::Join vs !Join": {
			old: `
V:
  Fn::Join: ['', ['arn:',!Ref AWS::Partition, ':s3:::elasticbeanstalk-*-',!Ref AWS::AccountId ]]`,
			curr: `
V:
  !Join
    - ''
    - - 'arn:'
      - !Ref AWS::Partition
      - ':s3:::elasticbeanstalk-*-'
      - !Ref AWS::AccountId`,
		},
		"detect diff in Fn::Join vs !Join": {
			old: `
TestJoin:
  Fn::Join: ['', ['arn:',!Ref AWS::Partition, ':s3:::elasticbeanstalk-*-pineapple1',!Ref AWS::AccountId ]]`,
			curr: `
TestJoin:
  !Join
    - ''
    - - 'arn:'
      - !Ref AWS::Partition
      - ':s3:::elasticbeanstalk-*-pineapple2'
      - !Ref AWS::AccountId`,
			wanted: func() diffNode {
				leaf := &seqItemNode{
					node{
						oldV: yamlScalarNode(":s3:::elasticbeanstalk-*-pineapple1", withStyle(yaml.SingleQuotedStyle)),
						newV: yamlScalarNode(":s3:::elasticbeanstalk-*-pineapple2", withStyle(yaml.SingleQuotedStyle)),
					},
				}
				joinElementsNode := &seqItemNode{
					node{
						childNodes: []diffNode{&unchangedNode{count: 2}, leaf, &unchangedNode{count: 1}},
					},
				}
				joinNode := &node{
					keyValue:   "Fn::Join",
					childNodes: []diffNode{&unchangedNode{count: 1}, joinElementsNode},
				}
				return &node{
					childNodes: []diffNode{
						&node{
							keyValue: "TestJoin",
							childNodes: []diffNode{
								joinNode,
							},
						},
					},
				}
			},
		},
		"no diff in Fn::Select vs !Select": {
			old: `CidrBlock: !Select [ 0, !Ref DbSubnetIpBlocks ]`,
			curr: `CidrBlock:
  Fn::Select:
    - 0
    - !Ref DbSubnetIpBlocks`,
		},
		"detect diff in Fn::Select vs !Select": {
			old: `TestSelect: !Select [ 1, !Ref DbSubnetIpBlocks ]`,
			curr: `TestSelect:
  Fn::Select:
    - 2
    - !Ref DbSubnetIpBlocks`,
			wanted: func() diffNode {
				leaf := &seqItemNode{
					node{
						oldV: yamlScalarNode("1"),
						newV: yamlScalarNode("2"),
					},
				}
				return &node{
					childNodes: []diffNode{
						&node{
							keyValue: "TestSelect",
							childNodes: []diffNode{
								&node{
									keyValue:   "Fn::Select",
									childNodes: []diffNode{leaf, &unchangedNode{1}},
								},
							},
						},
					},
				}
			},
		},
		"no diff in Fn::Split vs !Split": {
			old: `V: !Split [ "|" , "a||c|" ]`,
			curr: `
V:
  Fn::Split: [ "|" , "a||c|" ]`,
		},
		"detect diff in Fn::Split vs !Split": {
			old: `TestSplit: !Split [ "|" , "a||c|pineapple1" ]`,
			curr: `
TestSplit:
  Fn::Split: [ "|" , "a||c|pineapple2" ]`,
			wanted: func() diffNode {
				leaf := &seqItemNode{
					node{
						oldV: yamlScalarNode("a||c|pineapple1", withStyle(yaml.DoubleQuotedStyle)),
						newV: yamlScalarNode("a||c|pineapple2", withStyle(yaml.DoubleQuotedStyle)),
					},
				}
				return &node{
					childNodes: []diffNode{
						&node{
							keyValue: "TestSplit",
							childNodes: []diffNode{
								&node{
									keyValue:   "Fn::Split",
									childNodes: []diffNode{&unchangedNode{count: 1}, leaf},
								},
							},
						},
					},
				}
			},
		},
		"no diff in Fn::Sub vs !Sub": {
			old: `
Name: !Sub
  - 'www.${Domain}'
  - Domain: !Ref RootDomainName`,
			curr: `
Name:
  Fn::Sub:
    - 'www.${Domain}'
    - Domain: !Ref RootDomainName`,
		},
		"detect diff in Fn::Sub vs !Sub": {
			old: `
TestSub: !Sub
  - 'www.${Domain}.pineapple1'
  - Domain: !Ref RootDomainName`,
			curr: `
TestSub:
  Fn::Sub:
    - 'www.${Domain}.pineapple2'
    - Domain: !Ref RootDomainName`,
			wanted: func() diffNode {
				leaf := &seqItemNode{
					node{
						oldV: yamlScalarNode("www.${Domain}.pineapple1", withStyle(yaml.SingleQuotedStyle)),
						newV: yamlScalarNode("www.${Domain}.pineapple2", withStyle(yaml.SingleQuotedStyle)),
					},
				}
				return &node{
					childNodes: []diffNode{
						&node{
							keyValue: "TestSub",
							childNodes: []diffNode{
								&node{
									keyValue:   "Fn::Sub",
									childNodes: []diffNode{leaf, &unchangedNode{1}},
								},
							},
						},
					},
				}
			},
		},
		// 		"no diff in Fn::Transform vs !Transform": {// TODO(lou1415926)
		// 			old: `
		// Fn::Transform:
		//   Name : macro name
		//   Parameters :
		//     Key : value`,
		// 			curr: `
		// Transform:
		//   Name: macro name
		//   Parameters:
		//     Key: value`,
		// 		},
		// "no diff in Condition vs !Condition": // TODO(lou1415926)
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := From(tc.old).Parse([]byte(tc.curr), &intrinsicFuncFullShortFormConverter{})
			require.NoError(t, err)
			if tc.wanted != nil {
				require.True(t, equalTree(got, Tree{tc.wanted()}, t), "should get the expected tree")
			} else {
				require.True(t, equalTree(got, Tree{}, t), "should get the expected tree")
			}
		})
	}
}
