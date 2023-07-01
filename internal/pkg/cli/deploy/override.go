// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/override"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/spf13/afero"
)

// An Overrider transforms the content in body to out.
type Overrider interface {
	Override(body []byte) (out []byte, err error)
}

// UserAgentAdder is the interface that adds values to the User-Agent HTTP request header.
type UserAgentAdder interface {
	UserAgentExtras(extras ...string)
}

// NewOverrider looks up if a CDK or YAMLPatch Overrider exists at pathsToOverriderDir and initializes the respective Overrider.
// If the directory is empty, then returns a noop Overrider.
func NewOverrider(pathToOverridesDir, app, env string, fs afero.Fs, sess UserAgentAdder) (Overrider, error) {
	info, err := override.Lookup(pathToOverridesDir, fs)
	if err != nil {
		var errNotExist *override.ErrNotExist
		if errors.As(err, &errNotExist) {
			return new(override.Noop), nil
		}
		return nil, fmt.Errorf("look up overrider at %q: %w", pathToOverridesDir, err)
	}
	switch {
	case info.IsCDK():
		sess.UserAgentExtras("override cdk")
		// Write out-of-band info from sub-commands to stderr as users expect stdout to only
		// contain the final override output.
		opts := override.CDKOpts{
			FS:         fs,
			ExecWriter: log.DiagnosticWriter,
			EnvVars: map[string]string{
				"COPILOT_APPLICATION_NAME": app,
			},
		}
		if env != "" {
			opts.EnvVars["COPILOT_ENVIRONMENT_NAME"] = env
		}

		return override.WithCDK(info.Path(), opts), nil
	case info.IsYAMLPatch():
		sess.UserAgentExtras("override yamlpatch")
		return override.WithPatch(info.Path(), override.PatchOpts{
			FS: fs,
		}), nil
	default:
		return new(override.Noop), nil
	}
}
