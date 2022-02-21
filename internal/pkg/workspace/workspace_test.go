// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// mockBinaryMarshaler implements the encoding.BinaryMarshaler interface.
type mockBinaryMarshaler struct {
	content []byte
	err     error
}

func (m mockBinaryMarshaler) MarshalBinary() (data []byte, err error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.content, nil
}

func TestWorkspace_Path(t *testing.T) {
	const workspaceDir = "test"

	testCases := map[string]struct {
		expectedPath      string
		presetManifestDir string
		workingDir        string
		expectedError     error
		mockFileSystem    func(fs afero.Fs)
	}{
		"same directory level": {
			expectedPath: workspaceDir,
			workingDir:   filepath.FromSlash("test/"),
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll("test/copilot", 0755)
			},
		},

		"same directory": {
			expectedPath: workspaceDir,
			workingDir:   filepath.FromSlash("test/copilot"),
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll("test/copilot", 0755)
			},
		},

		"several levels deep": {
			expectedPath: workspaceDir,
			workingDir:   filepath.FromSlash("test/1/2/3/4"),
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll("test/copilot", 0755)
				fs.MkdirAll("test/1/2/3/4", 0755)
			},
		},

		"too many levels deep": {
			expectedError: fmt.Errorf("couldn't find a directory called copilot up to 5 levels up from " + filepath.FromSlash("test/1/2/3/4/5")),
			workingDir:    filepath.FromSlash("test/1/2/3/4/5"),
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll("test/copilot", 0755)
				fs.MkdirAll("test/1/2/3/4/5", 0755)
			},
		},

		"out of a workspace": {
			expectedError: fmt.Errorf("couldn't find a directory called copilot up to 5 levels up from " + filepath.FromSlash("/")),
			workingDir:    filepath.FromSlash("/"),
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll("test/copilot", 0755)
			},
		},

		"uses precomputed manifest path": {
			expectedPath:      workspaceDir,
			workingDir:        filepath.FromSlash("/"),
			mockFileSystem:    func(fs afero.Fs) {},
			presetManifestDir: filepath.FromSlash("test/copilot"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Create an empty FileSystem
			fs := afero.NewMemMapFs()
			// Set it up
			tc.mockFileSystem(fs)

			ws := Workspace{
				workingDir: tc.workingDir,
				fsUtils:    &afero.Afero{Fs: fs},
				copilotDir: tc.presetManifestDir,
			}
			workspacePath, err := ws.Path()
			if tc.expectedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.expectedPath, workspacePath)
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
		mockFileSystem  func(fs afero.Fs)
	}{
		"existing workspace summary": {
			expectedSummary: Summary{
				Application: "DavidsApp",
				Path:        "test/copilot/.workspace",
			},
			workingDir: "test/",
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll("test/copilot", 0755)
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(fmt.Sprintf("---\napplication: %s", "DavidsApp")), 0644)
			},
		},
		"no existing workspace summary": {
			workingDir:    "test/",
			expectedError: fmt.Errorf("couldn't find an application associated with this workspace"),
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll("test/copilot", 0755)
			},
		},
		"no existing manifest dir": {
			workingDir:     "test/",
			expectedError:  fmt.Errorf("couldn't find a directory called copilot up to 5 levels up from test/"),
			mockFileSystem: func(fs afero.Fs) {},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Create an empty FileSystem
			fs := afero.NewMemMapFs()
			// Set it up
			tc.mockFileSystem(fs)

			ws := Workspace{
				workingDir: tc.workingDir,
				fsUtils:    &afero.Afero{Fs: fs},
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
		mockFileSystem func(fs afero.Fs)
	}{
		"existing workspace and workspace summary": {
			workingDir:     "test/",
			appName:        "DavidsApp",
			expectNoWrites: true,
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll("test/copilot", 0755)
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(fmt.Sprintf("---\napplication: %s", "DavidsApp")), 0644)
			},
		},
		"existing workspace and workspace summary in different directory": {
			workingDir:     "test/app/",
			appName:        "DavidsApp",
			expectNoWrites: true,
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll("test/copilot", 0755)
				fs.MkdirAll("test/app", 0755)
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(fmt.Sprintf("---\napplication: %s", "DavidsApp")), 0644)
			},
		},
		"existing workspace and different application": {
			workingDir:    "test/",
			appName:       "DavidsApp",
			expectedError: fmt.Errorf("workspace is already registered with application DavidsOtherApp under copilot/.workspace"),
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll("test/copilot", 0755)
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(fmt.Sprintf("---\napplication: %s", "DavidsOtherApp")), 0644)
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
			mockFileSystem: func(fs afero.Fs) {},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Create an empty FileSystem
			fs := afero.NewMemMapFs()
			// Set it up
			tc.mockFileSystem(fs)
			// Throw an error if someone tries to write
			// if we expect there to be no writes.
			if tc.expectNoWrites {
				fs = afero.NewReadOnlyFs(fs)
			}

			ws := Workspace{
				workingDir: tc.workingDir,
				fsUtils:    &afero.Afero{Fs: fs},
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

func TestWorkspace_ListServices(t *testing.T) {
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
		"return error if directory name and manifest name do not match": {
			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.Mkdir("/copilot", 0755)
				fs.Create("/copilot/buildspec.yml")

				fs.Mkdir("/copilot/users", 0755)
				manifest, _ := fs.Create("/copilot/users/manifest.yml")
				defer manifest.Close()
				manifest.Write([]byte(`name: payment
type: Load Balanced Web Service`))

				// Missing manifest.yml.
				fs.Mkdir("/copilot/inventory", 0755)
				return fs
			},

			wantedErr: fmt.Errorf(`read manifest for workload users: name of the manifest "payment" and directory "users" do not match`),
		},
		"retrieve only directories with manifest files": {
			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.Mkdir("/copilot", 0755)
				fs.Create("/copilot/buildspec.yml")

				// Valid service directory structure.
				fs.Mkdir("/copilot/users", 0755)
				manifest, _ := fs.Create("/copilot/users/manifest.yml")
				defer manifest.Close()
				manifest.Write([]byte(`name: users
type: Load Balanced Web Service`))

				// Valid service directory structure.
				fs.MkdirAll("/copilot/payments/addons", 0755)
				manifest2, _ := fs.Create("/copilot/payments/manifest.yml")
				defer manifest2.Close()
				manifest2.Write([]byte(`name: payments
type: Load Balanced Web Service`))

				// Missing manifest.yml.
				fs.Mkdir("/copilot/inventory", 0755)
				return fs
			},

			wantedNames: []string{"users", "payments"},
		},
		"retrieve only workload names of the correct type": {
			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.Mkdir("/copilot", 0755)
				fs.Create("/copilot/buildspec.yml")

				// Valid service directory structure.
				fs.Mkdir("/copilot/users", 0755)
				manifest, _ := fs.Create("/copilot/users/manifest.yml")
				defer manifest.Close()
				manifest.Write([]byte(`name: users
type: Scheduled Job`))

				// Valid service directory structure.
				fs.MkdirAll("/copilot/payments/addons", 0755)
				manifest2, _ := fs.Create("/copilot/payments/manifest.yml")
				defer manifest2.Close()
				manifest2.Write([]byte(`name: payments
type: Load Balanced Web Service`))

				// Missing manifest.yml.
				fs.Mkdir("/copilot/inventory", 0755)
				return fs
			},

			wantedNames: []string{"payments"},
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

			names, err := ws.ListServices()
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.wantedNames, names)
			}
		})
	}
}

