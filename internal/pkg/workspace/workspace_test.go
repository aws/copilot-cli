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
		expectedPath   string
		workingDir     string
		mockFileSystem func(fs afero.Fs)
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
		"uses precomputed manifest path": {
			expectedPath:   workspaceDir,
			workingDir:     filepath.FromSlash("/"),
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
				workingDirAbs: tc.workingDir,
				fs:            &afero.Afero{Fs: fs},
				CopilotDirAbs: filepath.FromSlash("test/copilot"),
			}
			workspacePath := ws.Path()
			require.Equal(t, tc.expectedPath, workspacePath)
		})
	}
}

func TestWorkspace_Summary(t *testing.T) {
	testCases := map[string]struct {
		workingDir     string
		mockFileSystem func(fs afero.Fs)

		expectedSummary Summary
		expectedError   error
	}{
		"existing workspace summary": {
			workingDir: "test/",
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll("test/copilot", 0755)
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(fmt.Sprintf("---\napplication: %s", "DavidsApp")), 0644)
			},
			expectedSummary: Summary{
				Application: "DavidsApp",
				Path:        filepath.FromSlash("test/copilot/.workspace"),
			},
		},
		"existing workspace summary in a parent dir": {
			workingDir: "test/app",
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll("test/copilot", 0755)
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(fmt.Sprintf("---\napplication: %s", "DavidsApp")), 0644)
			},
			expectedSummary: Summary{
				Application: "DavidsApp",
				Path:        filepath.FromSlash("test/copilot/.workspace"),
			},
		},
		"no existing workspace summary": {
			workingDir: "test/",
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll("test/copilot", 0755)
			},
			expectedError: &ErrNoAssociatedApplication{},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Create an empty FileSystem
			fs := afero.NewMemMapFs()
			// Set it up
			tc.mockFileSystem(fs)

			ws := Workspace{
				CopilotDirAbs: filepath.Join("test", CopilotDirName),
				workingDirAbs: tc.workingDir,
				fs:            &afero.Afero{Fs: fs},
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
	wd, err := os.Getwd()
	require.NoError(t, err)
	parent := filepath.Dir(wd)
	testCases := map[string]struct {
		appName        string
		mockFileSystem func() afero.Fs

		expectedError         error
		expectedCopilotDirAbs string
	}{
		"successful no-op with existing workspace and summary": {
			appName: "DavidsApp",
			mockFileSystem: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll(fmt.Sprintf("%s/copilot", wd), 0755)
				_ = afero.WriteFile(fs, fmt.Sprintf("%s/copilot/.workspace", wd), []byte(fmt.Sprintf("---\napplication: %s", "DavidsApp")), 0644)
				fs = afero.NewReadOnlyFs(fs) // No write/
				return fs
			},
			expectedCopilotDirAbs: fmt.Sprintf("%s/copilot", wd),
		},
		"successful no-op with existing workspace and summary in a parent directory": {
			appName: "DavidsApp",
			mockFileSystem: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll(fmt.Sprintf("%s/copilot", parent), 0755)
				_ = afero.WriteFile(fs, fmt.Sprintf("%s/copilot/.workspace", parent), []byte(fmt.Sprintf("---\napplication: %s", "DavidsApp")), 0644)
				fs = afero.NewReadOnlyFs(fs) // No write.
				return fs
			},
			expectedCopilotDirAbs: fmt.Sprintf("%s/copilot", parent),
		},
		"error if workspace exists but associated with different application": {
			appName: "DavidsApp",
			mockFileSystem: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll(fmt.Sprintf("%s/copilot", wd), 0755)
				_ = afero.WriteFile(fs, fmt.Sprintf("%s/copilot/.workspace", wd), []byte(fmt.Sprintf("---\napplication: %s", "DavidsOtherApp")), 0644)
				fs = afero.NewReadOnlyFs(fs) // No write.
				return fs
			},
			expectedError: fmt.Errorf("workspace is already registered with application DavidsOtherApp under %s", filepath.FromSlash("copilot/.workspace")),
		},
		"successfully create a work summary if workspace existing but no workspace summary": {
			appName: "DavidsApp",
			mockFileSystem: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll(fmt.Sprintf("%s/copilot", wd), 0755)
				return fs
			},
			expectedCopilotDirAbs: fmt.Sprintf("%s/copilot", wd),
		},
		"successfully create both workspace and summary if neither exists": {
			appName: "DavidsApp",
			mockFileSystem: func() afero.Fs {
				return afero.NewMemMapFs()
			},
			expectedCopilotDirAbs: fmt.Sprintf("%s/copilot", wd),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Set up filesystem.
			gotWS, err := Create(tc.appName, tc.mockFileSystem())
			if tc.expectedError == nil {
				// an operation not permitted error means
				// we tried to write to the filesystem, but
				// the test indicated that we expected no writes.
				require.NoError(t, err)

				// Validate that the stored copilot dir path is correct.
				require.Equal(t, tc.expectedCopilotDirAbs, gotWS.CopilotDirAbs)

				// Validate that the workspace dir is created.
				exist, err := gotWS.fs.Exists(tc.expectedCopilotDirAbs)
				require.NoError(t, err)
				require.True(t, exist)

				// Validate that the summary file is associated with the app.
				gotSummary, err := gotWS.Summary()
				require.NoError(t, err)
				require.Equal(t, tc.appName, gotSummary.Application)

			} else {
				require.Equal(t, tc.expectedError.Error(), err.Error())
			}
		})
	}
}

