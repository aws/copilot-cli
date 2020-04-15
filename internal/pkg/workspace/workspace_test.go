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

func TestWorkspace_projectDirPath(t *testing.T) {
	// turn "test/ecs-project" into a platform-dependent path
	var manifestDir = filepath.FromSlash("test/ecs-project")

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
				appFS.MkdirAll("test/ecs-project", 0755)
			},
		},

		"same directory": {
			expectedManifestDir: manifestDir,
			workingDir:          filepath.FromSlash("test/ecs-project"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
			},
		},

		"several levels deep": {
			expectedManifestDir: manifestDir,
			workingDir:          filepath.FromSlash("test/1/2/3/4"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
				appFS.MkdirAll("test/1/2/3/4", 0755)
			},
		},

		"too many levels deep": {
			expectedError: fmt.Errorf("couldn't find a directory called ecs-project up to 5 levels up from " + filepath.FromSlash("test/1/2/3/4/5")),
			workingDir:    filepath.FromSlash("test/1/2/3/4/5"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
				appFS.MkdirAll("test/1/2/3/4/5", 0755)
			},
		},

		"out of a workspace": {
			expectedError: fmt.Errorf("couldn't find a directory called ecs-project up to 5 levels up from " + filepath.FromSlash("/")),
			workingDir:    filepath.FromSlash("/"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
			},
		},

		"uses precomputed manifest path": {
			expectedManifestDir: manifestDir,
			workingDir:          filepath.FromSlash("/"),
			mockFileSystem:      func(appFS afero.Fs) {},
			presetManifestDir:   filepath.FromSlash("test/ecs-project"),
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
				projectDir: tc.presetManifestDir,
			}
			manifestDirPath, err := ws.projectDirPath()
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
			expectedSummary: Summary{ProjectName: "DavidsProject"},
			workingDir:      "test/",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
				afero.WriteFile(appFS, "test/ecs-project/.ecs-workspace", []byte(fmt.Sprintf("---\nproject: %s", "DavidsProject")), 0644)
			},
		},
		"no existing workspace summary": {
			workingDir:    "test/",
			expectedError: fmt.Errorf("couldn't find a project associated with this workspace"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
			},
		},
		"no existing manifest dir": {
			workingDir:     "test/",
			expectedError:  fmt.Errorf("couldn't find a directory called ecs-project up to 5 levels up from test/"),
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
		projectName    string
		workingDir     string
		expectedError  error
		expectNoWrites bool
		mockFileSystem func(appFS afero.Fs)
	}{
		"existing workspace and workspace summary": {
			workingDir:     "test/",
			projectName:    "DavidsProject",
			expectNoWrites: true,
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
				afero.WriteFile(appFS, "test/ecs-project/.ecs-workspace", []byte(fmt.Sprintf("---\nproject: %s", "DavidsProject")), 0644)
			},
		},
		"existing workspace and workspace summary in different directory": {
			workingDir:     "test/app/",
			projectName:    "DavidsProject",
			expectNoWrites: true,
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
				appFS.MkdirAll("test/app", 0755)
				afero.WriteFile(appFS, "test/ecs-project/.ecs-workspace", []byte(fmt.Sprintf("---\nproject: %s", "DavidsProject")), 0644)
			},
		},
		"existing workspace and different project": {
			workingDir:    "test/",
			projectName:   "DavidsProject",
			expectedError: fmt.Errorf("this workspace is already registered with project DavidsOtherProject"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
				afero.WriteFile(appFS, "test/ecs-project/.ecs-workspace", []byte(fmt.Sprintf("---\nproject: %s", "DavidsOtherProject")), 0644)
			},
		},
		"existing workspace but no workspace summary": {
			workingDir:  "test/",
			projectName: "DavidsProject",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
			},
		},
		"no existing workspace or workspace summary": {
			workingDir:     "test/",
			projectName:    "DavidsProject",
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
			err := ws.Create(tc.projectName)
			if tc.expectedError == nil {
				// an operation not permitted error means
				// we tried to write to the filesystem, but
				// the test indicated that we expected no writes.
				require.NoError(t, err)
				project, err := ws.Summary()
				require.NoError(t, err)
				require.Equal(t, tc.projectName, project.ProjectName)
			} else {
				require.Equal(t, tc.expectedError.Error(), err.Error())
			}
		})
	}
}

