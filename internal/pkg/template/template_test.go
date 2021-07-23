// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/gobuffalo/packd"
	"github.com/stretchr/testify/require"
)

func TestTemplate_Read(t *testing.T) {
	testCases := map[string]struct {
		inPath           string
		mockDependencies func(t *Template)

		wantedContent string
		wantedErr     error
	}{
		"template does not exist": {
			inPath: "/fake/manifest.yml",
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				t.box = mockBox
			},

			wantedErr: errors.New("read template /fake/manifest.yml"),
		},
		"returns content": {
			inPath: "/fake/manifest.yml",
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				mockBox.AddString("/fake/manifest.yml", "hello")
				t.box = mockBox
			},

			wantedContent: "hello",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{}
			tc.mockDependencies(tpl)

			// WHEN
			c, err := tpl.Read(tc.inPath)

			if tc.wantedErr != nil {
				require.Contains(t, err.Error(), tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedContent, c.String())
			}
		})
	}
}

func TestTemplate_UploadEnvironmentCustomResources(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(t *Template)

		wantedErr error
	}{
		"success": {
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				for _, file := range envCustomResourceFiles {
					mockBox.AddString(fmt.Sprintf("custom-resources/%s.js", file), "hello")
				}
				t.box = mockBox
			},
		},
		"errors if env custom resource file doesn't exist": {
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				mockBox.AddString("badFile", "hello")
				t.box = mockBox
			},
			wantedErr: fmt.Errorf("read template custom-resources/dns-cert-validator.js: file does not exist"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{}
			tc.mockDependencies(tpl)
			mockUploader := s3.CompressAndUploadFunc(func(key string, files ...s3.NamedBinary) (string, error) {
				require.Contains(t, key, "scripts")
				require.Contains(t, key, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824")
				return "mockURL", nil
			})

			// WHEN
			gotCustomResources, err := tpl.UploadEnvironmentCustomResources(mockUploader)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, len(envCustomResourceFiles), len(gotCustomResources))
			}
		})
	}
}

func TestTemplate_UploadRequestDrivenWebServiceCustomResources(t *testing.T) {
	mockContent := "hello"
	testCases := map[string]struct {
		mockDependencies func(t *Template)
		mockUploader     s3.CompressAndUploadFunc

		wantedErr  error
		wantedURLs map[string]string
	}{
		"success": {
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				for _, file := range rdWkldCustomResourceFiles {
					mockBox.AddString(fmt.Sprintf("custom-resources/%s.js", file), mockContent)
				}
				t.box = mockBox
			},
			mockUploader: s3.CompressAndUploadFunc(func(key string, files ...s3.NamedBinary) (string, error) {
				require.Contains(t, key, "scripts")
				require.Contains(t, key, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824")
				for _, f := range files {
					require.Equal(t, mockContent, string(f.Content()))
				}
				return "mockURL", nil
			}),
			wantedURLs: map[string]string{
				AppRunnerCustomDomainLambdaFileName: "mockURL",
			},
		},
		"errors if rd web service custom resource file doesn't exist": {
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				mockBox.AddString("badFile", "hello")
				t.box = mockBox
			},
			wantedErr: fmt.Errorf("read template custom-resources/custom-domain-app-runner.js: file does not exist"),
		},
		"fail to upload": {
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				for _, file := range rdWkldCustomResourceFiles {
					mockBox.AddString(fmt.Sprintf("custom-resources/%s.js", file), mockContent)
				}
				t.box = mockBox
			},
			mockUploader: s3.CompressAndUploadFunc(func(key string, files ...s3.NamedBinary) (string, error) {
				require.Contains(t, key, "scripts")
				require.Contains(t, key, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824")
				for _, f := range files {
					require.Equal(t, mockContent, string(f.Content()))
				}
				if strings.Contains(key, "custom-domain-app-runner") {
					return "", errors.New("some error") // Upload fail on the custom-domain-app-runner.js
				} else {
					return "mockURL", nil
				}
			}),
			wantedErr: errors.New("upload scripts/custom-domain-app-runner: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{}
			tc.mockDependencies(tpl)

			// WHEN
			gotURLs, err := tpl.UploadRequestDrivenWebServiceCustomResources(tc.mockUploader)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, len(rdWkldCustomResourceFiles), len(gotURLs))
				require.Equal(t, tc.wantedURLs, gotURLs)
			}
		})
	}
}

