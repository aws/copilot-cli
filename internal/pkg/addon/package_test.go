// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	ws       *mocks.MockWorkspaceAddonsReader
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
		pkgError    string
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
      Role: !GetAtt "TestRole.Arn"
      Runtime: nodejs16.x
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
      Role: !GetAtt "TestRole.Arn"
      Runtime: nodejs16.x
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
		"Fn::Transform in lambda function": {
			setupMocks: func(m addonMocks) {
				m.uploader.EXPECT().Upload(bucket, indexFileS3Path, gomock.Any()).Return(s3.URL("us-west-2", bucket, "asdf"), nil)
				m.uploader.EXPECT().Upload(bucket, lambdaZipS3Path, gomock.Any()).Return(s3.URL("us-west-2", bucket, "hjkl"), nil)
			},
			inTemplate: `
Resources:
  Test:
    Metadata:
      "hihi": "byebye"
    Type: AWS::Lambda::Function
    Properties:
      Fn::Transform:
        # test comment uno
        Name: "AWS::Include"
        Parameters:
          # test comment dos
          Location: ./lambda/index.js
      Code: lambda
      Handler: "index.handler"
      Timeout: 900
      MemorySize: 512
      Role: !GetAtt "TestRole.Arn"
      Runtime: nodejs16.x
`,
			outTemplate: `
Resources:
  Test:
    Metadata:
      "hihi": "byebye"
    Type: AWS::Lambda::Function
    Properties:
      Fn::Transform:
        # test comment uno
        Name: "AWS::Include"
        Parameters:
          # test comment dos
          Location: s3://mockBucket/asdf
      Code:
        S3Bucket: mockBucket
        S3Key: hjkl
      Handler: "index.handler"
      Timeout: 900
      MemorySize: 512
      Role: !GetAtt "TestRole.Arn"
      Runtime: nodejs16.x
`,
		},
		"Fn::Transform nested in a yaml mapping and sequence node": {
			setupMocks: func(m addonMocks) {
				m.uploader.EXPECT().Upload(bucket, indexFileS3Path, gomock.Any()).Return(s3.URL("us-west-2", bucket, "asdf"), nil).Times(2)
			},
			inTemplate: `
Resources:
  Test:
    Type: AWS::Fake::Resource
    Properties:
      SequenceProperty:
        - KeyOne: ValOne
          KeyTwo: ValTwo
        - Fn::Transform:
            Name: "AWS::Include"
            Parameters:
              Location: ./lambda/index.js
      MappingProperty:
        KeyOne: ValOne
        Fn::Transform:
          Name: "AWS::Include"
          Parameters:
            Location: ./lambda/index.js
`,
			outTemplate: `
Resources:
  Test:
    Type: AWS::Fake::Resource
    Properties:
      SequenceProperty:
        - KeyOne: ValOne
          KeyTwo: ValTwo
        - Fn::Transform:
            Name: "AWS::Include"
            Parameters:
              Location: s3://mockBucket/asdf
      MappingProperty:
        KeyOne: ValOne
        Fn::Transform:
          Name: "AWS::Include"
          Parameters:
            Location: s3://mockBucket/asdf
`,
		},
		"Fn::Transform ignores top level Transform": {
			// example from https://medium.com/swlh/using-the-cloudformation-aws-include-macro-9e3056cf75b0
			setupMocks: func(m addonMocks) {
				m.uploader.EXPECT().Upload(bucket, indexFileS3Path, gomock.Any()).Return(s3.URL("us-west-2", "chris.hare", "common-tags.yaml"), nil)
			},
			inTemplate: `
Parameters:
  CreatedBy:
    Type: String
    Description: Email address of the person creating the resource.
Transform:
  Name: 'AWS::Include'
  Parameters:
    Location: 's3://chris.hare/alb-cw-mapping.yaml'
Resources:
  bucket:
    Type: AWS::S3::Bucket
    Properties:
      Fn::Transform:
        Name: 'AWS::Include'
        Parameters:
          Location: './lambda/index.js'
Outputs:
  bucketDomainName:
    Value: !GetAtt bucket.DomainName
  bucket:
    Value: !Ref bucket
`,
			outTemplate: `
Parameters:
  CreatedBy:
    Type: String
    Description: Email address of the person creating the resource.
Transform:
  Name: 'AWS::Include'
  Parameters:
    Location: 's3://chris.hare/alb-cw-mapping.yaml'
Resources:
  bucket:
    Type: AWS::S3::Bucket
    Properties:
      Fn::Transform:
        Name: 'AWS::Include'
        Parameters:
          Location: 's3://chris.hare/common-tags.yaml'
Outputs:
  bucketDomainName:
    Value: !GetAtt bucket.DomainName
  bucket:
    Value: !Ref bucket
`,
		},
		"error on file not existing": {
			inTemplate: `
Resources:
  Test:
    Type: AWS::Lambda::Function
    Properties:
      Code: does/not/exist.js
`,
			pkgError: `package property "Code" of "Test": upload asset: stat: open /does/not/exist.js: file does not exist`,
		},
		"error on file upload error": {
			setupMocks: func(m addonMocks) {
				m.uploader.EXPECT().Upload(bucket, indexZipS3Path, gomock.Any()).Return("", errors.New("mockError"))
			},
			inTemplate: `
Resources:
  Test:
    Type: AWS::Lambda::Function
    Properties:
      Code: lambda/index.js
`,
			pkgError: `package property "Code" of "Test": upload asset: upload /lambda/index.js to s3 bucket mockBucket: mockError`,
		},
		"error on file not existing for Fn::Transform": {
			inTemplate: `
Resources:
  Test:
    Type: AWS::Lambda::Function
    Properties:
      Fn::Transform:
        Name: "AWS::Include"
        Parameters:
          Location: does/not/exist.yml
`,
			pkgError: `package transforms: upload asset: stat: open /does/not/exist.yml: file does not exist`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := addonMocks{
				uploader: mocks.NewMockuploader(ctrl),
				ws:       mocks.NewMockWorkspaceAddonsReader(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(mocks)
			}

			stack := &WorkloadStack{
				workloadName: wlName,
				stack: stack{
					template: newCFNTemplate("merged"),
				},
			}

			require.NoError(t, yaml.Unmarshal([]byte(tc.inTemplate), stack.template))

			config := PackageConfig{
				Bucket:        bucket,
				WorkspacePath: wsPath,
				Uploader:      mocks.uploader,
				FS:            fs,
			}
			err := stack.Package(config)
			if tc.pkgError != "" {
				require.EqualError(t, err, tc.pkgError)
				return
			}
			require.NoError(t, err)

			tmpl, err := stack.Template()
			require.NoError(t, err)

			require.Equal(t, strings.TrimSpace(tc.outTemplate), strings.TrimSpace(tmpl))
		})
	}

}

