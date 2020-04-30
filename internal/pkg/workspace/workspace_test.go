// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestWorkspace_copilotDirPath(t *testing.T) {
	// turn "test/copilot" into a platform-dependent path
	var manifestDir = filepath.FromSlash("test/copilot")

	testCases := map[string]struct {
		expectedManifestDir string
		presetManifestDir   string
		workingDir          string
		expectedError       error
		mockFileSystem      func(appFS afero.Fs)
	}{
		"same directory level": {
			expectedManifestDir: manifestDir,
			workingDir:          filepath.FromSlash("test/"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/copilot", 0755)
			},
		},

		"same directory": {
			expectedManifestDir: manifestDir,
			workingDir:          filepath.FromSlash("test/copilot"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/copilot", 0755)
			},
		},

		"several levels deep": {
			expectedManifestDir: manifestDir,
			workingDir:          filepath.FromSlash("test/1/2/3/4"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/copilot", 0755)
				appFS.MkdirAll("test/1/2/3/4", 0755)
			},
		},

		"too many levels deep": {
			expectedError: fmt.Errorf("couldn't find a directory called copilot up to 5 levels up from " + filepath.FromSlash("test/1/2/3/4/5")),
			workingDir:    filepath.FromSlash("test/1/2/3/4/5"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/copilot", 0755)
				appFS.MkdirAll("test/1/2/3/4/5", 0755)
			},
		},

		"out of a workspace": {
			expectedError: fmt.Errorf("couldn't find a directory called copilot up to 5 levels up from " + filepath.FromSlash("/")),
			workingDir:    filepath.FromSlash("/"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/copilot", 0755)
			},
		},

		"uses precomputed manifest path": {
			expectedManifestDir: manifestDir,
			workingDir:          filepath.FromSlash("/"),
			mockFileSystem:      func(appFS afero.Fs) {},
			presetManifestDir:   filepath.FromSlash("test/copilot"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Create an empty FileSystem
			appFS := afero.NewMemMapFs()
			// Set it up
			tc.mockFileSystem(appFS)

			ws := Workspace{
				workingDir: tc.workingDir,
				fsUtils:    &afero.Afero{Fs: appFS},
				copilotDir: tc.presetManifestDir,
			}
			manifestDirPath, err := ws.copilotDirPath()
			if tc.expectedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.expectedManifestDir, manifestDirPath)
			} else {
				require.Equal(t, tc.expectedError.Error(), err.Error())
			}
		})
	}
}

func TestWorkspace_Summary(t *testing.T) {
	testCases := map[string]struct {
		expectedSummary Summary
		workingDir      string
		expectedError   error
		mockFileSystem  func(appFS afero.Fs)
	}{
		"existing workspace summary": {
			expectedSummary: Summary{Application: "DavidsApp"},
			workingDir:      "test/",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/copilot", 0755)
				afero.WriteFile(appFS, "test/copilot/.workspace", []byte(fmt.Sprintf("---\napplication: %s", "DavidsApp")), 0644)
			},
		},
		"no existing workspace summary": {
			workingDir:    "test/",
			expectedError: fmt.Errorf("couldn't find an application associated with this workspace"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/copilot", 0755)
			},
		},
		"no existing manifest dir": {
			workingDir:     "test/",
			expectedError:  fmt.Errorf("couldn't find a directory called copilot up to 5 levels up from test/"),
			mockFileSystem: func(appFS afero.Fs) {},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Create an empty FileSystem
			appFS := afero.NewMemMapFs()
			// Set it up
			tc.mockFileSystem(appFS)

			ws := Workspace{
				workingDir: tc.workingDir,
				fsUtils:    &afero.Afero{Fs: appFS},
			}
			summary, err := ws.Summary()
			if tc.expectedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.expectedSummary, *summary)
			} else {
				require.Equal(t, tc.expectedError.Error(), err.Error())
			}
		})
	}
}