func TestWorkspace_Use(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	parent := filepath.Dir(wd)
	testCases := map[string]struct {
		appName        string
		mockFileSystem func() afero.Fs

		expectedError         error
		expectedCopilotDirAbs string
	}{
		"returns the existing workspace that has the summary": {
			appName: "DavidsApp",
			mockFileSystem: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll(fmt.Sprintf("%s/copilot", wd), 0755)
				_ = afero.WriteFile(fs, fmt.Sprintf("%s/copilot/.workspace", wd), []byte(fmt.Sprintf("---\napplication: %s", "DavidsApp")), 0644)
				fs = afero.NewReadOnlyFs(fs) // No write/
				return fs
			},
			expectedCopilotDirAbs: fmt.Sprintf("%s/copilot", wd),
		},
		"returns the existing workspace and workspace summary in a parent directory": {
			appName: "DavidsApp",
			mockFileSystem: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll(fmt.Sprintf("%s/copilot", parent), 0755)
				_ = afero.WriteFile(fs, fmt.Sprintf("%s/copilot/.workspace", parent), []byte(fmt.Sprintf("---\napplication: %s", "DavidsApp")), 0644)
				fs = afero.NewReadOnlyFs(fs) // No write.
				return fs
			},
			expectedCopilotDirAbs: fmt.Sprintf("%s/copilot", parent),
		},
		"returns an ErrNoAssociatedApplication error when there is existing copilot/ directory that does not have a workspace summary": {
			appName: "DavidsApp",
			mockFileSystem: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll(fmt.Sprintf("%s/copilot", wd), 0755)
				return fs
			},
			expectedCopilotDirAbs: fmt.Sprintf("%s/copilot", wd),
			expectedError:         &ErrNoAssociatedApplication{},
		},
		"ErrWorkspaceNotFound if there is no workspace": {
			mockFileSystem: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll(fmt.Sprintf("%s/webhook", wd), 0755)
				return fs
			},
			expectedError: &ErrWorkspaceNotFound{
				&ErrTargetNotFound{
					startDir:              wd,
					numberOfLevelsChecked: maximumParentDirsToSearch,
				},
				CopilotDirName,
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Set up filesystem.
			gotWS, err := Use(tc.mockFileSystem())
			if tc.expectedError == nil {
				// an operation not permitted error means
				// we tried to write to the filesystem, but
				// the test indicated that we expected no writes.
				require.NoError(t, err)

				// Validate that the stored copilot dir path is correct.
				require.Equal(t, tc.expectedCopilotDirAbs, gotWS.CopilotDirAbs)

				// Validate that the workspace dir is there.
				exist, err := gotWS.fs.Exists(tc.expectedCopilotDirAbs)
				require.NoError(t, err)
				require.True(t, exist)

			} else {
				require.Equal(t, tc.expectedError.Error(), err.Error())
			}
		})
	}
}