func TestEnvironmentAddonStack_PackagePackage(t *testing.T) {
	const (
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

	indexZipS3PathForEnvironmentAddon := fmt.Sprintf("manual/addons/environments/assets/%s", hex.EncodeToString(indexZipHash.Sum(nil)))
	t.Run("package zipped AWS::Lambda::Function for environment addons", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Set up mocks.
		m := addonMocks{
			uploader: mocks.NewMockuploader(ctrl),
			ws:       mocks.NewMockWorkspaceAddonsReader(ctrl),
		}
		m.uploader.EXPECT().Upload(bucket, indexZipS3PathForEnvironmentAddon, gomock.Any()).Return(s3.URL("us-west-2", bucket, "asdf"), nil)

		// WHEN.
		inTemplate := `
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
      Role: !GetAtt "TestRole.Arn"
      Runtime: nodejs16.x
`
		stack := &EnvironmentStack{
			stack: stack{
				template: newCFNTemplate("merged"),
			},
		}
		require.NoError(t, yaml.Unmarshal([]byte(inTemplate), stack.template))
		config := PackageConfig{
			Bucket:        bucket,
			WorkspacePath: wsPath,
			Uploader:      m.uploader,
			FS:            fs,
		}
		err := stack.Package(config)

		// Expect.
		outTemplate := `
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
      Role: !GetAtt "TestRole.Arn"
      Runtime: nodejs16.x
`
		require.NoError(t, err)
		tmpl, err := stack.Template()
		require.NoError(t, err)
		require.Equal(t, strings.TrimSpace(outTemplate), strings.TrimSpace(tmpl))
	})

}
