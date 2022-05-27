// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package customresource

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
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

	// ensure custom resource names match.
	actualFnNames := make([]string, len(crs))
	for i, cr := range crs {
		actualFnNames[i] = cr.FunctionName()
	}
	require.ElementsMatch(t, []string{"CustomDomainFunction", "EnvControllerFunction"}, actualFnNames, "function names must match")

	// ensure the zip files contain an index.js file.
	for _, cr := range crs {
		zipFile, err := cr.Zip()
		require.NoError(t, err, "zipping the contents should not err")

		buf := new(bytes.Buffer)
		size, err := buf.ReadFrom(zipFile)
		require.NoError(t, err)
		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), size)
		require.NoError(t, err)

		_, err = r.Open("index.js")
		require.NoError(t, err, "an index.js file must be present in all custom resources")
	}
}

func TestLBWS(t *testing.T) {
	// GIVEN
	fakeFS := &fakeTemplateReader{
		files: map[string]*template.Content{
			"custom-resources/desired-count-delegation.js": {
				Buffer: bytes.NewBufferString("custom domain app runner"),
			},
			"custom-resources/env-controller.js": {
				Buffer: bytes.NewBufferString("env controller"),
			},
			"custom-resources/alb-rule-priority-generator.js": {
				Buffer: bytes.NewBufferString("rule priority"),
			},
			"custom-resources/nlb-custom-domain.js": {
				Buffer: bytes.NewBufferString("nlb custom domain"),
			},
			"custom-resources/nlb-cert-validator.js": {
				Buffer: bytes.NewBufferString("nlb cert"),
			},
		},
	}

	// WHEN
	crs, err := LBWS(fakeFS)

	// THEN
	require.NoError(t, err)
	require.Equal(t, fakeFS.matchCount, 5, "expected path calls do not match")

	actualFnNames := make([]string, len(crs))
	for i, cr := range crs {
		actualFnNames[i] = cr.FunctionName()
	}
	require.ElementsMatch(t,
		[]string{"DynamicDesiredCountFunction", "EnvControllerFunction", "RulePriorityFunction", "NLBCustomDomainFunction", "NLBCertValidatorFunction"},
		actualFnNames, "function names must match")

	// ensure the zip files contain an index.js file.
	for _, cr := range crs {
		zipFile, err := cr.Zip()
		require.NoError(t, err, "zipping the contents should not err")

		buf := new(bytes.Buffer)
		size, err := buf.ReadFrom(zipFile)
		require.NoError(t, err)
		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), size)
		require.NoError(t, err)

		_, err = r.Open("index.js")
		require.NoError(t, err, "an index.js file must be present in all custom resources")
	}
}

func TestWorker(t *testing.T) {
	// GIVEN
	fakeFS := &fakeTemplateReader{
		files: map[string]*template.Content{
			"custom-resources/desired-count-delegation.js": {
				Buffer: bytes.NewBufferString("custom domain app runner"),
			},
			"custom-resources/backlog-per-task-calculator.js": {
				Buffer: bytes.NewBufferString("backlog calc"),
			},
			"custom-resources/env-controller.js": {
				Buffer: bytes.NewBufferString("env controller"),
			},
		},
	}

	// WHEN
	crs, err := Worker(fakeFS)

	// THEN
	require.NoError(t, err)
	require.Equal(t, fakeFS.matchCount, 3, "expected path calls do not match")

	actualFnNames := make([]string, len(crs))
	for i, cr := range crs {
		actualFnNames[i] = cr.FunctionName()
	}
	require.ElementsMatch(t,
		[]string{"DynamicDesiredCountFunction", "BacklogPerTaskCalculatorFunction", "EnvControllerFunction"},
		actualFnNames, "function names must match")

	// ensure the zip files contain an index.js file.
	for _, cr := range crs {
		zipFile, err := cr.Zip()
		require.NoError(t, err, "zipping the contents should not err")

		buf := new(bytes.Buffer)
		size, err := buf.ReadFrom(zipFile)
		require.NoError(t, err)
		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), size)
		require.NoError(t, err)

		_, err = r.Open("index.js")
		require.NoError(t, err, "an index.js file must be present in all custom resources")
	}
}

func TestBackend(t *testing.T) {
	// GIVEN
	fakeFS := &fakeTemplateReader{
		files: map[string]*template.Content{
			"custom-resources/desired-count-delegation.js": {
				Buffer: bytes.NewBufferString("custom domain app runner"),
			},
			"custom-resources/alb-rule-priority-generator.js": {
				Buffer: bytes.NewBufferString("rule priority"),
			},
			"custom-resources/env-controller.js": {
				Buffer: bytes.NewBufferString("env controller"),
			},
		},
	}

	// WHEN
	crs, err := Backend(fakeFS)

	// THEN
	require.NoError(t, err)
	require.Equal(t, fakeFS.matchCount, 3, "expected path calls do not match")

	actualFnNames := make([]string, len(crs))
	for i, cr := range crs {
		actualFnNames[i] = cr.FunctionName()
	}
	require.ElementsMatch(t,
		[]string{"DynamicDesiredCountFunction", "RulePriorityFunction", "EnvControllerFunction"},
		actualFnNames, "function names must match")

	// ensure the zip files contain an index.js file.
	for _, cr := range crs {
		zipFile, err := cr.Zip()
		require.NoError(t, err, "zipping the contents should not err")

		buf := new(bytes.Buffer)
		size, err := buf.ReadFrom(zipFile)
		require.NoError(t, err)
		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), size)
		require.NoError(t, err)

		_, err = r.Open("index.js")
		require.NoError(t, err, "an index.js file must be present in all custom resources")
	}
}