func TestWorkspace_WorkloadExists(t *testing.T) {
	t.Run("returns true if workload exists in the workspace", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		_, _ = fs.Create("/copilot/api/manifest.yml")
		ws := &Workspace{
			CopilotDirAbs: "/copilot",
			fs: &afero.Afero{
				Fs: fs,
			},
		}
		got, err := ws.WorkloadExists("api")
		require.NoError(t, err)
		require.True(t, got)
	})

	t.Run("returns false if workload does not in the workspace", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		_, _ = fs.Create("a/copilot/api/manifest.yml")
		ws := &Workspace{
			CopilotDirAbs: "b/copilot",
			fs: &afero.Afero{
				Fs: fs,
			},
		}
		got, err := ws.WorkloadExists("api")
		require.NoError(t, err)
		require.False(t, got)
	})
}

func TestWorkspace_EnvironmentsExist(t *testing.T) {
	t.Run("returns true if environments are managed in the workspace", func(t *testing.T) {
		// GIVEN
		defer func() { getWd = os.Getwd }()

		getWd = func() (dir string, err error) {
			return "/copilot", nil
		}
		fs := afero.NewMemMapFs()
		_, _ = fs.Create("/copilot/environments/")
		_, _ = fs.Create("/copilot/.workspace")
		ws, err := Use(fs)

		// Then
		require.NoError(t, err)
		got, err := ws.HasEnvironments()
		require.NoError(t, err)
		require.True(t, got)
	})

	t.Run("returns false if environments are not managed in the workspace", func(t *testing.T) {
		// GIVEN
		defer func() { getWd = os.Getwd }()

		getWd = func() (dir string, err error) {
			return "b/copilot", nil
		}
		fs := afero.NewMemMapFs()
		_, _ = fs.Create("b/copilot/.workspace")
		_, _ = fs.Create("a/copilot/environments/")
		_, _ = fs.Create("a/copilot/.workspace")
		ws, err := Use(fs)

		// Then
		require.NoError(t, err)
		got, err := ws.HasEnvironments()
		require.NoError(t, err)
		require.False(t, got)
	})
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
			wantedErr: fmt.Errorf("read directory /copilot: open %s: file does not exist", filepath.FromSlash("/copilot")),
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
				CopilotDirAbs: tc.copilotDir,
				fs: &afero.Afero{
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
			wantedErr: fmt.Errorf("read directory /copilot: open %s: file does not exist", filepath.FromSlash("/copilot")),
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
				CopilotDirAbs: tc.copilotDir,
				fs: &afero.Afero{
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

func TestWorkspace_ListWorkloads(t *testing.T) {
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
				CopilotDirAbs: tc.copilotDir,
				fs: &afero.Afero{
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

func TestWorkspace_ListEnvironments(t *testing.T) {
	testCases := map[string]struct {
		copilotDir string
		fs         func() afero.Fs

		wantedNames []string
		wantedErr   error
	}{
		"environments directory does not exist": {
			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.Mkdir("/copilot", 0755)
				return fs
			},
			wantedErr: fmt.Errorf("read directory %s: open %s: file does not exist",
				filepath.FromSlash("/copilot/environments"), filepath.FromSlash("/copilot/environments")),
		},
		"retrieve only env directories with manifest files": {
			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.Mkdir("/copilot", 0755)

				// Environments.
				fs.Mkdir("/copilot/environments/test", 0755)
				fs.Create("/copilot/environments/test/manifest.yml")

				fs.Mkdir("/copilot/environments/dev", 0755)
				fs.Create("/copilot/environments/dev/manifest.yml")

				// Missing manifest.yml.
				fs.Mkdir("/copilot/environments/prod", 0755)

				// Legacy pipeline files.
				fs.Create("/copilot/buildspec.yml")
				fs.Create("/copilot/pipeline.yml")

				// Services.
				fs.Mkdir("/copilot/frontend", 0755)
				fs.Create("/copilot/frontend/manifest.yml")
				return fs
			},

			wantedNames: []string{"test", "dev"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ws := &Workspace{
				CopilotDirAbs: tc.copilotDir,
				fs: &afero.Afero{
					Fs: tc.fs(),
				},
			}

			names, err := ws.ListEnvironments()
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

		wantedPipelines []PipelineManifest
		wantedErr       error
		wantedLog       string
	}{
		"success finding legacy pipeline (copilot/pipeline.yml) and pipelines (copilot/pipelines/*/manifest.yml)": {
			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()

				fs.Mkdir("/copilot", 0755)
				fs.Create("/copilot/buildspec.yml")
				legacyInCopiDirManifest, _ := fs.Create("/copilot/pipeline.yml")
				defer legacyInCopiDirManifest.Close()
				legacyInCopiDirManifest.Write([]byte(`
name: legacyInCopiDir
version: 1
`))

				fs.Mkdir("/copilot/pipelines", 0755)
				fs.Create("/copilot/pipelines/randomFileToIgnore.yml")

				fs.Create("/copilot/pipelines/beta/buildspec.yml")
				betaPipelineManifest, _ := fs.Create("/copilot/pipelines/beta/manifest.yml")
				defer betaPipelineManifest.Close()
				betaPipelineManifest.Write([]byte(`
name: betaManifest
version: 1
`))

				fs.Create("/copilot/pipelines/prod/buildspec.yml")
				prodPipelineManifest, _ := fs.Create("/copilot/pipelines/prod/manifest.yml")
				defer prodPipelineManifest.Close()
				prodPipelineManifest.Write([]byte(`
name: prodManifest
version: 1
`))

				return fs
			},
			wantedPipelines: []PipelineManifest{
				{
					Name: "betaManifest",
					Path: filepath.FromSlash("/copilot/pipelines/beta/manifest.yml"),
				},
				{
					Name: "legacyInCopiDir",
					Path: filepath.FromSlash("/copilot/pipeline.yml"),
				},
				{
					Name: "prodManifest",
					Path: filepath.FromSlash("/copilot/pipelines/prod/manifest.yml"),
				},
			},

			wantedErr: nil,
		},
		"success finding legacy pipeline if it is the only pipeline": {
			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()

				fs.Mkdir("/copilot", 0755)
				fs.Create("/copilot/buildspec.yml")
				legacyInCopiDirManifest, _ := fs.Create("/copilot/pipeline.yml")
				defer legacyInCopiDirManifest.Close()
				legacyInCopiDirManifest.Write([]byte(`
name: legacyInCopiDir
version: 1
`))

				return fs
			},
			wantedPipelines: []PipelineManifest{
				{
					Name: "legacyInCopiDir",
					Path: filepath.FromSlash("/copilot/pipeline.yml"),
				},
			},
			wantedErr: nil,
		},
		"success finding pipelines without any legacy pipelines": {
			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()

				fs.Mkdir("/copilot", 0755)
				fs.Mkdir("/copilot/pipelines", 0755)

				fs.Create("/copilot/pipelines/beta/buildspec.yml")
				betaPipelineManifest, _ := fs.Create("/copilot/pipelines/beta/manifest.yml")
				defer betaPipelineManifest.Close()
				betaPipelineManifest.Write([]byte(`
name: betaManifest
version: 1
`))

				fs.Create("/copilot/pipelines/prod/buildspec.yml")
				prodPipelineManifest, _ := fs.Create("/copilot/pipelines/prod/manifest.yml")
				defer prodPipelineManifest.Close()
				prodPipelineManifest.Write([]byte(`
name: prodManifest
version: 1
`))

				return fs
			},
			wantedPipelines: []PipelineManifest{
				{
					Name: "betaManifest",
					Path: filepath.FromSlash("/copilot/pipelines/beta/manifest.yml"),
				},
				{
					Name: "prodManifest",
					Path: filepath.FromSlash("/copilot/pipelines/prod/manifest.yml"),
				},
			},
			wantedErr: nil,
		},
		"ignores missing manifest files": {
			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()

				fs.Mkdir("/copilot", 0755)
				fs.Mkdir("/copilot/pipelines", 0755)
				fs.Mkdir("/copilot/pipelines/beta", 0755)
				fs.Mkdir("/copilot/pipelines/prod", 0755)

				return fs
			},
			wantedPipelines: nil,
			wantedErr:       nil,
		},
		"ignores pipeline manifest with invalid version": {
			copilotDir: "/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()

				fs.Mkdir("/copilot", 0755)
				fs.Mkdir("/copilot/pipelines", 0755)
				fs.Mkdir("/copilot/pipelines/beta", 0755)
				fs.Create("/copilot/pipelines/beta/buildspec.yml")
				betaPipelineManifest, _ := fs.Create("/copilot/pipelines/beta/manifest.yml")
				defer betaPipelineManifest.Close()
				betaPipelineManifest.Write([]byte(`
name: betaManifest
version: invalidVersionShouldBe~int
`))

				fs.Mkdir("/copilot/pipelines/prod", 0755)
				fs.Create("/copilot/pipelines/prod/buildspec.yml")
				prodPipelineManifest, _ := fs.Create("/copilot/pipelines/prod/manifest.yml")
				defer prodPipelineManifest.Close()
				prodPipelineManifest.Write([]byte(`
name: prodManifest
version: 1
`))

				return fs
			},
			wantedPipelines: []PipelineManifest{
				{
					Name: "prodManifest",
					Path: filepath.FromSlash("/copilot/pipelines/prod/manifest.yml"),
				},
			},
			wantedErr: nil,
			wantedLog: fmt.Sprintf("Unable to read pipeline manifest at '%s': unmarshal pipeline manifest: yaml: unmarshal errors:\n  line 3: cannot unmarshal !!str `invalid...` into manifest.PipelineSchemaMajorVersion\n",
				filepath.FromSlash("/copilot/pipelines/beta/manifest.yml")),
		},
	}

	for name, tc := range testCases {
		var log string
		logCollector := func(format string, a ...interface{}) {
			log += fmt.Sprintf(format, a...)
		}

		t.Run(name, func(t *testing.T) {
			ws := &Workspace{
				CopilotDirAbs: tc.copilotDir,
				fs: &afero.Afero{
					Fs: tc.fs(),
				},
				logger: logCollector,
			}

			pipelines, err := ws.ListPipelines()
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedPipelines, pipelines)
			}
			require.Equal(t, tc.wantedLog, log)
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

func TestWorkspace_EnvAddonsAbsPath(t *testing.T) {
	mockWorkingDirAbs := "/app"
	testCases := map[string]struct {
		fs         func() afero.Fs
		wantedPath string
	}{
		"returns the correct env addons path": {
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/environments/addons/", 0755)
				return fs
			},
			wantedPath: "/copilot/environments/addons",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ws := &Workspace{
				CopilotDirAbs: "/copilot/",
				workingDirAbs: mockWorkingDirAbs,
				fs: &afero.Afero{
					Fs: tc.fs(),
				},
			}

			got := ws.EnvAddonsAbsPath()
			require.Equal(t, tc.wantedPath, got)
		})
	}
}

