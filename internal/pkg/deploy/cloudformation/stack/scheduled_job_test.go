// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var testScheduledJobManifest = manifest.NewScheduledJob(manifest.ScheduledJobProps{
	WorkloadProps: &manifest.WorkloadProps{
		Name:       "mailer",
		Dockerfile: "mailer/Dockerfile",
	},
	Schedule: "@daily",
	Timeout:  "1h30m",
	Retries:  3,
})

// mockTemplater is declared in lb_web_svc_test.go
const (
	testJobAppName      = "cuteoverload"
	testJobEnvName      = "test"
	testJobImageRepoURL = "123456789012.dkr.ecr.us-west-2.amazonaws.com/cuteoverload/mailer"
	testJobImageTag     = "stable"
)

func TestScheduledJob_Template(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(t *testing.T, ctrl *gomock.Controller, j *ScheduledJob)

		wantedTemplate string
		wantedError    error
	}{
		"render template without addons successfully": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, j *ScheduledJob) {
				m := mocks.NewMockscheduledJobParser(ctrl)
				m.EXPECT().ParseScheduledJob(template.WorkloadOpts{}).Return(&template.Content{Buffer: bytes.NewBufferString("template")}, nil)
				addons := mockTemplater{err: &addon.ErrDirNotExist{}}
				j.parser = m
				j.wkld.addons = addons
			},
			wantedTemplate: "template",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			conf := &ScheduledJob{
				wkld: &wkld{
					name: aws.StringValue(testScheduledJobManifest.Name),
					env:  testJobEnvName,
					app:  testJobAppName,
					rc: RuntimeConfig{
						ImageRepoURL: testJobImageRepoURL,
						ImageTag:     testJobImageTag,
					},
				},
				manifest: testScheduledJobManifest,
			}
			tc.mockDependencies(t, ctrl, conf)

			// WHEN
			template, err := conf.Template()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedTemplate, template)
			}
		})
	}
}
