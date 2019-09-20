// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/archer"
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
				appFS.MkdirAll("test/ecs", 0755)
				afero.WriteFile(appFS, "test/ecs/frontend-app.yml", []byte("frontend"), 0644)
				afero.WriteFile(appFS, "test/ecs/backend-app.yml", []byte("backend"), 0644)
				afero.WriteFile(appFS, "test/ecs/not-manifest.yml", []byte("nothing"), 0644)
				afero.WriteFile(appFS, "test/ecs/.project", []byte("hiddenproject"), 0644)
			},
		},

		"no existing manifests": {
			expectedManifests: []string{},
			workingDir:        "test/",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs", 0755)
				afero.WriteFile(appFS, "test/ecs/not-manifest.yml", []byte("nothing"), 0644)
				afero.WriteFile(appFS, "test/ecs/.project", []byte("hiddenproject"), 0644)
			},
		},

		"not in a valid workspace": {
			expectedError: fmt.Errorf("couldn't find a directory called ecs up to 5 levels up from test/"),
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

			ws := Service{
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
				appFS.MkdirAll("test/ecs", 0755)
				afero.WriteFile(appFS, "test/ecs/frontend-app.yml", []byte("frontend"), 0644)
				afero.WriteFile(appFS, "test/ecs/backend-app.yml", []byte("backend"), 0644)
				afero.WriteFile(appFS, "test/ecs/not-manifest.yml", []byte("nothing"), 0644)
				afero.WriteFile(appFS, "test/ecs/.project", []byte("hiddenproject"), 0644)
			},
		},

		"non-existent manifest": {
			manifestFile:  "traveling-salesman-app.yml",
			expectedError: fmt.Errorf("manifest file traveling-salesman-app.yml does not exists"),
			workingDir:    "test/",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs", 0755)
				afero.WriteFile(appFS, "test/ecs/frontend-app.yml", []byte("frontend"), 0644)
				afero.WriteFile(appFS, "test/ecs/backend-app.yml", []byte("backend"), 0644)
				afero.WriteFile(appFS, "test/ecs/not-manifest.yml", []byte("nothing"), 0644)
				afero.WriteFile(appFS, "test/ecs/.project", []byte("hiddenproject"), 0644)
			},
		},

		"invalid workspace": {
			manifestFile:  "frontend-app.yml",
			expectedError: fmt.Errorf("couldn't find a directory called ecs up to 5 levels up from /"),
			workingDir:    "/",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs", 0755)
				afero.WriteFile(appFS, "test/ecs/frontend-app.yml", []byte("frontend"), 0644)
				afero.WriteFile(appFS, "test/ecs/backend-app.yml", []byte("backend"), 0644)
				afero.WriteFile(appFS, "test/ecs/not-manifest.yml", []byte("nothing"), 0644)
				afero.WriteFile(appFS, "test/ecs/.project", []byte("hiddenproject"), 0644)
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Create an empty FileSystem
			appFS := afero.NewMemMapFs()
			// Set it up
			tc.mockFileSystem(appFS)

			ws := Service{
				workingDir: tc.workingDir,
				fsUtils:    &afero.Afero{Fs: appFS},
			}
			content, err := ws.ReadManifestFile(tc.manifestFile)
			if tc.expectedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.expectedContent, string(content))
			} else {
				require.Equal(t, tc.expectedError.Error(), err.Error())
			}
		})
	}
}

func TestWriteManifest(t *testing.T) {
	testCases := map[string]struct {
		expectedContent string
		manifestFile    string
		appName         string
		workingDir      string
		expectedError   error
		mockFileSystem  func(appFS afero.Fs)
	}{
		"new content": {
			manifestFile:    "frontend-app.yml",
			appName:         "frontend",
			expectedContent: "frontend",
			workingDir:      "test/",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs", 0755)
			},
		},
		"no manifest dir": {
			manifestFile:  "frontend-app.yml",
			appName:       "frontend",
			expectedError: fmt.Errorf("couldn't find a directory called ecs up to 5 levels up from /"),
			workingDir:    "/",
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

			ws := Service{
				workingDir: tc.workingDir,
				fsUtils:    &afero.Afero{Fs: appFS},
			}
			err := ws.WriteManifest([]byte(tc.expectedContent), tc.appName)
			if tc.expectedError == nil {
				require.NoError(t, err)
				readContent, err := ws.ReadManifestFile(tc.manifestFile)
				require.NoError(t, err)
				require.Equal(t, tc.expectedContent, string(readContent))
			} else {
				require.Equal(t, tc.expectedError.Error(), err.Error())
			}
		})
	}
}