func TestWorkspace_WorkloadAddonFilePath(t *testing.T) {
	ws := &Workspace{}
	require.Equal(t, filepath.FromSlash("webhook/addons/db.yml"), ws.WorkloadAddonFilePath("webhook", "db.yml"))
}

func TestWorkspace_EnvAddonFilePath(t *testing.T) {
	ws := &Workspace{}
	require.Equal(t, filepath.FromSlash("environments/addons/db.yml"), ws.EnvAddonFilePath("db.yml"))
}

func TestWorkspace_EnvOverridesPath(t *testing.T) {
	// GIVEN
	defer func() { getWd = os.Getwd }()

	getWd = func() (dir string, err error) {
		return ".", nil
	}
	fs := afero.NewMemMapFs()
	ws, err := Create("demo", fs)

	// THEN
	require.NoError(t, err)
	require.Equal(t, filepath.Join("copilot", "environments", "overrides"), ws.EnvOverridesPath())
}

func TestWorkspace_WorkloadOverridesPath(t *testing.T) {
	// GIVEN
	defer func() { getWd = os.Getwd }()

	getWd = func() (dir string, err error) {
		return ".", nil
	}
	fs := afero.NewMemMapFs()
	ws, err := Create("demo", fs)

	// THEN
	require.NoError(t, err)
	require.Equal(t, filepath.Join("copilot", "frontend", "overrides"), ws.WorkloadOverridesPath("frontend"))
}