func TestWorkspace_DeleteAll(t *testing.T) {
	t.Run("should delete the folder", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		err := fs.Mkdir(ProjectDirectoryName, 0755)
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

func TestWorkspace_AppNames(t *testing.T) {
	testCases := map[string]struct {
		projectDir string
		fs         func() afero.Fs

		wantedNames []string
		wantedErr   error
	}{
		"read not-existing directory": {
			projectDir: "/ecs-project",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				return fs
			},
			wantedErr: errors.New("read directory /ecs-project: open /ecs-project: file does not exist"),
		},
		"retrieve only directories with manifest files": {
			projectDir: "/ecs-project",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.Mkdir("/ecs-project", 0755)
				fs.Create("/ecs-project/buildspec.yml")

				// Valid app directory structure.
				fs.Mkdir("/ecs-project/users", 0755)
				fs.Create("/ecs-project/users/manifest.yml")

				// Valid app directory structure.
				fs.MkdirAll("/ecs-project/payments/addons", 0755)
				fs.Create("/ecs-project/payments/manifest.yml")

				// Missing manifest.yml.
				fs.Mkdir("/ecs-project/inventory", 0755)
				return fs
			},
			wantedNames: []string{"users", "payments"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ws := &Workspace{
				projectDir: tc.projectDir,
				fsUtils: &afero.Afero{
					Fs: tc.fs(),
				},
			}

			names, err := ws.AppNames()
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.ElementsMatch(t, tc.wantedNames, names)
			}
		})
	}
}

func TestWorkspace_Read(t *testing.T) {
	testCases := map[string]struct {
		elems []string

		projectDir string
		fs         func() afero.Fs

		wantedData []byte
	}{
		"read existing file": {
			elems: []string{"webhook", "manifest.yml"},

			projectDir: "/ecs-project",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/ecs-project/webhook/", 0755)
				f, _ := fs.Create("/ecs-project/webhook/manifest.yml")
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
				projectDir: tc.projectDir,
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

func TestWorkspace_Write(t *testing.T) {
	testCases := map[string]struct {
		elems []string

		wantedPath string
		wantedErr  error
	}{
		"create file under nested directories": {
			elems:      []string{"webhook", "addons", "policy.yml"},
			wantedPath: "/ecs-project/webhook/addons/policy.yml",
		},
		"create file under project directory": {
			elems:      []string{pipelineFileName},
			wantedPath: "/ecs-project/pipeline.yml",
		},
		"return ErrFileExists if file already exists": {
			elems:     []string{"manifest.yml"},
			wantedErr: &ErrFileExists{FileName: "/ecs-project/manifest.yml"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			fs := afero.NewMemMapFs()
			utils := &afero.Afero{
				Fs: fs,
			}
			utils.MkdirAll("/ecs-project", 0755)
			utils.WriteFile("/ecs-project/manifest.yml", []byte{}, 0644)
			ws := &Workspace{
				workingDir: "/",
				projectDir: "/ecs-project",
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

func TestWorkspace_DeleteApp(t *testing.T) {
	testCases := map[string]struct {
		name string

		projectDir string
		fs         func() afero.Fs
	}{
		"deletes existing app": {
			name: "webhook",

			projectDir: "/ecs-project",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/ecs-project/webhook", 0755)
				manifest, _ := fs.Create("/ecs-project/webhook/manifest.yml")
				defer manifest.Close()
				manifest.Write([]byte("hello"))
				return fs
			},
		},
		"deletes an non-existing app": {
			name: "webhook",

			projectDir: "/ecs-project",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/ecs-project", 0755)
				return fs
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			fs := tc.fs()
			ws := &Workspace{
				projectDir: tc.projectDir,
				fsUtils: &afero.Afero{
					Fs: fs,
				},
			}

			// WHEN
			err := ws.DeleteApp(tc.name)

			// THEN
			require.NoError(t, err)

			// There should be no more directory under the project directory.
			path := filepath.Join(tc.projectDir, tc.name)
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

func TestWorkspace_DeletePipeline(t *testing.T) {
	projectDir := "/ecs-project"
	testCases := map[string]struct {
		fs            func() afero.Fs
		expectedError error
	}{
		"deletes existing pipeline manifest": {
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.Mkdir(projectDir, 0755)
				manifest, _ := fs.Create("/ecs-project/pipeline.yml")
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
			expectedError: &os.PathError{
				Op:   "remove",
				Path: filepath.Join(projectDir, "pipeline.yml"),
				Err:  os.ErrNotExist,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			fs := tc.fs()
			ws := &Workspace{
				projectDir: projectDir,
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
				path := filepath.Join(projectDir, "pipeline.yml")
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
		projectDirPath string
		fs             func() afero.Fs

		wantedFileNames []string
		wantedErr       error
	}{
		"dir not exist": {
			appName:        "webhook",
			projectDirPath: "/ecs-project",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/ecs-project/webhook", 0755)
				return fs
			},
			wantedErr: &os.PathError{
				Op:   "open",
				Path: "/ecs-project/webhook/addons",
				Err:  os.ErrNotExist,
			},
		},
		"retrieves file names": {
			appName:        "webhook",
			projectDirPath: "/ecs-project",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/ecs-project/webhook/addons", 0755)
				params, _ := fs.Create("/ecs-project/webhook/addons/params.yml")
				outputs, _ := fs.Create("/ecs-project/webhook/addons/outputs.yml")
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
				projectDir: tc.projectDirPath,
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
	projectDir := "/ecs-project"
	testCases := map[string]struct {
		fs            func() afero.Fs
		expectedError error
	}{
		"reads existing pipeline manifest": {
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/ecs-project", 0755)
				manifest, _ := fs.Create("/ecs-project/pipeline.yml")
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
				projectDir: projectDir,
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
