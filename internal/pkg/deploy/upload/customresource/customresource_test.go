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
	fakePaths := map[string]string{
		"CustomDomainFunction":  "manual/scripts/custom-resources/customdomainfunction/2611784f21e91e499306dac066aae5fd8f2ba664b38073bdd3198d2e041c076e.zip",
		"EnvControllerFunction": "manual/scripts/custom-resources/envcontrollerfunction/72297cacaeab3a267e371c17ea3f0235905b0da51410eb31c10f7c66ba944044.zip",
	}

	// WHEN
	crs, err := RDWS(fakeFS)

	// THEN
	require.NoError(t, err)
	require.Equal(t, fakeFS.matchCount, 2, "expected path calls do not match")

	// ensure custom resource names match.
	actualFnNames := make([]string, len(crs))
	for i, cr := range crs {
		actualFnNames[i] = cr.Name()
	}
	require.ElementsMatch(t, []string{"CustomDomainFunction", "EnvControllerFunction"}, actualFnNames, "function names must match")

	// ensure the zip files contain an index.js file.
	for _, cr := range crs {
		buf := new(bytes.Buffer)
		size, err := buf.ReadFrom(cr.zipReader())
		require.NoError(t, err)
		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), size)
		require.NoError(t, err)

		_, err = r.Open("index.js")
		require.NoError(t, err, "an index.js file must be present in all custom resources")
	}

	// ensure artifact paths match.
	for _, cr := range crs {
		require.Equal(t, fakePaths[cr.Name()], cr.ArtifactPath())
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
			"custom-resources/wkld-custom-domain.js": {
				Buffer: bytes.NewBufferString("service-level custom domain"),
			},
			"custom-resources/wkld-cert-validator.js": {
				Buffer: bytes.NewBufferString("service-level cert"),
			},
		},
	}
	fakePaths := map[string]string{
		"DynamicDesiredCountFunction": "manual/scripts/custom-resources/dynamicdesiredcountfunction/2611784f21e91e499306dac066aae5fd8f2ba664b38073bdd3198d2e041c076e.zip",
		"EnvControllerFunction":       "manual/scripts/custom-resources/envcontrollerfunction/72297cacaeab3a267e371c17ea3f0235905b0da51410eb31c10f7c66ba944044.zip",
		"RulePriorityFunction":        "manual/scripts/custom-resources/rulepriorityfunction/1385d258950a50faf4b5cd7deeecbc4bcc79a0d41d631e3977cffa0332e6f0c6.zip",
		"NLBCustomDomainFunction":     "manual/scripts/custom-resources/nlbcustomdomainfunction/ac1c96e7f0823f3167b4e74c8b286ffe8f9d43279dc232d9478837327e57905e.zip",
		"NLBCertValidatorFunction":    "manual/scripts/custom-resources/nlbcertvalidatorfunction/41aeafc64f18f82c452432a214ae83d8c8de4aba2d5df6a752b7e9a2c86833f1.zip",
	}

	// WHEN
	crs, err := LBWS(fakeFS)

	// THEN
	require.NoError(t, err)
	require.Equal(t, fakeFS.matchCount, 5, "expected path calls do not match")

	actualFnNames := make([]string, len(crs))
	for i, cr := range crs {
		actualFnNames[i] = cr.Name()
	}
	require.ElementsMatch(t,
		[]string{"DynamicDesiredCountFunction", "EnvControllerFunction", "RulePriorityFunction", "NLBCustomDomainFunction", "NLBCertValidatorFunction"},
		actualFnNames, "function names must match")

	// ensure the zip files contain an index.js file.
	for _, cr := range crs {
		buf := new(bytes.Buffer)
		size, err := buf.ReadFrom(cr.zipReader())
		require.NoError(t, err)
		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), size)
		require.NoError(t, err)

		_, err = r.Open("index.js")
		require.NoError(t, err, "an index.js file must be present in all custom resources")
	}

	// ensure artifact paths match.
	for _, cr := range crs {
		require.Equal(t, fakePaths[cr.Name()], cr.ArtifactPath())
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
	fakePaths := map[string]string{
		"DynamicDesiredCountFunction":      "manual/scripts/custom-resources/dynamicdesiredcountfunction/2611784f21e91e499306dac066aae5fd8f2ba664b38073bdd3198d2e041c076e.zip",
		"BacklogPerTaskCalculatorFunction": "manual/scripts/custom-resources/backlogpertaskcalculatorfunction/bc925d682cb47de9c65ed9cc5438ee51d9e2b9b39ca6b57bb9adda81b0091b30.zip",
		"EnvControllerFunction":            "manual/scripts/custom-resources/envcontrollerfunction/72297cacaeab3a267e371c17ea3f0235905b0da51410eb31c10f7c66ba944044.zip",
	}

	// WHEN
	crs, err := Worker(fakeFS)

	// THEN
	require.NoError(t, err)
	require.Equal(t, fakeFS.matchCount, 3, "expected path calls do not match")

	actualFnNames := make([]string, len(crs))
	for i, cr := range crs {
		actualFnNames[i] = cr.Name()
	}
	require.ElementsMatch(t,
		[]string{"DynamicDesiredCountFunction", "BacklogPerTaskCalculatorFunction", "EnvControllerFunction"},
		actualFnNames, "function names must match")

	// ensure the zip files contain an index.js file.
	for _, cr := range crs {
		buf := new(bytes.Buffer)
		size, err := buf.ReadFrom(cr.zipReader())
		require.NoError(t, err)
		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), size)
		require.NoError(t, err)

		_, err = r.Open("index.js")
		require.NoError(t, err, "an index.js file must be present in all custom resources")
	}

	// ensure artifact paths match.
	for _, cr := range crs {
		require.Equal(t, fakePaths[cr.Name()], cr.ArtifactPath())
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
	fakePaths := map[string]string{
		"DynamicDesiredCountFunction": "manual/scripts/custom-resources/dynamicdesiredcountfunction/2611784f21e91e499306dac066aae5fd8f2ba664b38073bdd3198d2e041c076e.zip",
		"EnvControllerFunction":       "manual/scripts/custom-resources/envcontrollerfunction/72297cacaeab3a267e371c17ea3f0235905b0da51410eb31c10f7c66ba944044.zip",
		"RulePriorityFunction":        "manual/scripts/custom-resources/rulepriorityfunction/1385d258950a50faf4b5cd7deeecbc4bcc79a0d41d631e3977cffa0332e6f0c6.zip",
	}

	// WHEN
	crs, err := Backend(fakeFS)

	// THEN
	require.NoError(t, err)
	require.Equal(t, fakeFS.matchCount, 3, "expected path calls do not match")

	actualFnNames := make([]string, len(crs))
	for i, cr := range crs {
		actualFnNames[i] = cr.Name()
	}
	require.ElementsMatch(t,
		[]string{"DynamicDesiredCountFunction", "RulePriorityFunction", "EnvControllerFunction"},
		actualFnNames, "function names must match")

	// ensure the zip files contain an index.js file.
	for _, cr := range crs {
		buf := new(bytes.Buffer)
		size, err := buf.ReadFrom(cr.zipReader())
		require.NoError(t, err)
		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), size)
		require.NoError(t, err)

		_, err = r.Open("index.js")
		require.NoError(t, err, "an index.js file must be present in all custom resources")
	}

	// ensure artifact paths match.
	for _, cr := range crs {
		require.Equal(t, fakePaths[cr.Name()], cr.ArtifactPath())
	}
}

