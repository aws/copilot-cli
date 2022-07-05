package addon

import (
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
	tests := map[string]struct {
		inTemplate  string
		outTemplate string
		fs          func() afero.Fs
		setupMocks  func(m addonMocks)
	}{
		"lambda": {
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.Mkdir("/lambda", 0644)
				f, _ := fs.Create("/lambda/index.js")
				defer f.Close()
				f.Write([]byte(`exports.handler = function(event, context) {}`))
				return fs
			},
			setupMocks: func(m addonMocks) {
				m.uploader.EXPECT().Upload(bucket, gomock.Any(), gomock.Any()).Return(s3.URL("us-west-2", bucket, "asdf"), nil)
			},
			inTemplate: `
Resources:
  HelloWorldFunctionFile:
    Metadata:
      "testKey": "testValue"
    Type: AWS::Lambda::Function
    Properties:
      Code: lambda/
      Handler: "index.handler"
      Timeout: 900
      MemorySize: 512
      Role: !GetAtt "HelloWorldRole.Arn"
      Runtime: nodejs12.x
`,
			outTemplate: `
Resources:
  HelloWorldFunctionFile:
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
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := addonMocks{
				uploader: mocks.NewMockuploader(ctrl),
				ws:       mocks.NewMockworkspaceReader(ctrl),
			}
			tc.setupMocks(mocks)

			a := &Addons{
				wlName:   wlName,
				wsPath:   wsPath,
				Uploader: mocks.uploader,
				ws:       mocks.ws,
			}

			tmpl := newCFNTemplate("merged")
			err := yaml.Unmarshal([]byte(tc.inTemplate), tmpl)
			require.NoError(t, err)

			packaged, err := a.packageLocalArtifacts(tmpl)
			require.NoError(t, err)

			out, err := yaml.Marshal(packaged)
			require.NoError(t, err)
			require.Equal(t, tc.outTemplate, out)
		})
	}
}