func TestWorkspace_ListJobs(t *testing.T) {
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
				manifest, _ := fs.Create("/copilot/users/manifest.yml")
				defer manifest.Close()
				manifest.Write([]byte(`name: users
type: Scheduled Job`))

				// Valid service directory structure.
				fs.MkdirAll("/copilot/payments/addons", 0755)
				manifest2, _ := fs.Create("/copilot/payments/manifest.yml")
				defer manifest2.Close()
				manifest2.Write([]byte(`name: payments
type: Scheduled Job`))

				// Missing manifest.yml.
				fs.Mkdir("/copilot/inventory", 0755)
				return fs
			},

			wantedNames: []string{"users", "payments"},
		},
		"retrieve only workload names of the correct type": {
			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.Mkdir("/copilot", 0755)
				fs.Create("/copilot/buildspec.yml")

				// Valid service directory structure.
				fs.Mkdir("/copilot/users", 0755)
				manifest, _ := fs.Create("/copilot/users/manifest.yml")
				defer manifest.Close()
				manifest.Write([]byte(`name: users
type: Scheduled Job`))

				// Valid service directory structure.
				fs.MkdirAll("/copilot/payments/addons", 0755)
				manifest2, _ := fs.Create("/copilot/payments/manifest.yml")
				defer manifest2.Close()
				manifest2.Write([]byte(`name: payments
type: Load Balanced Web Service`))

				// Missing manifest.yml.
				fs.Mkdir("/copilot/inventory", 0755)
				return fs
			},

			wantedNames: []string{"users"},
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

			names, err := ws.ListJobs()
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.ElementsMatch(t, tc.wantedNames, names)
			}
		})
	}
}