func TestWorkspace_Create(t *testing.T) {
	testCases := map[string]struct {
		appName        string
		workingDir     string
		expectedError  error
		expectNoWrites bool
		mockFileSystem func(appFS afero.Fs)
	}{
		"existing workspace and workspace summary": {
			workingDir:     "test/",
			appName:        "DavidsApp",
			expectNoWrites: true,
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/copilot", 0755)
				afero.WriteFile(appFS, "test/copilot/.workspace", []byte(fmt.Sprintf("---\napplication: %s", "DavidsApp")), 0644)
			},
		},
		"existing workspace and workspace summary in different directory": {
			workingDir:     "test/app/",
			appName:        "DavidsApp",
			expectNoWrites: true,
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/copilot", 0755)
				appFS.MkdirAll("test/app", 0755)
				afero.WriteFile(appFS, "test/copilot/.workspace", []byte(fmt.Sprintf("---\napplication: %s", "DavidsApp")), 0644)
			},
		},
		"existing workspace and different application": {
			workingDir:    "test/",
			appName:       "DavidsApp",
			expectedError: fmt.Errorf("this workspace is already registered with application DavidsOtherApp"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/copilot", 0755)
				afero.WriteFile(appFS, "test/copilot/.workspace", []byte(fmt.Sprintf("---\napplication: %s", "DavidsOtherApp")), 0644)
			},
		},
		"existing workspace but no workspace summary": {
			workingDir: "test/",
			appName:    "DavidsApp",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/copilot", 0755)
			},
		},
		"no existing workspace or workspace summary": {
			workingDir:     "test/",
			appName:        "DavidsApp",
			mockFileSystem: func(appFS afero.Fs) {},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Create an empty FileSystem
			appFS := afero.NewMemMapFs()
			// Set it up
			tc.mockFileSystem(appFS)
			// Throw an error if someone tries to write
			// if we expect there to be no writes.
			if tc.expectNoWrites {
				appFS = afero.NewReadOnlyFs(appFS)
			}

			ws := Workspace{
				workingDir: tc.workingDir,
				fsUtils:    &afero.Afero{Fs: appFS},
			}
			err := ws.Create(tc.appName)
			if tc.expectedError == nil {
				// an operation not permitted error means
				// we tried to write to the filesystem, but
				// the test indicated that we expected no writes.
				require.NoError(t, err)
				summary, err := ws.Summary()
				require.NoError(t, err)
				require.Equal(t, tc.appName, summary.Application)
			} else {
				require.Equal(t, tc.expectedError.Error(), err.Error())
			}
		})
	}
}

func TestWorkspace_DeleteAll(t *testing.T) {
	t.Run("should delete the folder", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		err := fs.Mkdir(CopilotDirName, 0755)
		require.NoError(t, err)
		ws := Workspace{
			fsUtils: &afero.Afero{
				Fs: fs,
			},
		}

		got := ws.DeleteAll()

		require.NoError(t, got)
	})
}

func TestWorkspace_ServiceNames(t *testing.T) {
	testCases := map[string]struct {
		copilotDir string
		fs         func() afero.Fs

		wantedNames []string
		wantedErr   error
	}{
		"read not-existing directory": {
			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				return fs
			},
			wantedErr: errors.New("read directory /copilot: open /copilot: file does not exist"),
		},
		"retrieve only directories with manifest files": {
			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.Mkdir("/copilot", 0755)
				fs.Create("/copilot/buildspec.yml")

				// Valid service directory structure.
				fs.Mkdir("/copilot/users", 0755)
				fs.Create("/copilot/users/manifest.yml")

				// Valid service directory structure.
				fs.MkdirAll("/copilot/payments/addons", 0755)
				fs.Create("/copilot/payments/manifest.yml")

				// Missing manifest.yml.
				fs.Mkdir("/copilot/inventory", 0755)
				return fs
			},
			wantedNames: []string{"users", "payments"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ws := &Workspace{
				copilotDir: tc.copilotDir,
				fsUtils: &afero.Afero{
					Fs: tc.fs(),
				},
			}

			names, err := ws.ServiceNames()
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.ElementsMatch(t, tc.wantedNames, names)
			}
		})
	}
}

func TestWorkspace_read(t *testing.T) {
	testCases := map[string]struct {
		elems []string

		copilotDir string
		fs         func() afero.Fs

		wantedData []byte
	}{
		"read existing file": {
			elems: []string{"webhook", "manifest.yml"},

			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/webhook/", 0755)
				f, _ := fs.Create("/copilot/webhook/manifest.yml")
				defer f.Close()
				f.Write([]byte("hello"))
				return fs
			},

			wantedData: []byte("hello"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ws := &Workspace{
				copilotDir: tc.copilotDir,
				fsUtils: &afero.Afero{
					Fs: tc.fs(),
				},
			}

			data, err := ws.read(tc.elems...)

			require.NoError(t, err)
			require.Equal(t, tc.wantedData, data)
		})
	}
}

func TestWorkspace_write(t *testing.T) {
	testCases := map[string]struct {
		elems []string

		wantedPath string
		wantedErr  error
	}{
		"create file under nested directories": {
			elems:      []string{"webhook", "addons", "policy.yml"},
			wantedPath: "/copilot/webhook/addons/policy.yml",
		},
		"create file under project directory": {
			elems:      []string{pipelineFileName},
			wantedPath: "/copilot/pipeline.yml",
		},
		"return ErrFileExists if file already exists": {
			elems:     []string{"manifest.yml"},
			wantedErr: &ErrFileExists{FileName: "/copilot/manifest.yml"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			fs := afero.NewMemMapFs()
			utils := &afero.Afero{
				Fs: fs,
			}
			utils.MkdirAll("/copilot", 0755)
			utils.WriteFile("/copilot/manifest.yml", []byte{}, 0644)
			ws := &Workspace{
				workingDir: "/",
				copilotDir: "/copilot",
				fsUtils:    utils,
			}

			// WHEN
			actualPath, actualErr := ws.write(nil, tc.elems...)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, actualErr, tc.wantedErr.Error(), "expected the same error")
			} else {
				require.Equal(t, tc.wantedPath, actualPath, "expected the same path")
			}
		})
	}
}