func TestWorkspace_EnvAddonFileAbsPath(t *testing.T) {
	mockWorkingDirAbs := "/app"
	testCases := map[string]struct {
		fName      string
		fs         func() afero.Fs
		wantedPath string
		wantedErr  error
	}{
		"returns the correct env addon file path": {
			fName: "db.yml",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/environments/addons/db.yml", 0755)
				return fs
			},
			wantedPath: "/copilot/environments/addons/db.yml",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ws := &Workspace{
				CopilotDirAbs: "/copilot",
				workingDirAbs: mockWorkingDirAbs,
				fs: &afero.Afero{
					Fs: tc.fs(),
				},
			}

			got := ws.EnvAddonFileAbsPath(tc.fName)
			require.Equal(t, tc.wantedPath, got)
		})
	}
}

func TestWorkspace_WorkloadAddonsAbsPath(t *testing.T) {
	mockWorkingDirAbs := "/app"
	testCases := map[string]struct {
		fs         func() afero.Fs
		wantedPath string
	}{
		"returns the correct workload addons path": {
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/mockSvc/addons/", 0755)
				return fs
			},
			wantedPath: "/copilot/mockSvc/addons",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ws := &Workspace{
				CopilotDirAbs: "/copilot/",
				workingDirAbs: mockWorkingDirAbs,
				fs: &afero.Afero{
					Fs: tc.fs(),
				},
			}

			got := ws.WorkloadAddonsAbsPath("mockSvc")
			require.Equal(t, tc.wantedPath, got)
		})
	}
}

