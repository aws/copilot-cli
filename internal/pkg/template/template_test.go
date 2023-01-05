// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"testing"

	"github.com/spf13/afero"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/stretchr/testify/require"
)

// mockFS implements the fs.ReadFileFS interface.
type mockFS struct {
	afero.Fs
}

func (m *mockFS) ReadFile(name string) ([]byte, error) {
	return afero.ReadFile(m.Fs, name)
}

func (m *mockFS) ReadDir(name string) ([]fs.DirEntry, error) {
	files, err := afero.ReadDir(m.Fs, name)
	if err != nil {
		return nil, err
	}
	out := make([]fs.DirEntry, len(files))
	for i, f := range files {
		out[i] = &mockDirEntry{FileInfo: f}
	}
	return out, nil
}

func (m *mockFS) Open(name string) (fs.File, error) {
	return m.Fs.Open(name)
}

type mockDirEntry struct {
	os.FileInfo
}

func (m *mockDirEntry) Type() fs.FileMode {
	return m.Mode()
}

func (m *mockDirEntry) Info() (fs.FileInfo, error) {
	return m.FileInfo, nil
}

func TestTemplate_Read(t *testing.T) {
	testCases := map[string]struct {
		inPath string
		fs     func() afero.Fs

		wantedContent string
		wantedErr     error
	}{
		"template does not exist": {
			inPath: "/fake/manifest.yml",
			fs: func() afero.Fs {
				return afero.NewMemMapFs()
			},
			wantedErr: errors.New("read template /fake/manifest.yml"),
		},
		"returns content": {
			inPath: "/fake/manifest.yml",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll("templates/fake/", 0755)
				_ = afero.WriteFile(fs, "templates/fake/manifest.yml", []byte("hello"), 0644)
				return fs
			},
			wantedContent: "hello",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{
				fs: &mockFS{Fs: tc.fs()},
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
		fs func() afero.Fs

		wantedErr error
	}{
		"success": {
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll("templates/custom-resources/", 0755)
				for _, file := range envCustomResourceFiles {
					_ = afero.WriteFile(fs, fmt.Sprintf("templates/custom-resources/%s.js", file), []byte("hello"), 0644)
				}
				return fs
			},
		},
		"errors if env custom resource file doesn't exist": {
			fs: func() afero.Fs {
				return afero.NewMemMapFs()
			},
			wantedErr: fmt.Errorf("read template custom-resources/dns-cert-validator.js: open templates/custom-resources/dns-cert-validator.js: file does not exist"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{
				fs: &mockFS{tc.fs()},
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

func TestTemplate_Parse(t *testing.T) {
	testCases := map[string]struct {
		inPath string
		inData interface{}
		fs     func() afero.Fs

		wantedContent string
		wantedErr     error
	}{
		"template does not exist": {
			inPath: "/fake/manifest.yml",
			fs: func() afero.Fs {
				return afero.NewMemMapFs()
			},

			wantedErr: errors.New("read template /fake/manifest.yml"),
		},
		"template cannot be parsed": {
			inPath: "/fake/manifest.yml",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll("templates/fake", 0755)
				_ = afero.WriteFile(fs, "templates/fake/manifest.yml", []byte(`{{}}`), 0644)
				return fs
			},

			wantedErr: errors.New("parse template /fake/manifest.yml"),
		},
		"template cannot be executed": {
			inPath: "/fake/manifest.yml",
			inData: struct{}{},
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll("templates/fake", 0755)
				_ = afero.WriteFile(fs, "templates/fake/manifest.yml", []byte(`{{.Name}}`), 0644)
				return fs
			},

			wantedErr: fmt.Errorf("execute template %s", "/fake/manifest.yml"),
		},
		"valid template": {
			inPath: "/fake/manifest.yml",
			inData: struct {
				Name string
			}{
				Name: "webhook",
			},
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll("templates/fake", 0755)
				_ = afero.WriteFile(fs, "templates/fake/manifest.yml", []byte(`{{.Name}}`), 0644)
				return fs
			},
			wantedContent: "webhook",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{
				fs: &mockFS{Fs: tc.fs()},
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