func TestWorkspace_DeleteService(t *testing.T) {
	testCases := map[string]struct {
		name string

		copilotDir string
		fs         func() afero.Fs
	}{
		"deletes existing app": {
			name: "webhook",

			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/webhook", 0755)
				manifest, _ := fs.Create("/copilot/webhook/manifest.yml")
				defer manifest.Close()
				manifest.Write([]byte("hello"))
				return fs
			},
		},
		"deletes an non-existing app": {
			name: "webhook",

			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot", 0755)
				return fs
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			fs := tc.fs()
			ws := &Workspace{
				copilotDir: tc.copilotDir,
				fsUtils: &afero.Afero{
					Fs: fs,
				},
			}

			// WHEN
			err := ws.DeleteService(tc.name)

			// THEN
			require.NoError(t, err)

			// There should be no more directory under the copilot/ directory.
			path := filepath.Join(tc.copilotDir, tc.name)
			_, existErr := fs.Stat(path)
			expectedErr := &os.PathError{
				Op:   "open",
				Path: path,
				Err:  os.ErrNotExist,
			}
			require.EqualError(t, existErr, expectedErr.Error())
		})
	}
}

func TestWorkspace_DeletePipelineManifest(t *testing.T) {
	copilotDir := "/ecs-project"
	testCases := map[string]struct {
		fs            func() afero.Fs
		expectedError error
	}{
		"deletes existing pipeline manifest": {
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.Mkdir(copilotDir, 0755)
				manifest, _ := fs.Create("/copilot/pipeline.yml")
				defer manifest.Close()
				manifest.Write([]byte("hello"))
				return fs
			},
			expectedError: nil,
		},
		"when no pipeline file exists": {
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.Mkdir(copilotDir, 0755)
				return fs
			},
			expectedError: &os.PathError{
				Op:   "remove",
				Path: filepath.Join(copilotDir, "pipeline.yml"),
				Err:  os.ErrNotExist,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			fs := tc.fs()
			ws := &Workspace{
				copilotDir: copilotDir,
				fsUtils:    &afero.Afero{Fs: fs},
			}

			// WHEN
			err := ws.DeletePipelineManifest()

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			}

			// There should be no pipeline manifest in the project
			// directory.
			if err == nil {
				path := filepath.Join(copilotDir, "pipeline.yml")
				_, existErr := fs.Stat(path)
				expectedErr := &os.PathError{
					Op:   "open",
					Path: path,
					Err:  os.ErrNotExist,
				}
				require.EqualError(t, existErr, expectedErr.Error())
			}
		})
	}
}

func TestWorkspace_ReadAddonsDir(t *testing.T) {
	testCases := map[string]struct {
		appName        string
		copilotDirPath string
		fs             func() afero.Fs

		wantedFileNames []string
		wantedErr       error
	}{
		"dir not exist": {
			appName:        "webhook",
			copilotDirPath: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/webhook", 0755)
				return fs
			},
			wantedErr: &os.PathError{
				Op:   "open",
				Path: "/copilot/webhook/addons",
				Err:  os.ErrNotExist,
			},
		},
		"retrieves file names": {
			appName:        "webhook",
			copilotDirPath: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/webhook/addons", 0755)
				params, _ := fs.Create("/copilot/webhook/addons/params.yml")
				outputs, _ := fs.Create("/copilot/webhook/addons/outputs.yml")
				defer params.Close()
				defer outputs.Close()
				return fs
			},
			wantedFileNames: []string{"outputs.yml", "params.yml"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ws := &Workspace{
				copilotDir: tc.copilotDirPath,
				fsUtils: &afero.Afero{
					Fs: tc.fs(),
				},
			}

			// WHEN
			actualFileNames, actualErr := ws.ReadAddonsDir(tc.appName)

			// THEN
			require.Equal(t, tc.wantedErr, actualErr)
			require.Equal(t, tc.wantedFileNames, actualFileNames)
		})
	}
}

func TestWorkspace_ReadPipelineManifest(t *testing.T) {
	projectDir := "/copilot"
	testCases := map[string]struct {
		fs            func() afero.Fs
		expectedError error
	}{
		"reads existing pipeline manifest": {
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot", 0755)
				manifest, _ := fs.Create("/copilot/pipeline.yml")
				defer manifest.Close()
				manifest.Write([]byte("hello"))
				return fs
			},
			expectedError: nil,
		},

		"when no pipeline file exists": {
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.Mkdir(projectDir, 0755)
				return fs
			},
			expectedError: ErrNoPipelineInWorkspace,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			fs := tc.fs()
			ws := &Workspace{
				copilotDir: projectDir,
				fsUtils:    &afero.Afero{Fs: fs},
			}

			// WHEN
			_, err := ws.ReadPipelineManifest()

			// THEN
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