func TestWorkspace_WorkloadAddonFileAbsPath(t *testing.T) {
	mockWorkingDirAbs := "/app"
	testCases := map[string]struct {
		svc        string
		fName      string
		fs         func() afero.Fs
		wantedPath string
		wantedErr  error
	}{
		"returns the correct env addon file path": {
			svc:   "webhook",
			fName: "db.yml",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/webhook/addons/db.yml", 0755)
				return fs
			},
			wantedPath: "/copilot/webhook/addons/db.yml",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ws := &Workspace{
				CopilotDirAbs: "/copilot/",
				workingDirAbs: mockWorkingDirAbs,
				fs: &afero.Afero{
					Fs: tc.fs(),
				},
			}

			got := ws.WorkloadAddonFileAbsPath(tc.svc, tc.fName)
			require.Equal(t, tc.wantedPath, got)
		})
	}
}

func TestWorkspace_ListFiles(t *testing.T) {
	testCases := map[string]struct {
		inDirPath string
		fs        func() afero.Fs

		wantedFileNames []string
		wantedErr       error
	}{
		"dir not exist": {
			inDirPath: "/copilot/webhook/addons",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/webhook/", 0755)
				return fs
			},
			wantedErr: &os.PathError{
				Op:   "open",
				Path: filepath.FromSlash("/copilot/webhook/addons"),
				Err:  os.ErrNotExist,
			},
		},
		"retrieves file names": {
			inDirPath: "/copilot/webhook/addons",
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
				CopilotDirAbs: "copilot/",
				fs: &afero.Afero{
					Fs: tc.fs(),
				},
			}

			// WHEN
			actualFileNames, actualErr := ws.ListFiles(tc.inDirPath)

			// THEN
			require.Equal(t, tc.wantedErr, actualErr)
			require.Equal(t, tc.wantedFileNames, actualFileNames)
		})
	}
}

func TestWorkspace_ReadFile(t *testing.T) {
	testCases := map[string]struct {
		fPath string
		fs    func() afero.Fs

		wantedData []byte
		wantedErr  error
	}{
		"return error if file does not exist": {
			fPath: "/copilot/api/addons/db.yml",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				return fs
			},
			wantedErr: fmt.Errorf("file %s does not exists", filepath.FromSlash("/copilot/api/addons/db.yml")),
		},
		"read existing file": {
			fPath: "/copilot/environments/addons/db.yml",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/environments/addons", 0755)
				f, _ := fs.Create("/copilot/environments/addons/db.yml")
				defer f.Close()
				f.Write([]byte("mydb"))
				return fs
			},

			wantedData: []byte("mydb"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ws := &Workspace{
				CopilotDirAbs: "/copilot",
				fs: &afero.Afero{
					Fs: tc.fs(),
				},
			}
			data, err := ws.ReadFile(tc.fPath)
			if tc.wantedErr == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantedData, data)
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}

