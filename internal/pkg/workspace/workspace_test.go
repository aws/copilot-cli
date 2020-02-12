// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestListManifests(t *testing.T) {
	testCases := map[string]struct {
		expectedManifests []string
		workingDir        string
		expectedError     error
		mockFileSystem    func(appFS afero.Fs)
	}{
		"manifests with extra files": {
			expectedManifests: []string{"frontend-app.yml", "backend-app.yml"},
			workingDir:        "test/",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
				afero.WriteFile(appFS, "test/ecs-project/frontend-app.yml", []byte("frontend"), 0644)
				afero.WriteFile(appFS, "test/ecs-project/backend-app.yml", []byte("backend"), 0644)
				afero.WriteFile(appFS, "test/ecs-project/not-manifest.yml", []byte("nothing"), 0644)
				afero.WriteFile(appFS, "test/ecs-project/.ecs-workspace", []byte("hiddenproject"), 0644)
			},
		},

		"no existing manifests": {
			expectedManifests: []string{},
			workingDir:        "test/",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
				afero.WriteFile(appFS, "test/ecs-project/not-manifest.yml", []byte("nothing"), 0644)
				afero.WriteFile(appFS, "test/ecs-project/.ecs-workspace", []byte("hiddenproject"), 0644)
			},
		},

		"not in a valid workspace": {
			expectedError: fmt.Errorf("couldn't find a directory called ecs-project up to 5 levels up from test/"),
			workingDir:    "test/",
			mockFileSystem: func(appFS afero.Fs) {
			},
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
			manifests, err := ws.ListManifestFiles()
			if tc.expectedError == nil {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.expectedManifests, manifests)
			} else {
				require.Equal(t, tc.expectedError.Error(), err.Error())
			}
		})
	}
}

func genApps(names ...string) []archer.Manifest {
	result := make([]archer.Manifest, 0, len(names))
	for _, name := range names {
		result = append(result, &manifest.LBFargateManifest{
			AppManifest: manifest.AppManifest{
				Name: name,
				Type: manifest.LoadBalancedWebApplication,
			},
		})
	}
	return result
}

func renderManifest(name string) string {
	const template = `
name: %s
# The "architecture" of the application you're running.
type: Load Balanced Web App
`
	return fmt.Sprintf(template, name)
}

func TestApps(t *testing.T) {
	testCases := map[string]struct {
		expectedApps   []archer.Manifest
		workingDir     string
		expectedError  error
		mockFileSystem func(appFS afero.Fs)
	}{
		"multiple local app manifests": {
			expectedApps: genApps("frontend", "backend"),
			workingDir:   "test/",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
				afero.WriteFile(appFS, "test/ecs-project/frontend-app.yml", []byte(renderManifest("frontend")), 0644)
				afero.WriteFile(appFS, "test/ecs-project/backend-app.yml", []byte(renderManifest("backend")), 0644)
				afero.WriteFile(appFS, "test/ecs-project/.ecs-workspace", []byte("hiddenproject"), 0644)
			},
		},

		"no existing app manifest": {
			expectedApps: []archer.Manifest{},
			workingDir:   "test/",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
				afero.WriteFile(appFS, "test/ecs-project/.ecs-workspace", []byte("hiddenproject"), 0644)
			},
		},

		"not in a valid workspace": {
			expectedError: fmt.Errorf("couldn't find a directory called ecs-project up to 5 levels up from test/"),
			workingDir:    "test/",
			mockFileSystem: func(appFS afero.Fs) {
			},
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

			apps, err := ws.Apps()
			if tc.expectedError == nil {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.expectedApps, apps)
			} else {
				require.Equal(t, tc.expectedError.Error(), err.Error())
			}
		})
	}
}

func TestReadManifest(t *testing.T) {
	testCases := map[string]struct {
		expectedContent string
		workingDir      string
		manifestFile    string
		expectedError   error
		mockFileSystem  func(appFS afero.Fs)
	}{
		"existing manifest": {
			manifestFile:    "frontend-app.yml",
			expectedContent: "frontend",
			workingDir:      "test/",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
				afero.WriteFile(appFS, "test/ecs-project/frontend-app.yml", []byte("frontend"), 0644)
				afero.WriteFile(appFS, "test/ecs-project/backend-app.yml", []byte("backend"), 0644)
				afero.WriteFile(appFS, "test/ecs-project/not-manifest.yml", []byte("nothing"), 0644)
				afero.WriteFile(appFS, "test/ecs-project/.ecs-workspace", []byte("hiddenproject"), 0644)
			},
		},

		"non-existent manifest": {
			manifestFile:  "traveling-salesman-app.yml",
			expectedError: fmt.Errorf("manifest file traveling-salesman-app.yml does not exists"),
			workingDir:    "test/",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
				afero.WriteFile(appFS, "test/ecs-project/frontend-app.yml", []byte("frontend"), 0644)
				afero.WriteFile(appFS, "test/ecs-project/backend-app.yml", []byte("backend"), 0644)
				afero.WriteFile(appFS, "test/ecs-project/not-manifest.yml", []byte("nothing"), 0644)
				afero.WriteFile(appFS, "test/ecs-project/.ecs-workspace", []byte("hiddenproject"), 0644)
			},
		},

		"invalid workspace": {
			manifestFile:  "frontend-app.yml",
			expectedError: fmt.Errorf("couldn't find a directory called ecs-project up to 5 levels up from /"),
			workingDir:    "/",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs-project", 0755)
				afero.WriteFile(appFS, "test/ecs-project/frontend-app.yml", []byte("frontend"), 0644)
				afero.WriteFile(appFS, "test/ecs-project/backend-app.yml", []byte("backend"), 0644)
				afero.WriteFile(appFS, "test/ecs-project/not-manifest.yml", []byte("nothing"), 0644)
				afero.WriteFile(appFS, "test/ecs-project/.ecs-workspace", []byte("hiddenproject"), 0644)
			},
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
			content, err := ws.ReadFile(tc.manifestFile)
			if tc.expectedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.expectedContent, string(content))
			} else {
				require.Equal(t, tc.expectedError.Error(), err.Error())
			}
		})
	}
}

func TestManifestDirectoryPath(t *testing.T) {
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

func TestReadSummary(t *testing.T) {
	testCases := map[string]struct {
		expectedSummary archer.WorkspaceSummary
		workingDir      string
		expectedError   error
		mockFileSystem  func(appFS afero.Fs)
	}{
		"existing workspace summary": {
			expectedSummary: archer.WorkspaceSummary{ProjectName: "DavidsProject"},
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

func TestCreate(t *testing.T) {
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
			elems:      []string{PipelineFileName},
			wantedPath: "/ecs-project/pipeline.yml",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			fs := afero.NewMemMapFs()
			fs.MkdirAll("/ecs-project", 0755)
			ws := &Workspace{
				workingDir: "/",
				projectDir: "/ecs-project",
				fsUtils: &afero.Afero{
					Fs: fs,
				},
			}

			// WHEN
			actualPath, actualErr := ws.Write(nil, tc.elems...)

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