func TestWorkspace_ListWorkspaces(t *testing.T) {
	testCases := map[string]struct {
		copilotDir string
		fs         func() afero.Fs

		wantedNames []string
		wantedErr   error
	}{
		"retrieve only directories with manifest files": {
			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.Mkdir("/copilot", 0755)
				fs.Create("/copilot/buildspec.yml")
				fs.Create("/copilot/pipeline.yml")

				fs.Mkdir("/copilot/frontend", 0755)
				frontendManifest, _ := fs.Create("/copilot/frontend/manifest.yml")
				defer frontendManifest.Close()
				frontendManifest.Write([]byte(`name: frontend
type: Load Balanced Web Service`))

				fs.Mkdir("/copilot/users", 0755)
				userManifest, _ := fs.Create("/copilot/users/manifest.yml")
				defer userManifest.Close()
				userManifest.Write([]byte(`name: users
type: Backend Service`))

				fs.MkdirAll("/copilot/report/addons", 0755)
				reportManifest, _ := fs.Create("/copilot/report/manifest.yml")
				defer reportManifest.Close()
				reportManifest.Write([]byte(`name: report
type: Scheduled Job`))

				// Missing manifest.yml.
				fs.Mkdir("/copilot/inventory", 0755)
				return fs
			},

			wantedNames: []string{"frontend", "users", "report"},
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

			names, err := ws.ListWorkloads()
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.ElementsMatch(t, tc.wantedNames, names)
			}
		})
	}
}

func TestWorkspace_ListPipelines(t *testing.T) {
	testCases := map[string]struct {
		copilotDir string
		fs         func() afero.Fs

		wantedPipelines []Pipeline
		wantedErr       error
	}{
		"success finding legacy pipeline (in both copilot/ and copilot/pipelines) and other pipelines, weeding out buildspecs": {
			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.Mkdir("/copilot", 0755)
				legacyInCopiDirManifest, _ := fs.Create("/copilot/pipeline.yml")
				defer legacyInCopiDirManifest.Close()
				legacyInCopiDirManifest.Write([]byte(`
name: legacyInCopiDir
version: 1
`))

				fs.Create("/copilot/buildspec.yml")

				fs.Mkdir("/copilot/pipelines", 0755)
				legacyInPipelinesDirManifest, _ := fs.Create("/copilot/pipelines/pipeline.yml")
				defer legacyInPipelinesDirManifest.Close()
				legacyInPipelinesDirManifest.Write([]byte(`
name: legacyInPipelinesDir
version: 1
`))

				fs.Create("/copilot/pipelines/buildspec.yml")

				otherInPipelinesDirManifest, _ := fs.Create("/copilot/pipelines/other.yml")
				defer otherInPipelinesDirManifest.Close()
				otherInPipelinesDirManifest.Write([]byte(`
name: otherInPipelinesDir
version: 1
`))

				otherInPipelinesDirBuildspec, _ := fs.Create("/copilot/pipelines/other.buildspec.yml")
				defer otherInPipelinesDirBuildspec.Close()
				otherInPipelinesDirBuildspec.Write([]byte(`
name: buildspecInPipelinesDir
`))

				return fs
			},

			wantedPipelines: []Pipeline{
				{
					Name: "legacyInCopiDir",
					Path: "/copilot/pipeline.yml",
				},
				{
					Name: "otherInPipelinesDir",
					Path: "/copilot/pipelines/other.yml",
				},
				{
					Name: "legacyInPipelinesDir",
					Path: "/copilot/pipelines/pipeline.yml",
				},
			},

			wantedErr: nil,
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

			pipelines, err := ws.ListPipelines()
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedPipelines, pipelines)
			}
		})
	}
}