func TestTemplate_UploadRequestDrivenWebServiceLayers(t *testing.T) {
	mockContent := "hello"
	testCases := map[string]struct {
		mockDependencies func(t *Template)
		mockUploader     s3.UploadFunc

		wantedURLs map[string]string
		wantedErr  error
	}{
		"success": {
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				for _, file := range rdWkldCustomResourceLayers {
					mockBox.AddString(fmt.Sprintf("custom-resources/%s.zip", file), mockContent)
				}
				t.box = mockBox
			},
			mockUploader: s3.UploadFunc(func(key string, file s3.NamedBinary) (string, error) {
				require.Contains(t, key, "layers")
				require.Contains(t, key, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824")
				require.Equal(t, mockContent, string(file.Content()))
				return "mockURL", nil
			}),
			wantedURLs: map[string]string{
				"aws-sdk-layer": "mockURL",
			},
		},
		"errors if rd web service custom layer file doesn't exist": {
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				mockBox.AddString("badFile", "hello")
				t.box = mockBox
			},
			wantedErr: fmt.Errorf("read template custom-resources/aws-sdk-layer.zip: file does not exist"),
		},
		"fail to upload": {
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				for _, file := range rdWkldCustomResourceLayers {
					mockBox.AddString(fmt.Sprintf("custom-resources/%s.zip", file), mockContent)
				}
				t.box = mockBox
			},
			mockUploader: s3.UploadFunc(func(key string, file s3.NamedBinary) (string, error) {
				require.Contains(t, key, "layers")
				require.Contains(t, key, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824")
				require.Equal(t, mockContent, string(file.Content()))
				if strings.Contains(key, "aws-sdk-layer") {
					return "", errors.New("some error") // Upload fail on the aws-sdk zip.
				} else {
					return "mockURL", nil
				}
			}),
			wantedErr: errors.New("upload layers/aws-sdk-layer: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{}
			tc.mockDependencies(tpl)

			// WHEN
			gotURLs, err := tpl.UploadRequestDrivenWebServiceLayers(tc.mockUploader)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, len(rdWkldCustomResourceLayers), len(gotURLs))
				require.Equal(t, tc.wantedURLs, gotURLs)
			}
		})
	}
}

func TestTemplate_Parse(t *testing.T) {
	testCases := map[string]struct {
		inPath           string
		inData           interface{}
		mockDependencies func(t *Template)

		wantedContent string
		wantedErr     error
	}{
		"template does not exist": {
			inPath: "/fake/manifest.yml",
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				t.box = mockBox
			},

			wantedErr: errors.New("read template /fake/manifest.yml"),
		},
		"template cannot be parsed": {
			inPath: "/fake/manifest.yml",
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				mockBox.AddString("/fake/manifest.yml", `{{}}`)
				t.box = mockBox
			},

			wantedErr: errors.New("parse template /fake/manifest.yml"),
		},
		"template cannot be executed": {
			inPath: "/fake/manifest.yml",
			inData: struct{}{},
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				mockBox.AddString("/fake/manifest.yml", `{{.Name}}`)
				t.box = mockBox
			},

			wantedErr: fmt.Errorf("execute template %s with data %v", "/fake/manifest.yml", struct{}{}),
		},
		"valid template": {
			inPath: "/fake/manifest.yml",
			inData: struct {
				Name string
			}{
				Name: "webhook",
			},
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				mockBox.AddString("/fake/manifest.yml", `{{.Name}}`)
				t.box = mockBox
			},

			wantedContent: "webhook",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{}
			tc.mockDependencies(tpl)

			// WHEN
			c, err := tpl.Parse(tc.inPath, tc.inData)

			if tc.wantedErr != nil {
				require.Contains(t, err.Error(), tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedContent, c.String())
			}
		})
	}
}
