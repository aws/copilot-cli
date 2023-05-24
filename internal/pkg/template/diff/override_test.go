// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestIntrinsicFuncConverters(t *testing.T) {
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
				leaf := &keyNode{
					keyValue: "Ref",
					oldV:     yamlScalarNode("pineapple1"),
					newV:     yamlScalarNode("pineapple2"),
				}
				return &keyNode{
					childNodes: []diffNode{&keyNode{
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
				leaf := &keyNode{
					keyValue: "Fn::Base64",
					oldV:     yamlScalarNode("1"),
					newV:     yamlScalarNode("2"),
				}
				return &keyNode{
					childNodes: []diffNode{&keyNode{
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
					keyNode: keyNode{
						oldV: yamlScalarNode("192.168.0.0/16", withStyle(yaml.DoubleQuotedStyle)),
						newV: yamlScalarNode("192.168.0.0/24"),
					},
				}
				unchanged := &unchangedNode{count: 1}
				changedNum := &seqItemNode{
					keyNode: keyNode{
						oldV: yamlScalarNode("4"),
						newV: yamlScalarNode("5"),
					},
				}
				return &keyNode{
					childNodes: []diffNode{&keyNode{
						keyValue: "cedar",
						childNodes: []diffNode{&keyNode{
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
					keyNode: keyNode{
						oldV: yamlScalarNode("RegionMap"),
						newV: yamlScalarNode("Chizu"),
					},
				}
				unchanged := &unchangedNode{count: 2}
				leaf := &keyNode{
					keyValue: "ImageId",
					childNodes: []diffNode{&keyNode{
						keyValue:   "Fn::FindInMap",
						childNodes: []diffNode{changedMapName, unchanged},
					}},
				}
				return &keyNode{
					childNodes: []diffNode{leaf},
				}
			},
		},
		"no diff in Fn::GetAtt vs !GetAtt when comparing list to scalar": {
			old: `SourceSecurityGroupOwnerId:
  Fn::GetAtt:
    - myELB
    - SourceSecurityGroup.OwnerAlias`,
			curr: `SourceSecurityGroupOwnerId: !GetAtt myELB.SourceSecurityGroup.OwnerAlias`,
		},
		"diff in Fn::GetAtt vs !GetAtt when comparing list to scalar": {
			old: `SourceSecurityGroupOwnerId: !GetAtt myELB.SourceSecurityGroup.OwnerAlias`,
			curr: `SourceSecurityGroupOwnerId:
  Fn::GetAtt:
    - theirELB
    - SourceSecurityGroup.OwnerAlias`,
			wanted: func() diffNode {
				changedLogicalID := &seqItemNode{
					keyNode: keyNode{
						oldV: yamlScalarNode("myELB"),
						newV: yamlScalarNode("theirELB"),
					},
				}
				unchanged := &unchangedNode{count: 1}
				return &keyNode{
					childNodes: []diffNode{&keyNode{
						keyValue: "SourceSecurityGroupOwnerId",
						childNodes: []diffNode{&keyNode{
							keyValue:   "Fn::GetAtt",
							childNodes: []diffNode{changedLogicalID, unchanged},
						}},
					}},
				}
			},
		},
		"no diff in Fn::GetAtt vs !GetAtt when comparing list to list": {
			old: `SourceSecurityGroupOwnerId: !GetAtt [myELB, SourceSecurityGroup]`,
			curr: `SourceSecurityGroupOwnerId:
  Fn::GetAtt:
    - myELB
    - SourceSecurityGroup`,
		},
		"diff in Fn::GetAtt vs !GetAtt when comparing list to list": {
			old: `SourceSecurityGroupOwnerId: !GetAtt [myELB, SourceSecurityGroup.OwnerAlias]`,
			curr: `SourceSecurityGroupOwnerId:
  Fn::GetAtt:
    - theirELB
    - SourceSecurityGroup.OwnerAlias`,
			wanted: func() diffNode {
				changedLogicalID := &seqItemNode{
					keyNode: keyNode{
						oldV: yamlScalarNode("myELB"),
						newV: yamlScalarNode("theirELB"),
					},
				}
				unchanged := &unchangedNode{count: 1}
				return &keyNode{
					childNodes: []diffNode{&keyNode{
						keyValue: "SourceSecurityGroupOwnerId",
						childNodes: []diffNode{&keyNode{
							keyValue:   "Fn::GetAtt",
							childNodes: []diffNode{changedLogicalID, unchanged},
						}},
					}},
				}
			},
		},
		"no diff in Fn::GetAtt vs !GetAtt when comparing scalar to scalar": {
			old: `SourceSecurityGroupOwnerId:
  Fn::GetAtt: myELB.SourceSecurityGroup`,
			curr: `SourceSecurityGroupOwnerId: !GetAtt myELB.SourceSecurityGroup`,
		},
		"diff in Fn::GetAtt vs !GetAtt when comparing scalar to scalar": {
			old: `SourceSecurityGroupOwnerId: !GetAtt myELB.SourceSecurityGroup.OwnerAlias`,
			curr: `SourceSecurityGroupOwnerId:
  Fn::GetAtt: theirELB.SourceSecurityGroup.OwnerAlias`,
			wanted: func() diffNode {
				return &keyNode{
					childNodes: []diffNode{&keyNode{
						keyValue: "SourceSecurityGroupOwnerId",
						childNodes: []diffNode{&keyNode{
							keyValue: "Fn::GetAtt",
							oldV:     yamlScalarNode("myELB.SourceSecurityGroup.OwnerAlias"),
							newV:     yamlScalarNode("theirELB.SourceSecurityGroup.OwnerAlias")}},
					}},
				}
			},
		},
		"no diff in Fn::GetAtt vs !GetAtt when comparing scalar to scalar both with tags": {
			old: `      SecurityGroups:
        - !GetAtt PublicHTTPLoadBalancerSecurityGroup.GroupId`,
			curr: `      SecurityGroups:
        - !GetAtt PublicHTTPLoadBalancerSecurityGroup.GroupId`,
		},
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
					keyNode{
						childNodes: []diffNode{
							&keyNode{
								keyValue: "Fn::GetAZs",
								oldV:     yamlScalarNode("amazon", withStyle(yaml.DoubleQuotedStyle)),
								newV:     yamlScalarNode("arizona", withStyle(yaml.SingleQuotedStyle)),
							},
						},
					},
				}
				leaf := &keyNode{
					keyValue: "AvailabilityZone",
					childNodes: []diffNode{&keyNode{
						keyValue:   "Fn::Select",
						childNodes: []diffNode{unchanged, changedAZName},
					}},
				}
				return &keyNode{
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
				leaf := &keyNode{
					keyValue: "Fn::ImportValue",
					oldV:     yamlScalarNode("pineapple1"),
					newV:     yamlScalarNode("pineapple2"),
				}
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
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
					keyNode{
						oldV: yamlScalarNode(":s3:::elasticbeanstalk-*-pineapple1", withStyle(yaml.SingleQuotedStyle)),
						newV: yamlScalarNode(":s3:::elasticbeanstalk-*-pineapple2", withStyle(yaml.SingleQuotedStyle)),
					},
				}
				joinElementsNode := &seqItemNode{
					keyNode{
						childNodes: []diffNode{&unchangedNode{count: 2}, leaf, &unchangedNode{count: 1}},
					},
				}
				joinNode := &keyNode{
					keyValue:   "Fn::Join",
					childNodes: []diffNode{&unchangedNode{count: 1}, joinElementsNode},
				}
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
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
					keyNode{
						oldV: yamlScalarNode("1"),
						newV: yamlScalarNode("2"),
					},
				}
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
							keyValue: "TestSelect",
							childNodes: []diffNode{
								&keyNode{
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
					keyNode{
						oldV: yamlScalarNode("a||c|pineapple1", withStyle(yaml.DoubleQuotedStyle)),
						newV: yamlScalarNode("a||c|pineapple2", withStyle(yaml.DoubleQuotedStyle)),
					},
				}
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
							keyValue: "TestSplit",
							childNodes: []diffNode{
								&keyNode{
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
					keyNode{
						oldV: yamlScalarNode("www.${Domain}.pineapple1", withStyle(yaml.SingleQuotedStyle)),
						newV: yamlScalarNode("www.${Domain}.pineapple2", withStyle(yaml.SingleQuotedStyle)),
					},
				}
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
							keyValue: "TestSub",
							childNodes: []diffNode{
								&keyNode{
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
		"do not match unexpected keys": {
			old: `
Stuff:
  Sub: not_an_intrinsic_function`,
			curr: `
Stuff: !Sub this_is_one`,
			wanted: func() diffNode {
				return &keyNode{
					childNodes: []diffNode{
						&keyNode{
							keyValue: "Stuff",
							oldV:     yamlNode("Sub: not_an_intrinsic_function", t),
							newV:     yamlNode("!Sub this_is_one", t),
						},
					},
				}
			},
		},
		"no diff in Condition vs !Condition": {
			old: `ALB:
  - Condition: CreateALB`,
			curr: `ALB:
  - !Condition CreateALB`,
		},
		"no diff in Fn::And vs !And": {
			old: `
ALB:
  Fn::And: [this, that]`,
			curr: `
ALB: !And
  - this
  - that`,
		},
		"no diff in Fn::Equals vs !Equals": {
			old: `
UseProdCondition:
  Fn::Equals: [!Ref EnvironmentType, prod]`,
			curr: `
UseProdCondition:
  !Equals [!Ref EnvironmentType, prod]`,
		},
		"no diff in Fn::If vs !If": {
			old: `
SecurityGroups:
  - !If [CreateNewSecurityGroup, !Ref NewSecurityGroup, !Ref ExistingSecurityGroup]`,
			curr: `
SecurityGroups:
  - Fn::If: [CreateNewSecurityGroup, !Ref NewSecurityGroup, !Ref ExistingSecurityGroup]`,
		},
		"no diff in Fn::Not vs !Not": {
			old: `
MyNotCondition:
  !Not [!Equals [!Ref EnvironmentType, prod]]`,
			curr: `
MyNotCondition:
  Fn::Not: [!Equals [!Ref EnvironmentType, prod]]`,
		},
		"no diff in Fn::Or vs !Or": {
			old: `
MyOrCondition:
  Fn::Or: [!Equals [sg-mysggroup, !Ref ASecurityGroup], Condition: SomeOtherCondition]`,
			curr: `
MyOrCondition:
  !Or [!Equals [sg-mysggroup, !Ref ASecurityGroup], Condition: SomeOtherCondition]`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := From(tc.old).Parse([]byte(tc.curr), &getAttConverter{}, &intrinsicFuncMapTagConverter{})
			require.NoError(t, err)
			got.Write(os.Stdout)
			if tc.wanted != nil {
				require.True(t, equalTree(got, Tree{tc.wanted()}, t), "should get the expected tree")
			} else {
				require.True(t, equalTree(got, Tree{}, t), "should get the expected tree")
			}
		})
	}
}