func TestIsInGitRepository(t *testing.T) {
	testCases := map[string]struct {
		given  func() FileStat
		wanted bool
	}{
		"return false if directory does not contain a .git directory": {
			given: func() FileStat {
				fs := afero.NewMemMapFs()
				return fs
			},
			wanted: false,
		},
		"return true if directory has a .git directory": {
			given: func() FileStat {
				fs := afero.NewMemMapFs()
				fs.MkdirAll(".git", 0755)
				return fs
			},
			wanted: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			fs := tc.given()

			actual := IsInGitRepository(fs)

			require.Equal(t, tc.wanted, actual)
		})
	}
}

func TestWorkspace_ReadAddonsDir(t *testing.T) {
	testCases := map[string]struct {
		svcName        string
		copilotDirPath string
		fs             func() afero.Fs

		wantedFileNames []string
		wantedErr       error
	}{
		"dir not exist": {
			svcName:        "webhook",
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
			svcName:        "webhook",
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
			actualFileNames, actualErr := ws.ReadAddonsDir(tc.svcName)

			// THEN
			require.Equal(t, tc.wantedErr, actualErr)
			require.Equal(t, tc.wantedFileNames, actualFileNames)
		})
	}
}

func TestWorkspace_WriteAddon(t *testing.T) {
	testCases := map[string]struct {
		marshaler   mockBinaryMarshaler
		svc         string
		storageName string

		wantedPath string
		wantedErr  error
	}{
		"writes addons file with content": {
			marshaler: mockBinaryMarshaler{
				content: []byte("hello"),
			},
			svc:         "webhook",
			storageName: "s3",

			wantedPath: "/copilot/webhook/addons/s3.yml",
		},
		"wraps error if cannot marshal to binary": {
			marshaler: mockBinaryMarshaler{
				err: errors.New("some error"),
			},
			svc:         "webhook",
			storageName: "s3",

			wantedErr: errors.New("marshal binary addon content: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			fs := afero.NewMemMapFs()
			utils := &afero.Afero{
				Fs: fs,
			}
			utils.MkdirAll(filepath.Join("/", "copilot", tc.svc), 0755)
			ws := &Workspace{
				workingDir: "/",
				copilotDir: "/copilot",
				fsUtils:    utils,
			}

			// WHEN
			actualPath, actualErr := ws.WriteAddon(tc.marshaler, tc.svc, tc.storageName)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, actualErr, tc.wantedErr.Error(), "expected the same error")
			} else {
				require.Equal(t, tc.wantedPath, actualPath, "expected the same path")
				out, err := utils.ReadFile(tc.wantedPath)
				require.NoError(t, err)
				require.Equal(t, tc.marshaler.content, out, "expected the contents of the file to match")
			}
		})
	}
}

