// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/stretchr/testify/require"
)

// mockReadFileFS implements the fs.ReadFileFS interface.
type mockReadFileFS struct {
	fs map[string][]byte
}

func (m *mockReadFileFS) ReadFile(name string) ([]byte, error) {
	dat, ok := m.fs[name]
	if !ok {
		return nil, &fs.PathError{
			Op:   "open",
			Path: name,
			Err:  fs.ErrNotExist,
		}
	}
	return dat, nil
}

func (m *mockReadFileFS) Open(name string) (fs.File, error) {
	return nil, errors.New("open should not be called")
}

func TestTemplate_Read(t *testing.T) {
	testCases := map[string]struct {
		inPath string
		fs     map[string][]byte

		wantedContent string
		wantedErr     error
	}{
		"template does not exist": {
			inPath:    "/fake/manifest.yml",
			wantedErr: errors.New("read template /fake/manifest.yml"),
		},
		"returns content": {
			inPath: "/fake/manifest.yml",
			fs: map[string][]byte{
				"templates/fake/manifest.yml": []byte("hello"),
			},
			wantedContent: "hello",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{
				fs: &mockReadFileFS{tc.fs},
			}

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
		fs func() map[string][]byte

		wantedErr error
	}{
		"success": {
			fs: func() map[string][]byte {
				m := make(map[string][]byte)
				for _, file := range envCustomResourceFiles {
					m[fmt.Sprintf("templates/custom-resources/%s.js", file)] = []byte("hello")
				}
				return m
			},
		},
		"errors if env custom resource file doesn't exist": {
			fs: func() map[string][]byte {
				return nil
			},
			wantedErr: fmt.Errorf("read template custom-resources/dns-cert-validator.js: open templates/custom-resources/dns-cert-validator.js: file does not exist"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{
				fs: &mockReadFileFS{tc.fs()},
			}
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
		fs           func() map[string][]byte
		mockUploader s3.CompressAndUploadFunc

		wantedErr  error
		wantedURLs map[string]string
	}{
		"success": {
			fs: func() map[string][]byte {
				m := make(map[string][]byte)
				for _, file := range rdWkldCustomResourceFiles {
					m[fmt.Sprintf("templates/custom-resources/%s.js", file)] = []byte(mockContent)
				}
				return m
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
			fs: func() map[string][]byte {
				return nil
			},
			wantedErr: fmt.Errorf("read template custom-resources/custom-domain-app-runner.js: open templates/custom-resources/custom-domain-app-runner.js: file does not exist"),
		},
		"fail to upload": {
			fs: func() map[string][]byte {
				m := make(map[string][]byte)
				for _, file := range rdWkldCustomResourceFiles {
					m[fmt.Sprintf("templates/custom-resources/%s.js", file)] = []byte(mockContent)
				}
				return m
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
			tpl := &Template{
				fs: &mockReadFileFS{tc.fs()},
			}

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

func TestTemplate_Parse(t *testing.T) {
	testCases := map[string]struct {
		inPath string
		inData interface{}
		fs     map[string][]byte

		wantedContent string
		wantedErr     error
	}{
		"template does not exist": {
			inPath: "/fake/manifest.yml",

			wantedErr: errors.New("read template /fake/manifest.yml"),
		},
		"template cannot be parsed": {
			inPath: "/fake/manifest.yml",
			fs: map[string][]byte{
				"templates/fake/manifest.yml": []byte(`{{}}`),
			},

			wantedErr: errors.New("parse template /fake/manifest.yml"),
		},
		"template cannot be executed": {
			inPath: "/fake/manifest.yml",
			inData: struct{}{},
			fs: map[string][]byte{
				"templates/fake/manifest.yml": []byte(`{{.Name}}`),
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
			fs: map[string][]byte{
				"templates/fake/manifest.yml": []byte(`{{.Name}}`),
			},
			wantedContent: "webhook",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{
				fs: &mockReadFileFS{tc.fs},
			}

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
