// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"gopkg.in/yaml.v3"

	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/spf13/afero"
)

const (
	defaultPackageManager    = "npm"
	maxNumberOfLevelsChecked = 5
)

// CDK is an Overrider that can transform a CloudFormation template with the Cloud Development Kit.
type CDK struct {
	rootAbsPath string // Absolute path to the overrides/ directory.

	execWriter io.Writer // Writer to pipe stdout and stderr content from os/exec calls.
	fs         afero.Fs  // OS file system.
	exec       struct {
		LookPath func(file string) (string, error)
		Command  func(name string, args ...string) *exec.Cmd
		Find     func(startDir string, maxLevels int, matchFn workspace.TraverseUpProcessFn) (string, error)
		Getwd    func() (dir string, err error)
	} // For testing os/exec calls.
}

// CDKOpts is optional configuration for initializing a CDK Overrider.
type CDKOpts struct {
	ExecWriter io.Writer                                   // Writer to forward stdout and stderr writes from os/exec calls. If nil default to io.Discard.
	FS         afero.Fs                                    // File system interface. If nil, defaults to the OS file system.
	EnvVars    map[string]string                           // Environment variables key value pairs to pass to the "cdk synth" command.
	LookPathFn func(executable string) (string, error)     // Search for the executable under $PATH. Defaults to exec.LookPath.
	CommandFn  func(name string, args ...string) *exec.Cmd // Create a new executable command. Defaults to exec.Command rooted at the overrides/ dir.
}

// WithCDK instantiates a new CDK Overrider with root being the path to the overrides/ directory.
func WithCDK(root string, opts CDKOpts) *CDK {
	writer := io.Discard
	if opts.ExecWriter != nil {
		writer = opts.ExecWriter
	}

	fs := afero.NewOsFs()
	if opts.FS != nil {
		fs = opts.FS
	}

	lookPathFn := exec.LookPath
	if opts.LookPathFn != nil {
		lookPathFn = opts.LookPathFn
	}

	cmdFn := func(name string, args ...string) *exec.Cmd {
		cmd := exec.Command(name, args...)
		cmd.Dir = root
		envs, idx := make([]string, len(opts.EnvVars)), 0
		for k, v := range opts.EnvVars {
			envs[idx] = fmt.Sprintf("%s=%s", k, v)
			idx += 1
		}
		cmd.Env = append(os.Environ(), envs...)
		return cmd
	}
	if opts.CommandFn != nil {
		cmdFn = opts.CommandFn
	}
	return &CDK{
		rootAbsPath: root,
		execWriter:  writer,
		fs:          fs,
		exec: struct {
			LookPath func(file string) (string, error)
			Command  func(name string, args ...string) *exec.Cmd
			Find     func(startDir string, maxLevels int, matchFn workspace.TraverseUpProcessFn) (string, error)
			Getwd    func() (dir string, err error)
		}{
			LookPath: lookPathFn,
			Command:  cmdFn,
			Find:     workspace.TraverseUp,
			Getwd:    os.Getwd,
		},
	}
}

// Override returns the extended CloudFormation template body using the CDK.
// In order to ensure the CDK transformations can be applied, Copilot first installs any CDK dependencies
// as well as the toolkit itself.
func (cdk *CDK) Override(body []byte) ([]byte, error) {
	if err := cdk.install(); err != nil {
		return nil, err
	}
	out, err := cdk.transform(body)
	if err != nil {
		return nil, err
	}
	return cdk.cleanUp(out)
}

func (cdk *CDK) install() error {
	manager, err := cdk.packageManager()
	if err != nil {
		return err
	}
	cmd := cdk.exec.Command(manager, "install")
	cmd.Stdout = cdk.execWriter
	cmd.Stderr = cdk.execWriter

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(`run %q: %w`, cmd.String(), err)
	}
	return nil
}

func (cdk *CDK) transform(body []byte) ([]byte, error) {
	buildPath := filepath.Join(cdk.rootAbsPath, ".build")
	if err := cdk.fs.MkdirAll(buildPath, 0755); err != nil {
		return nil, fmt.Errorf("create %s directory to store the CloudFormation template body: %w", buildPath, err)
	}
	inputPath := filepath.Join(buildPath, "in.yml")
	if err := afero.WriteFile(cdk.fs, inputPath, body, 0644); err != nil {
		return nil, fmt.Errorf("write CloudFormation template body content at %s: %w", inputPath, err)
	}

	// We assume that a node_modules/ dir is present with the CDK downloaded after running "npm install".
	// This way clients don't need to install the CDK toolkit separately.
	cmd := cdk.exec.Command(filepath.Join("node_modules", ".bin", "cdk"), "synth", "--no-version-reporting")
	buf := new(bytes.Buffer)
	cmd.Stdout = buf
	cmd.Stderr = cdk.execWriter
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(`run %q: %w`, cmd.String(), err)
	}
	return buf.Bytes(), nil
}