func TestWorkspace_Write(t *testing.T) {
	testCases := map[string]struct {
		marshaler mockBinaryMarshaler
		path      string

		wantedPath string
		wantedErr  error
	}{
		"writes addons file with content": {
			marshaler: mockBinaryMarshaler{
				content: []byte("hello"),
			},
			path:       filepath.FromSlash("webhook/addons/s3.yml"),
			wantedPath: filepath.FromSlash("/copilot/webhook/addons/s3.yml"),
		},
		"wraps error if cannot marshal to binary": {
			marshaler: mockBinaryMarshaler{
				err: errors.New("some error"),
			},
			path:      filepath.FromSlash("webhook/addons/s3.yml"),
			wantedErr: errors.New("marshal binary content: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			fs := afero.NewMemMapFs()
			utils := &afero.Afero{
				Fs: fs,
			}
			utils.MkdirAll(filepath.Join("/", "copilot", "webhook"), 0755)
			ws := &Workspace{
				workingDirAbs: "/",
				CopilotDirAbs: "/copilot",
				fs:            utils,
			}

			// WHEN
			actualPath, actualErr := ws.Write(tc.marshaler, tc.path)

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

func TestWorkspace_ReadWorkloadManifest(t *testing.T) {
	const (
		mockCopilotDir   = "/copilot"
		mockWorkloadName = "webhook"
	)
	testCases := map[string]struct {
		elems  []string
		mockFS func() afero.Fs

		wantedData      WorkloadManifest
		wantedErr       error
		wantedErrPrefix string
	}{
		"return error if directory does not exist": {
			mockFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				return fs
			},
			wantedErr: fmt.Errorf("file %s does not exists", filepath.FromSlash("/copilot/webhook/manifest.yml")),
		},
		"fail to read name from manifest": {
			mockFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/webhook/", 0755)
				f, _ := fs.Create("/copilot/webhook/manifest.yml")
				defer f.Close()
				f.Write([]byte(`hello`))
				return fs
			},
			wantedErrPrefix: `unmarshal manifest file to retrieve "name"`,
		},
		"name from manifest does not match with dir": {
			mockFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/webhook/", 0755)
				f, _ := fs.Create("/copilot/webhook/manifest.yml")
				defer f.Close()
				f.Write([]byte(`name: not-webhook`))
				return fs
			},
			wantedErr: errors.New(`name of the manifest "not-webhook" and directory "webhook" do not match`),
		},
		"read existing file": {
			mockFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/webhook/", 0755)
				f, _ := fs.Create("/copilot/webhook/manifest.yml")
				defer f.Close()
				f.Write([]byte(`name: webhook
type: Load Balanced Web Service
flavor: vanilla`))
				return fs
			},

			wantedData: []byte(`name: webhook
type: Load Balanced Web Service
flavor: vanilla`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ws := &Workspace{
				CopilotDirAbs: mockCopilotDir,
				fs: &afero.Afero{
					Fs: tc.mockFS(),
				},
			}
			data, err := ws.ReadWorkloadManifest(mockWorkloadName)
			if tc.wantedErr == nil && tc.wantedErrPrefix == "" {
				require.NoError(t, err)
				require.Equal(t, tc.wantedData, data)
			}
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
			if tc.wantedErrPrefix != "" {
				require.ErrorContains(t, err, tc.wantedErrPrefix)
			}
		})
	}
}

