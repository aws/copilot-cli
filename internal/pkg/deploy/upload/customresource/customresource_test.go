// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package customresource

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/template"
)

type fakeTemplateReader struct {
	files map[string]*template.Content

	matchCount int
}

func (fr *fakeTemplateReader) Read(path string) (*template.Content, error) {
	content, ok := fr.files[path]
	if !ok {
		return nil, fmt.Errorf("unexpected read %s", path)
	}
	fr.matchCount += 1
	return content, nil
}

func TestRDWS(t *testing.T) {
	// GIVEN
	fakeFS := &fakeTemplateReader{
		files: map[string]*template.Content{
			"custom-resources/custom-domain-app-runner.js": {
				Buffer: bytes.NewBufferString("custom domain app runner"),
			},
			"custom-resources/env-controller.js": {
				Buffer: bytes.NewBufferString("env controller"),
			},
		},
	}

	// WHEN
	crs, err := RDWS(fakeFS)

	// THEN
	require.NoError(t, err)
	require.Equal(t, fakeFS.matchCount, 2, "expected path calls do not match")

	actualFnNames := make([]string, len(crs))
	for i, cr := range crs {
		actualFnNames[i] = cr.FunctionName()
	}
	require.ElementsMatch(t, []string{"CustomDomainFunction", "EnvControllerFunction"}, actualFnNames, "function names must match")

	for _, cr := range crs {
		require.Equal(t, 1, len(cr.Files()), "expected only a single index.js file to be zipped for the custom resource")
		require.Equal(t, "index.js", cr.Files()[0].Name())
	}
}