// cleanUp removes YAML additions that get injected by the CDK that are unnecessary,
// and transforms the Description string of the CloudFormation template to highlight the template is now overridden with the CDK.
func (cdk *CDK) cleanUp(in []byte) ([]byte, error) {
	// See [template anatomy]: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/template-anatomy.html
	// We ignore Rules on purpose as it's only used by the CDK.
	type template struct {
		AWSTemplateFormatVersion string               `yaml:"AWSTemplateFormatVersion,omitempty"`
		Description              string               `yaml:"Description,omitempty"`
		Metadata                 yaml.Node            `yaml:"Metadata,omitempty"`
		Parameters               map[string]yaml.Node `yaml:"Parameters,omitempty"`
		Mappings                 yaml.Node            `yaml:"Mappings,omitempty"`
		Conditions               yaml.Node            `yaml:"Conditions,omitempty"`
		Transform                yaml.Node            `yaml:"Transform,omitempty"`
		Resources                yaml.Node            `yaml:"Resources,omitempty"`
		Outputs                  yaml.Node            `yaml:"Outputs,omitempty"`
	}
	var body template
	if err := yaml.Unmarshal(in, &body); err != nil {
		return nil, fmt.Errorf("unmarsal CDK transformed YAML template: %w", err)
	}

	// Augment the description with Copilot and the CDK metrics.
	body.Description = fmt.Sprintf("%s using AWS Copilot and CDK.", strings.TrimSuffix(body.Description, "."))

	// Get rid of CDK parameters.
	delete(body.Parameters, "BootstrapVersion")

	out := new(bytes.Buffer)
	encoder := yaml.NewEncoder(out)
	encoder.SetIndent(2)
	if err := encoder.Encode(body); err != nil {
		return nil, fmt.Errorf("marshal cleaned up CDK transformed template: %w", err)
	}
	return out.Bytes(), nil
}

type packageManager struct {
	name     string
	lockFile string
}

var packageManagers = []packageManager{ // Alphabetically sorted based on name.
	{
		name:     "npm",
		lockFile: "package-lock.json",
	},
	{
		name:     "yarn",
		lockFile: "yarn.lock",
	},
}

func (cdk *CDK) installedPackageManagers() ([]string, error) {
	var installed []string
	for _, candidate := range packageManagers {
		if _, err := cdk.exec.LookPath(candidate.name); err == nil {
			installed = append(installed, candidate.name)
		} else if !errors.Is(err, exec.ErrNotFound) {
			return nil, err
		}
	}
	return installed, nil
}

// closestProjectManager returns the package manager of the project.
// It searches five levels up from the working directory and look for the lock file of each package managers.
// If no lock file is found, it returns an empty string.
// If only one lock file is found, it returns that corresponding package manager name.
// If multiple are found, it returns the package manager whose lock file is closer to the working dir.
// If multiple are equally close, then it returns the one whose name is the alphabetically smallest.
func (cdk *CDK) closestProjectManager() (string, error) {
	wd, err := cdk.exec.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	var closest string
	findLockFileFn := func(dir string) (string, error) {
		for _, candidate := range packageManagers {
			exists, err := afero.Exists(cdk.fs, filepath.Join(dir, candidate.lockFile))
			if err != nil {
				return "", err
			}
			if exists {
				closest = candidate.name
				return "", workspace.ErrTraverseUpShouldStop
			}
		}
		return "", nil
	}
	_, err = cdk.exec.Find(wd, maxNumberOfLevelsChecked, findLockFileFn)
	if err == nil {
		return closest, nil
	}
	var errTargetNotFound *workspace.ErrTargetNotFound
	if errors.As(err, &errTargetNotFound) {
		return "", nil
	}
	return "", fmt.Errorf("find a package lock file: %w", err)
}

func (cdk *CDK) packageManager() (string, error) {
	installed, err := cdk.installedPackageManagers()
	if err != nil {
		return "", err
	}
	if len(installed) == 0 {
		return "", &errPackageManagerUnavailable{}
	}
	if len(installed) == 1 {
		return installed[0], nil
	}
	manager, err := cdk.closestProjectManager()
	if err != nil {
		return "", err
	}
	if manager != "" {
		return manager, nil
	}
	return defaultPackageManager, nil
}

// ScaffoldWithCDK bootstraps a CDK application under dir/ to override the seed CloudFormation resources.
// If the directory is not empty, then returns an error.
func ScaffoldWithCDK(fs afero.Fs, dir string, seeds []template.CFNResource, requiresEnv bool) error {
	// If the directory does not exist, [afero.IsEmpty] returns false and an error.
	// Therefore, we only want to check if a directory is empty only if it also exists.
	exists, _ := afero.Exists(fs, dir)
	isEmpty, _ := afero.IsEmpty(fs, dir)
	if exists && !isEmpty {
		return fmt.Errorf("directory %q is not empty", dir)
	}

	return templates.WalkOverridesCDKDir(seeds, writeFilesToDir(dir, fs), requiresEnv)
}

func writeFilesToDir(dir string, fs afero.Fs) template.WalkDirFunc {
	return func(name string, content *template.Content) error {
		path := filepath.Join(dir, name)
		if err := fs.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("make directories along %q: %w", filepath.Dir(path), err)
		}
		if err := afero.WriteFile(fs, path, content.Bytes(), 0644); err != nil {
			return fmt.Errorf("write file at %q: %w", path, err)
		}
		return nil
	}
}
