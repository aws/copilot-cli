// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOutputs(t *testing.T) {
	testCases := map[string]struct {
		template         string
		testdataFileName string

		wantedOut []Output
		wantedErr error
	}{
		"returns an error if Resources is not defined as a map": {
			template:  "Resources: hello",
			wantedErr: errors.New(`"Resources" field in cloudformation template is not a map`),
		},
		"returns an error if a resource does not define a \"Type\" field": {
			template: `
Resources:
  Hello: World
`,
			wantedErr: errors.New(`decode the "Type" field of resource "Hello"`),
		},
		"returns an error if Outputs is not defined as a map": {
			template: `
Resources:
  MyDBInstance:
    Type: AWS::RDS::DBInstance
Outputs: hello
`,
			wantedErr: errors.New(`"Outputs" field in cloudformation template is not a map`),
		},
		"returns an error if an output does not define a \"Value\" field": {
			template: `
Resources:
  MyDBInstance:
    Type: AWS::RDS::DBInstance
Outputs:
  Hello: World
`,
			wantedErr: errors.New(`decode the "Value" field of output "Hello"`),
		},
		"returns a nil list if there are no outputs defined": {
			template: `
Resources:
  MyDBInstance:
    Type: AWS::RDS::DBInstance
`,
		},
		"injects parameters defined in addons as environment variables": {
			// See #1565
			template: `
Parameters:
  MinPort:
    Type: Number
    Default: 40000
  MaxPort:
    Type: Number
    Default: 49999

Resources:
  EnvironmentSecurityGroupIngressTCPWebRTC:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      Description: Ingress TCP for WebRTC media streams
      GroupId:
        Fn::ImportValue:
          !Sub "${App}-${Env}-EnvironmentSecurityGroup"
      IpProtocol: tcp
      CidrIp: '0.0.0.0/0'
      FromPort: !Ref MinPort # <- Problem is here
      ToPort:   !Ref MaxPort # <- Problem is here

Outputs:
  MediasoupMinPort:
    Value: !Ref MinPort
  MediasoupMaxPort:
    Value: !Ref MaxPort`,
			wantedOut: []Output{
				{
					Name: "MediasoupMinPort",
				},
				{
					Name: "MediasoupMaxPort",
				},
			},
		},
		"parses CFN template with an IAM managed policy and secret": {
			testdataFileName: "template.yml",

			wantedOut: []Output{
				{
					Name:            "AdditionalResourcesPolicyArn",
					IsManagedPolicy: true,
				},
				{
					Name:     "MyRDSInstanceRotationSecretArn",
					IsSecret: true,
				},
				{
					Name: "MyDynamoDBTableName",
				},
				{
					Name: "MyDynamoDBTableArn",
				},
				{
					Name: "TestExport",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			template := tc.template
			if tc.testdataFileName != "" {
				content, err := os.ReadFile(filepath.Join("testdata", "outputs", tc.testdataFileName))
				require.NoError(t, err)
				template = string(content)
			}

			// WHEN
			out, err := Outputs(template)

			// THEN
			if tc.wantedErr != nil {
				require.NotNil(t, err, "expected a non-nil error to be returned")
				require.True(t, strings.HasPrefix(err.Error(), tc.wantedErr.Error()), "expected the error %v to be wrapped by our prefix %v", err, tc.wantedErr)
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.wantedOut, out)
			}
		})
	}
}