func TestWorkspace_ReadEnvironmentManifest(t *testing.T) {
	const mockEnvironmentName = "test"

	testCases := map[string]struct {
		elems  []string
		mockFS func() afero.Fs

		wantedData      EnvironmentManifest
		wantedErr       error
		wantedErrPrefix string
	}{
		"return error if directory does not exist": {
			mockFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				return fs
			},
			wantedErr: fmt.Errorf("file %s does not exists", filepath.FromSlash("/copilot/environments/test/manifest.yml")),
		},
		"fail to read name from manifest": {
			mockFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/environments/test/", 0755)
				f, _ := fs.Create("/copilot/environments/test/manifest.yml")
				defer f.Close()
				f.Write([]byte(`hello`))
				return fs
			},
			wantedErrPrefix: `unmarshal manifest file to retrieve "name"`,
		},
		"name from manifest does not match with dir": {
			mockFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/environments/test", 0755)
				f, _ := fs.Create("/copilot/environments/test/manifest.yml")
				defer f.Close()
				f.Write([]byte(`name: not-test`))
				return fs
			},
			wantedErr: errors.New(`name of the manifest "not-test" and directory "test" do not match`),
		},
		"type from manifest is not environment": {
			mockFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/environments/test", 0755)
				f, _ := fs.Create("/copilot/environments/test/manifest.yml")
				defer f.Close()
				f.Write([]byte(`name: test
type: Load Balanced
flavor: vanilla`))
				return fs
			},
			wantedErr: errors.New(`manifest test has type of "Load Balanced", not "Environment"`),
		},
		"read existing file": {
			mockFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/copilot/environments/test/", 0755)
				f, _ := fs.Create("/copilot/environments/test/manifest.yml")
				defer f.Close()
				f.Write([]byte(`name: test
type: Environment
flavor: vanilla`))
				return fs
			},

			wantedData: []byte(`name: test
type: Environment
flavor: vanilla`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ws := &Workspace{
				CopilotDirAbs: "/copilot",
				fs: &afero.Afero{
					Fs: tc.mockFS(),
				},
			}
			data, err := ws.ReadEnvironmentManifest(mockEnvironmentName)
			if tc.wantedErr == nil && tc.wantedErrPrefix == "" {
				require.NoError(t, err)
				require.Equal(t, tc.wantedData, data)
			}
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
			if tc.wantedErrPrefix != "" {
				require.ErrorContains(t, err, tc.wantedErrPrefix)
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
				manifest, _ := fs.Create("/copilot/pipelines/my-pipeline/manifest.yml")
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
				manifest, _ := fs.Create("/copilot/pipelines/my-pipeline/manifest.yml")
				defer manifest.Close()
				manifest.Write([]byte(`
name: somePipelineName
version: 0
`))
				return fs
			},
			expectedError: errors.New("unmarshal pipeline manifest: pipeline manifest contains invalid schema version: 0"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			fs := tc.fs()
			ws := &Workspace{
				CopilotDirAbs: copilotDir,
				fs:            &afero.Afero{Fs: fs},
			}

			// WHEN
			_, err := ws.ReadPipelineManifest("/copilot/pipelines/my-pipeline/manifest.yml")

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
			copilotDir: "/path/to/copilot",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/path/to/copilot", 0755)
				fs.Create("/path/to/copilot/.workspace")
				return fs
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			// GIVEN
			fs := tc.fs()
			ws := &Workspace{
				CopilotDirAbs: tc.copilotDir,
				fs: &afero.Afero{
					Fs: fs,
				},
			}

			// WHEN
			err := ws.DeleteWorkspaceFile()

			// THEN
			require.NoError(t, err)

			// There should be no more .workspace file under the copilot/ directory.
			path := filepath.Join(tc.copilotDir, ".workspace")
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

			wantedErr: fmt.Errorf("file %s does not exists", filepath.FromSlash("/copilot/webhook/manifest.yml")),
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
				CopilotDirAbs: tc.copilotDir,
				fs: &afero.Afero{
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
			wantedPath: filepath.FromSlash("/copilot/webhook/addons/policy.yml"),
		},
		"create file under copilot directory": {
			elems:      []string{legacyPipelineFileName},
			wantedPath: filepath.FromSlash("/copilot/pipeline.yml"),
		},
		"return ErrFileExists if file already exists": {
			elems:     []string{"manifest.yml"},
			wantedErr: &ErrFileExists{FileName: filepath.FromSlash("/copilot/manifest.yml")},
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
				workingDirAbs: "/",
				CopilotDirAbs: "/copilot",
				fs:            utils,
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
