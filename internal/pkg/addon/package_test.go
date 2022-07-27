// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"bytes"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type addonMocks struct {
	// uploader *mocks.Mockuploader
	// ws *mocks.MockworkspaceReader
}

func TestPackage(t *testing.T) {
	const (
		wlName = "mock-wl"
		wsPath = "/"
		bucket = "mockBucket"
	)

	tests := map[string]struct {
		inTemplate  string
		outTemplate string
		setupMocks  func(m addonMocks)
	}{
		"AWS::Lambda::Function, zipped file": {
			inTemplate: `
Resources:
  Test:
    Metadata:
      "testKey": "testValue"
    Type: AWS::Lambda::Function
    Properties:
      Code: lambda/index.js
      Handler: "index.handler"
      Timeout: 900
      MemorySize: 512
      Role: !GetAtt "HelloWorldRole.Arn"
      Runtime: nodejs12.x
`,
			outTemplate: `
Resources:
  Test:
    Metadata:
      "testKey": "testValue"
    Type: AWS::Lambda::Function
    Properties:
      Code:
        S3Bucket: mockBucket
        S3Key: TODO
      Handler: "index.handler"
      Timeout: 900
      MemorySize: 512
      Role: !GetAtt "HelloWorldRole.Arn"
      Runtime: nodejs12.x
`,
		},
		"AWS::Glue::Job, non-zipped file": {
			inTemplate: `
Resources:
  Test:
    Metadata:
      "testKey": "testValue"
    Type: AWS::Glue::Job
    Properties:
      Command:
        ScriptLocation: lambda/index.js
`,
			outTemplate: `
Resources:
  Test:
    Metadata:
      "testKey": "testValue"
    Type: AWS::Glue::Job
    Properties:
      Command:
        ScriptLocation: s3://mockBucket/TODO
`,
		},
		"AWS::CodeCommit::Repository, directory without slash": {
			inTemplate: `
Resources:
  Test:
    Metadata:
      "testKey": "testValue"
    Type: AWS::CodeCommit::Repository
    Properties:
      Code:
        S3: lambda
`,
			outTemplate: `
Resources:
  Test:
    Metadata:
      "testKey": "testValue"
    Type: AWS::CodeCommit::Repository
    Properties:
      Code:
        S3:
          Bucket: mockBucket
          Key: TODO
`,
		},
		"AWS::ApiGateway::RestApi, directory with slash": {
			inTemplate: `
Resources:
  Test:
    Metadata:
      "testKey": "testValue"
    Type: AWS::ApiGateway::RestApi
    Properties:
      BodyS3Location: lambda/
`,
			outTemplate: `
Resources:
  Test:
    Metadata:
      "testKey": "testValue"
    Type: AWS::ApiGateway::RestApi
    Properties:
      BodyS3Location:
        Bucket: mockBucket
        Key: TODO
`,
		},
		"AWS::AppSync::Resolver, multiple replacements in one resource": {
			inTemplate: `
Resources:
  Test:
    Metadata:
      "testKey": "testValue"
    Type: AWS::AppSync::Resolver
    Properties:
      RequestMappingTemplateS3Location: lambda
      ResponseMappingTemplateS3Location: lambda/index.js
`,
			outTemplate: `
Resources:
  Test:
    Metadata:
      "testKey": "testValue"
    Type: AWS::AppSync::Resolver
    Properties:
      RequestMappingTemplateS3Location: s3://mockBucket/TODO
      ResponseMappingTemplateS3Location: s3://mockBucket/TODO
`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := addonMocks{}
			if tc.setupMocks != nil {
				tc.setupMocks(mocks)
			}

			a := &Addons{
				wlName: wlName,
				bucket: bucket,
			}

			tmpl := newCFNTemplate("merged")
			err := yaml.Unmarshal([]byte(tc.inTemplate), tmpl)
			require.NoError(t, err)

			require.NoError(t, tmpl.packageTemplate(a))

			buf := &bytes.Buffer{}
			enc := yaml.NewEncoder(buf)
			enc.SetIndent(2)

			require.NoError(t, enc.Encode(tmpl))
			require.Equal(t, strings.TrimSpace(tc.outTemplate), strings.TrimSpace(buf.String()))
		})
	}
}