func TestStaticSite(t *testing.T) {
	// GIVEN
	fakeFS := &fakeTemplateReader{
		files: map[string]*template.Content{
			"custom-resources/trigger-state-machine.js": {
				Buffer: bytes.NewBufferString("trigger state machine"),
			},
			"custom-resources/wkld-custom-domain.js": {
				Buffer: bytes.NewBufferString("service-level custom domain"),
			},
			"custom-resources/wkld-cert-validator.js": {
				Buffer: bytes.NewBufferString("service-level cert"),
			},
		},
	}
	fakePaths := map[string]string{
		"TriggerStateMachineFunction":   "manual/scripts/custom-resources/triggerstatemachinefunction/edfa40b595a5a4a6d24bfb7ad6e173560a29b7d720651ccc9c87eda76b93c7dd.zip",
		"CustomDomainFunction":          "manual/scripts/custom-resources/customdomainfunction/ac1c96e7f0823f3167b4e74c8b286ffe8f9d43279dc232d9478837327e57905e.zip",
		"CertificateValidationFunction": "manual/scripts/custom-resources/certificatevalidationfunction/41aeafc64f18f82c452432a214ae83d8c8de4aba2d5df6a752b7e9a2c86833f1.zip",
	}

	// WHEN
	crs, err := StaticSite(fakeFS)

	// THEN
	require.NoError(t, err)
	require.Equal(t, fakeFS.matchCount, 3, "expected path calls do not match")

	actualFnNames := make([]string, len(crs))
	for i, cr := range crs {
		actualFnNames[i] = cr.Name()
	}
	require.ElementsMatch(t,
		[]string{"TriggerStateMachineFunction", "CustomDomainFunction", "CertificateValidationFunction"},
		actualFnNames, "function names must match")

	// ensure the zip files contain an index.js file.
	for _, cr := range crs {
		buf := new(bytes.Buffer)
		size, err := buf.ReadFrom(cr.zipReader())
		require.NoError(t, err)
		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), size)
		require.NoError(t, err)

		_, err = r.Open("index.js")
		require.NoError(t, err, "an index.js file must be present in all custom resources")
	}

	// ensure artifact paths match.
	for _, cr := range crs {
		require.Equal(t, fakePaths[cr.Name()], cr.ArtifactPath())
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
	fakePaths := map[string]string{
		"EnvControllerFunction": "manual/scripts/custom-resources/envcontrollerfunction/72297cacaeab3a267e371c17ea3f0235905b0da51410eb31c10f7c66ba944044.zip",
	}

	// WHEN
	crs, err := ScheduledJob(fakeFS)

	// THEN
	require.NoError(t, err)
	require.Equal(t, fakeFS.matchCount, 1, "expected path calls do not match")

	actualFnNames := make([]string, len(crs))
	for i, cr := range crs {
		actualFnNames[i] = cr.Name()
	}
	require.ElementsMatch(t,
		[]string{"EnvControllerFunction"},
		actualFnNames, "function names must match")

	// ensure the zip files contain an index.js file.
	for _, cr := range crs {
		buf := new(bytes.Buffer)
		size, err := buf.ReadFrom(cr.zipReader())
		require.NoError(t, err)
		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), size)
		require.NoError(t, err)

		_, err = r.Open("index.js")
		require.NoError(t, err, "an index.js file must be present in all custom resources")
	}

	// ensure artifact paths match.
	for _, cr := range crs {
		require.Equal(t, fakePaths[cr.Name()], cr.ArtifactPath())
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
			"custom-resources/cert-replicator.js": {
				Buffer: bytes.NewBufferString("cert replication"),
			},
			"custom-resources/unique-json-values.js": {
				Buffer: bytes.NewBufferString("unique json values"),
			},
			"custom-resources/bucket-cleaner.js": {
				Buffer: bytes.NewBufferString("bucket cleaner"),
			},
		},
	}
	fakePaths := map[string]string{
		"CertificateValidationFunction": "manual/scripts/custom-resources/certificatevalidationfunction/ef49fd0cefe5525c1b98ab66614bfaebdf57dfa513a7de0d0677fc024b2f0a2b.zip",
		"CustomDomainFunction":          "manual/scripts/custom-resources/customdomainfunction/01baf83827dca2ff7df3cdf24f6ad354b3fa4f9b7cda39b5bf91de378f81c791.zip",
		"DNSDelegationFunction":         "manual/scripts/custom-resources/dnsdelegationfunction/17ec5f580cdb9c1d7c6b5b91decee031592547629a6bfed7cd33b9229f61ab19.zip",
		"CertificateReplicatorFunction": "manual/scripts/custom-resources/certificatereplicatorfunction/647f83437e4736ddf2915784e13d023a7d342d162ffb42a9eec3d7c842072030.zip",
		"UniqueJSONValuesFunction":      "manual/scripts/custom-resources/uniquejsonvaluesfunction/68c7ace14491d82ac4bb5ad81b3371743d669a26638f419265c18e9bdfca8dd1.zip",
		"BucketCleanerFunction":         "manual/scripts/custom-resources/bucketcleanerfunction/44c1eb88b269251952c25a0e17cd2c166d1de3f2340d60ad2d6b3899ceb058d9.zip",
	}

	// WHEN
	crs, err := Env(fakeFS)

	// THEN
	require.NoError(t, err)
	require.Equal(t, fakeFS.matchCount, 6, "expected path calls do not match")

	actualFnNames := make([]string, len(crs))
	for i, cr := range crs {
		actualFnNames[i] = cr.Name()
	}
	require.ElementsMatch(t,
		[]string{"CertificateValidationFunction", "CustomDomainFunction", "DNSDelegationFunction", "CertificateReplicatorFunction", "UniqueJSONValuesFunction", "BucketCleanerFunction"},
		actualFnNames, "function names must match")

	// ensure the zip files contain an index.js file.
	for _, cr := range crs {
		buf := new(bytes.Buffer)
		size, err := buf.ReadFrom(cr.zipReader())
		require.NoError(t, err)
		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), size)
		require.NoError(t, err)

		_, err = r.Open("index.js")
		require.NoError(t, err, "an index.js file must be present in all custom resources")
	}

	// ensure artifact paths match.
	for _, cr := range crs {
		require.Equal(t, fakePaths[cr.Name()], cr.ArtifactPath())
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
		crs []*CustomResource

		wantedURLs map[string]string
		wantedErr  error
	}{
		"should return a wrapped error if a custom resource cannot be uploaded": {
			s3: &fakeS3{
				err: errors.New("some err"),
			},
			crs: []*CustomResource{
				{
					name: "fn1",
					zip:  bytes.NewBufferString("hello"),
				},
			},
			wantedErr: errors.New(`upload custom resource "fn1": some err`),
		},
		"should zip and upload all custom resources": {
			s3: &fakeS3{
				objects: map[string]string{
					"manual/scripts/custom-resources/func1/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855.zip": "url1",
					"manual/scripts/custom-resources/func2/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855.zip": "url2",
				},
			},
			crs: []*CustomResource{
				{
					name: "Func1",
					zip:  new(bytes.Buffer),
				},
				{
					name: "Func2",
					zip:  new(bytes.Buffer),
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
