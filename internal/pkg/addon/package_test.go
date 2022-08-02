// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/addon/mocks"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type addonMocks struct {
	uploader *mocks.Mockuploader
	ws       *mocks.MockworkspaceReader
}

func TestPackage(t *testing.T) {
	const (
		wlName = "mock-wl"
		wsPath = "/"
		bucket = "mockBucket"
	)

	lambdaZipHash := sha256.New()
	indexZipHash := sha256.New()
	indexFileHash := sha256.New()

	// fs has the following structure:
	//  .
	//  ├─ lambda
	//  │  ├─ index.js (contains lambda function)
	//  ┴  └─ test.js (empty)
	fs := afero.NewMemMapFs()
	fs.Mkdir("/lambda", 0644)

	f, _ := fs.Create("/lambda/index.js")
	defer f.Close()
	info, _ := f.Stat()
	io.MultiWriter(lambdaZipHash, indexZipHash).Write([]byte("index.js " + info.Mode().String()))
	io.MultiWriter(f, lambdaZipHash, indexZipHash, indexFileHash).Write([]byte(`exports.handler = function(event, context) {}`))

	f2, _ := fs.Create("/lambda/test.js")
	info, _ = f2.Stat()
	lambdaZipHash.Write([]byte("test.js " + info.Mode().String()))

	lambdaZipS3Path := fmt.Sprintf("manual/addons/mock-wl/assets/%s", hex.EncodeToString(lambdaZipHash.Sum(nil)))
	indexZipS3Path := fmt.Sprintf("manual/addons/mock-wl/assets/%s", hex.EncodeToString(indexZipHash.Sum(nil)))
	indexFileS3Path := fmt.Sprintf("manual/addons/mock-wl/assets/%s", hex.EncodeToString(indexFileHash.Sum(nil)))

	tests := map[string]struct {
		inTemplate  string
		outTemplate string
		setupMocks  func(m addonMocks)
	}{
		"AWS::Lambda::Function, zipped file": {
			setupMocks: func(m addonMocks) {
				m.uploader.EXPECT().Upload(bucket, indexZipS3Path, gomock.Any()).Return(s3.URL("us-west-2", bucket, "asdf"), nil)
			},
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
        S3Key: asdf
      Handler: "index.handler"
      Timeout: 900
      MemorySize: 512
      Role: !GetAtt "HelloWorldRole.Arn"
      Runtime: nodejs12.x
`,
		},
		"AWS::Glue::Job, non-zipped file": {
			setupMocks: func(m addonMocks) {
				m.uploader.EXPECT().Upload(bucket, indexFileS3Path, gomock.Any()).Return(s3.URL("us-east-2", bucket, "asdf"), nil)
			},
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
        ScriptLocation: s3://mockBucket/asdf
`,
		},
		"AWS::CodeCommit::Repository, directory without slash": {
			setupMocks: func(m addonMocks) {
				m.uploader.EXPECT().Upload(bucket, lambdaZipS3Path, gomock.Any()).Return(s3.URL("ap-northeast-1", bucket, "asdf"), nil)
			},
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
          Key: asdf
`,
		},
		"AWS::ApiGateway::RestApi, directory with slash": {
			setupMocks: func(m addonMocks) {
				m.uploader.EXPECT().Upload(bucket, lambdaZipS3Path, gomock.Any()).Return(s3.URL("eu-west-1", bucket, "asdf"), nil)
			},
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
        Key: asdf
`,
		},
		"AWS::AppSync::Resolver, multiple replacements in one resource": {
			setupMocks: func(m addonMocks) {
				m.uploader.EXPECT().Upload(bucket, lambdaZipS3Path, gomock.Any()).Return(s3.URL("ca-central-1", bucket, "asdf"), nil)
				m.uploader.EXPECT().Upload(bucket, indexFileS3Path, gomock.Any()).Return(s3.URL("ca-central-1", bucket, "hjkl"), nil)
			},
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
      RequestMappingTemplateS3Location: s3://mockBucket/asdf
      ResponseMappingTemplateS3Location: s3://mockBucket/hjkl
`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := addonMocks{
				uploader: mocks.NewMockuploader(ctrl),
				ws:       mocks.NewMockworkspaceReader(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(mocks)
			}

			a := &Addons{
				wlName:   wlName,
				wsPath:   wsPath,
				bucket:   bucket,
				uploader: mocks.uploader,
				fs: &afero.Afero{
					Fs: fs,
				},
			}

			tmpl := newCFNTemplate("merged")
			err := yaml.Unmarshal([]byte(tc.inTemplate), tmpl)
			require.NoError(t, err)

			require.NoError(t, tmpl.pkg(a))

			buf := &bytes.Buffer{}
			enc := yaml.NewEncoder(buf)
			enc.SetIndent(2)

			require.NoError(t, enc.Encode(tmpl))
			require.Equal(t, strings.TrimSpace(tc.outTemplate), strings.TrimSpace(buf.String()))
		})
	}
}