func TestManifestDirectoryPath(t *testing.T) {
	// turn "test/ecs" into a platform-dependent path
	var manifestDir = filepath.FromSlash("test/ecs")

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
				appFS.MkdirAll("test/ecs", 0755)
			},
		},

		"same directory": {
			expectedManifestDir: manifestDir,
			workingDir:          filepath.FromSlash("test/ecs"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs", 0755)
			},
		},

		"several levels deep": {
			expectedManifestDir: manifestDir,
			workingDir:          filepath.FromSlash("test/1/2/3/4"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs", 0755)
				appFS.MkdirAll("test/1/2/3/4", 0755)
			},
		},

		"too many levels deep": {
			expectedError: fmt.Errorf("couldn't find a directory called ecs up to 5 levels up from " + filepath.FromSlash("test/1/2/3/4/5")),
			workingDir:    filepath.FromSlash("test/1/2/3/4/5"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs", 0755)
				appFS.MkdirAll("test/1/2/3/4/5", 0755)
			},
		},

		"out of a workspace": {
			expectedError: fmt.Errorf("couldn't find a directory called ecs up to 5 levels up from " + filepath.FromSlash("/")),
			workingDir:    filepath.FromSlash("/"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs", 0755)
			},
		},

		"uses precomputed manifest path": {
			expectedManifestDir: manifestDir,
			workingDir:          filepath.FromSlash("/"),
			mockFileSystem:      func(appFS afero.Fs) {},
			presetManifestDir:   filepath.FromSlash("test/ecs"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Create an empty FileSystem
			appFS := afero.NewMemMapFs()
			// Set it up
			tc.mockFileSystem(appFS)

			ws := Service{
				workingDir:  tc.workingDir,
				fsUtils:     &afero.Afero{Fs: appFS},
				manifestDir: tc.presetManifestDir,
			}
			manifestDirPath, err := ws.manifestDirectoryPath()
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
				appFS.MkdirAll("test/ecs", 0755)
				afero.WriteFile(appFS, "test/ecs/.project", []byte(fmt.Sprintf("---\nproject: %s", "DavidsProject")), 0644)
			},
		},
		"no existing workspace summary": {
			workingDir:    "test/",
			expectedError: fmt.Errorf("couldn't find a project associated with this workspace"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs", 0755)
			},
		},
		"no existing manifest dir": {
			workingDir:     "test/",
			expectedError:  fmt.Errorf("couldn't find a directory called ecs up to 5 levels up from test/"),
			mockFileSystem: func(appFS afero.Fs) {},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Create an empty FileSystem
			appFS := afero.NewMemMapFs()
			// Set it up
			tc.mockFileSystem(appFS)

			ws := Service{
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
		mockFileSystem func(appFS afero.Fs)
	}{
		"existing workspace and workspace summary": {
			workingDir:  "test/",
			projectName: "DavidsProject",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs", 0755)
				afero.WriteFile(appFS, "test/ecs/.project", []byte(fmt.Sprintf("---\nproject: %s", "DavidsProject")), 0644)
			},
		},
		"existing workspace and different project": {
			workingDir:    "test/",
			projectName:   "DavidsProject",
			expectedError: fmt.Errorf("this workspace is already registered with project DavidsOtherProject"),
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs", 0755)
				afero.WriteFile(appFS, "test/ecs/.project", []byte(fmt.Sprintf("---\nproject: %s", "DavidsOtherProject")), 0644)
			},
		},
		"existing workspace but no workspace summary": {
			workingDir:  "test/",
			projectName: "DavidsProject",
			mockFileSystem: func(appFS afero.Fs) {
				appFS.MkdirAll("test/ecs", 0755)
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

			ws := Service{
				workingDir: tc.workingDir,
				fsUtils:    &afero.Afero{Fs: appFS},
			}
			err := ws.Create(tc.projectName)
			if tc.expectedError == nil {
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
