// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIntrinsicFuncFullShortFormConverter(t *testing.T) {
	testCases := map[string]struct {
		curr   string
		old    string
		wanted func() diffNode
	}{
		"no diff in Ref and !Ref": {
			old: `Value: !Sub 'blah'`,
			curr: `Value:
  Fn::Sub: 'blah'`,
		},
		"no diff in Fn::Base64 amd !Base64": {
			old: `Value:
  Fn::Base64: "AWS CloudFormation"`,
			curr: `Value: !Base64 AWS CloudFormation`,
		},
		"no diff in Fn::Cidr vs !Cidr": {
			old: `CidrBlock:
  Fn::Cidr:
    - 192.168.0.0/24
    - 6
    - 5`,
			curr: `CidrBlock: !Cidr ["192.168.0.0/24", 6, 5 ]`,
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
		"no diff in Fn::ImportValue vs !ImportValue": {
			old: `
V:
  Fn::ImportValue: sharedValueToImport`,
			curr: `
V: !ImportValue sharedValueToImport`,
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
		"no diff in Fn::Select vs !Select": {
			old: `CidrBlock: !Select [ 0, !Ref DbSubnetIpBlocks ]`,
			curr: `CidrBlock:
  Fn::Select:
    - 0
    - !Ref DbSubnetIpBlocks`,
		},
		"no diff in Fn::Split vs !Split": {
			old: `V: !Split [ "|" , "a||c|" ]`,
			curr: `
V:
  Fn::Split: [ "|" , "a||c|" ]`,
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
			got, err := From(tc.old).parseRoot([]byte(tc.curr), &intrinsicFuncFullShortFormConverter{})
			require.NoError(t, err)
			if tc.wanted != nil {
				require.True(t, equalTree(got, Tree{tc.wanted()}, t), "should get the expected tree")
			} else {
				require.True(t, equalTree(got, Tree{}, t), "should get the expected tree")
			}
		})
	}
}
