// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockWriteCloser struct {
	buf *bytes.Buffer
}

func (m *mockWriteCloser) Write(p []byte) (int, error) {
	return m.buf.Write(p)
}

func (m *mockWriteCloser) Close() error {
	return nil
}

func TestErrInvalidTemplate_Error(t *testing.T) {
	err := ErrInvalidTemplate{tpl: "Horrible App"}
	require.Equal(t, "invalid manifest template: Horrible App, must be one of: Load Balanced Web App, Empty", err.Error())
}

func TestNew(t *testing.T) {
	testCases := map[string]struct {
		inputTemplate string

		wantedManifest *Manifest
		wantedError    error
	}{
		"with existing template name": {
			inputTemplate: TemplateNames[0],

			wantedManifest: &Manifest{
				tpl: TemplateNames[0],
				wc:  nil,
			},
			wantedError: nil,
		},
		"with invalid template name": {
			inputTemplate:  "Horrible App",
			wantedManifest: nil,
			wantedError:    &ErrInvalidTemplate{tpl: "Horrible App"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			m, err := New(tc.inputTemplate)

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantedManifest, m)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

func TestManifest_Render(t *testing.T) {
	testCases := map[string]struct {
		inputTemplate string
		inputApp      interface{}

		wantedErr     error
		wantedContent string
	}{
		"load balanced web app": {
			inputTemplate: TemplateNames[0],
			inputApp: struct {
				Project string
				Name    string
			}{
				Project: "chicken",
				Name:    "fryer",
			},
			wantedErr: nil,
			wantedContent: `# First is the Project name. The Project is the grouping of the
# environments related to each other.
project: chicken

application:
  # Your application name will be used in naming your resources
  # like log groups, services, etc.
  fryer:
    # The "Type" of the application you're running. For a list of all types that we support see
    # https://github.com/aws/PRIVATE-amazon-ecs-archer/app/template/manifest/
    type: LoadBalancedFargateService

    # The port exposed through your container. We need to know
    # this so that we can route traffic to it.
    containerPort: 80

    # Size of CPU
    cpu: '256'

    # Size of memory
    memory: '512'

    # Logging is enabled by default. We'll create a loggroup that is
    # the chicken/fryer/Stage
    logging: true

    # Determines whether the application will have a public IP or not.
    public: true

    # You can also pass in environment variables as key/value pairs
    #environment-variables:
    #  dog: 'Clyde'
    #  cute: 'hekya'
    #
    # Additional Sidecar apps that can run along side your main application
    #sidecars:
    #  fluentbit:
    #    containerPort: 80
    #    image: 'amazon/aws-for-fluent-bit:1.2.0'
    #    memory: '512'

# This section defines each of the release stages
# and their specific configuration for your app.
stages:
  -
    # The "environment" (cluster/vpc/lb) to contain this service.
    env: test
    # The number of tasks that we want, at minimum.
    desiredCount: 1
    # Any secrets via ARNs
    #secrets:
      #lemonaidpassword: arn:aws:secretsmanager:us-west-2:902697171733:secret:DavidsLemons/DavidsFrontEnd
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			var buf bytes.Buffer
			m := &Manifest{
				tpl: tc.inputTemplate,
				wc:  &mockWriteCloser{buf: &buf},
			}

			// WHEN
			err := m.Render("test", &tc.inputApp)

			// THEN
			if tc.wantedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
			require.Equal(t, tc.wantedContent, buf.String(), "rendered manifest templates aren't same")
		})
	}
}