func TestScheduledJob(t *testing.T) {
	// GIVEN
	fakeFS := &fakeTemplateReader{
		files: map[string]*template.Content{
			"custom-resources/env-controller.js": {
				Buffer: bytes.NewBufferString("env controller"),
			},
		},
	}

	// WHEN
	crs, err := ScheduledJob(fakeFS)

	// THEN
	require.NoError(t, err)
	require.Equal(t, fakeFS.matchCount, 1, "expected path calls do not match")

	actualFnNames := make([]string, len(crs))
	for i, cr := range crs {
		actualFnNames[i] = cr.FunctionName()
	}
	require.ElementsMatch(t,
		[]string{"EnvControllerFunction"},
		actualFnNames, "function names must match")

	// ensure the zip files contain an index.js file.
	for _, cr := range crs {
		zipFile, err := cr.Zip()
		require.NoError(t, err, "zipping the contents should not err")

		buf := new(bytes.Buffer)
		size, err := buf.ReadFrom(zipFile)
		require.NoError(t, err)
		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), size)
		require.NoError(t, err)

		_, err = r.Open("index.js")
		require.NoError(t, err, "an index.js file must be present in all custom resources")
	}
}

func TestEnv(t *testing.T) {
	// GIVEN
	fakeFS := &fakeTemplateReader{
		files: map[string]*template.Content{
			"custom-resources/dns-cert-validator.js": {
				Buffer: bytes.NewBufferString("cert validator"),
			},
			"custom-resources/custom-domain.js": {
				Buffer: bytes.NewBufferString("custom domain"),
			},
			"custom-resources/dns-delegation.js": {
				Buffer: bytes.NewBufferString("dns delegation"),
			},
		},
	}

	// WHEN
	crs, err := Env(fakeFS)

	// THEN
	require.NoError(t, err)
	require.Equal(t, fakeFS.matchCount, 3, "expected path calls do not match")

	actualFnNames := make([]string, len(crs))
	for i, cr := range crs {
		actualFnNames[i] = cr.FunctionName()
	}
	require.ElementsMatch(t,
		[]string{"CertificateValidationFunction", "CustomDomainFunction", "DNSDelegationFunction"},
		actualFnNames, "function names must match")

	// ensure the zip files contain an index.js file.
	for _, cr := range crs {
		zipFile, err := cr.Zip()
		require.NoError(t, err, "zipping the contents should not err")

		buf := new(bytes.Buffer)
		size, err := buf.ReadFrom(zipFile)
		require.NoError(t, err)
		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), size)
		require.NoError(t, err)

		_, err = r.Open("index.js")
		require.NoError(t, err, "an index.js file must be present in all custom resources")
	}
}

type fakeS3 struct {
	objects map[string]string
	err     error
}

func (f *fakeS3) UploadFunc() func(string, io.Reader) (string, error) {
	return func(key string, dat io.Reader) (url string, err error) {
		if f.err != nil {
			return "", f.err
		}
		url, ok := f.objects[key]
		if !ok {
			return "", fmt.Errorf("key %q does not exist in fakeS3", key)
		}
		return url, nil
	}
}

func TestUpload(t *testing.T) {
	testCases := map[string]struct {
		s3  *fakeS3
		crs []CustomResource

		wantedURLs map[string]string
		wantedErr  error
	}{
		"should return a wrapped error if a custom resource cannot be uploaded": {
			s3: &fakeS3{
				err: errors.New("some err"),
			},
			crs: []CustomResource{
				{
					name: "fn1",
				},
			},
			wantedErr: errors.New(`upload custom resource "fn1": some err`),
		},
		"should zip and upload all custom resources": {
			s3: &fakeS3{
				objects: map[string]string{
					"manual/scripts/custom-resources/func1/5443a001ec68131761e20b0896fe49ade55c4162adf61ede27daa208b8fb150d.zip": "url1",
					"manual/scripts/custom-resources/func2/18ef4a5e530a7a52d95d5426e41a4fc0c2bcd1b1febaf19cd05b324d07ef5547.zip": "url2",
				},
			},
			crs: []CustomResource{
				{
					name: "Func1",
					files: []file{
						{
							name:    "hello.js",
							content: []byte("hello"),
						},
						{
							name:    "world.js",
							content: []byte("world"),
						},
					},
				},
				{
					name: "Func2",
					files: []file{
						{
							name:    "index.js",
							content: []byte("some code"),
						},
					},
				},
			},

			wantedURLs: map[string]string{
				"Func1": "url1",
				"Func2": "url2",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			urls, err := Upload(tc.s3.UploadFunc(), tc.crs)
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error(), "errors do not match")
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedURLs, urls)
			}
		})
	}
}