func TestWorkspace_ReadPipelineManifest(t *testing.T) {
	copilotDir := "/copilot"
	testCases := map[string]struct {
		fs            func() afero.Fs
		expectedError error
	}{
		"reads existing pipeline manifest": {
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll(copilotDir, 0755)
				manifest, _ := fs.Create("/copilot/pipeline.yml")
				defer manifest.Close()
				manifest.Write([]byte(`
name: somePipelineName
version: 1
`))
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
			expectedError: ErrNoPipelineInWorkspace,
		},
		"error unmarshaling pipeline manifest": {
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll(copilotDir, 0755)
				manifest, _ := fs.Create("/copilot/pipeline.yml")
				defer manifest.Close()
				manifest.Write([]byte(`
name: somePipelineName
version: 0
`))
				return fs
			},
			expectedError: errors.New("unmarshal pipeline manifest: pipeline.yml contains invalid schema version: 0"),
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
			_, err := ws.ReadPipelineManifest("/copilot/pipeline.yml")

			// THEN
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWorkspace_DeleteWorkspaceFile(t *testing.T) {
	testCases := map[string]struct {
		copilotDir string
		fs         func() afero.Fs
	}{
		".workspace should be deleted": {
			copilotDir: "copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot", 0755)
				fs.Create("/copilot/.workspace")
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
			ws.fsUtils.MkdirAll("copilot", 0755)
			ws.fsUtils.Create(tc.copilotDir + "/" + ".workspace")

			// WHEN
			err := ws.DeleteWorkspaceFile()

			// THEN
			require.NoError(t, err)

			// There should be no more .workspace file under the copilot/ directory.
			path := filepath.Join(tc.copilotDir, "/.workspace")
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

func TestWorkspace_ListDockerfiles(t *testing.T) {
	wantedDockerfiles := []string{"./Dockerfile", "backend/Dockerfile", "frontend/Dockerfile"}
	testCases := map[string]struct {
		mockFileSystem func(mockFS afero.Fs)
		err            error
		dockerfiles    []string
	}{
		"find Dockerfiles": {
			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("frontend", 0755)
				mockFS.MkdirAll("backend", 0755)

				afero.WriteFile(mockFS, "Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "frontend/Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "backend/Dockerfile", []byte("FROM nginx"), 0644)
			},
			err:         nil,
			dockerfiles: wantedDockerfiles,
		},
		"exclude dockerignore files": {
			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("frontend", 0755)
				mockFS.MkdirAll("backend", 0755)

				afero.WriteFile(mockFS, "Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "frontend/Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "frontend/Dockerfile.dockerignore", []byte("*/temp*"), 0644)
				afero.WriteFile(mockFS, "backend/Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "backend/Dockerfile.dockerignore", []byte("*/temp*"), 0644)
			},
			err:         nil,
			dockerfiles: wantedDockerfiles,
		},
		"nonstandard Dockerfile names": {
			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("frontend", 0755)
				mockFS.MkdirAll("dockerfiles", 0755)
				afero.WriteFile(mockFS, "Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "frontend/dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "Job.dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "Job.dockerfile.dockerignore", []byte("*/temp*"), 0644)
			},
			err:         nil,
			dockerfiles: []string{"./Dockerfile", "./Job.dockerfile", "frontend/dockerfile"},
		},
		"no Dockerfiles": {
			mockFileSystem: func(mockFS afero.Fs) {},
			dockerfiles:    []string{},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			fs := &afero.Afero{Fs: afero.NewMemMapFs()}
			tc.mockFileSystem(fs)
			ws := &Workspace{
				workingDir: "/",
				copilotDir: "copilot",
				fsUtils: &afero.Afero{
					Fs: fs,
				},
			}
			got, err := ws.ListDockerfiles()

			if tc.err != nil {
				require.EqualError(t, err, tc.err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.dockerfiles, got)
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
		wantedErr  error
	}{
		"return error if file does not exist": {
			elems: []string{"webhook", "manifest.yml"},

			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				return fs
			},

			wantedErr: fmt.Errorf("file /copilot/webhook/manifest.yml does not exists"),
		},
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

			if tc.wantedErr == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantedData, data)
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
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
		"create file under copilot directory": {
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
