// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"testing"

	awssecretsmanager "github.com/aws/copilot-cli/internal/pkg/aws/secretsmanager"
	awsssm "github.com/aws/copilot-cli/internal/pkg/aws/ssm"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeploySvcOpts_MissingSecrets(t *testing.T) {
	testCases := map[string]struct {
		inManifest string
		setupMocks func(m *deploySvcSecretMocks)

		wanted []missingSecret
	}{
		"returns SSM and Secrets Manager refs that do not exist": {
			inManifest: `
name: frontend
type: "Load Balanced Web Service"
image:
  location: nginx
  port: 80
secrets:
  DB_PASSWORD: /copilot/phonetool/test/secrets/db-password
logging:
  secretOptions:
    LOG_TOKEN: /copilot/phonetool/test/secrets/log-token
sidecars:
  xray:
    image: xray-daemon
    secrets:
      API_TOKEN:
        secretsmanager: sidecar-secret
`,
			setupMocks: func(m *deploySvcSecretMocks) {
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "/copilot/phonetool/test/secrets/db-password").
					Return("", &awsssm.ErrParameterNotFound{})
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "/copilot/phonetool/test/secrets/log-token").
					Return("log-token", nil)
				m.secretsmanager.EXPECT().DescribeSecret("sidecar-secret").
					Return(nil, &awssecretsmanager.ErrSecretNotFound{})
			},
			wanted: []missingSecret{
				{name: "/copilot/phonetool/test/secrets/db-password", store: secretStoreSSM},
				{name: "sidecar-secret", store: secretStoreSecretsManager},
			},
		},
		"skips imported secrets and ignores validation errors that are not not-found errors": {
			inManifest: `
name: frontend
type: "Backend Service"
image:
  location: nginx
secrets:
  IMPORTED:
    from_cfn: stack-SecretName
  API_KEY: /copilot/phonetool/test/secrets/api-key
`,
			setupMocks: func(m *deploySvcSecretMocks) {
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "/copilot/phonetool/test/secrets/api-key").
					Return("", errors.New("access denied"))
			},
		},
		"deduplicates repeated refs": {
			inManifest: `
name: frontend
type: "Request-Driven Web Service"
image:
  location: nginx
secrets:
  FIRST: shared-secret
  SECOND: shared-secret
`,
			setupMocks: func(m *deploySvcSecretMocks) {
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "shared-secret").
					Return("", &awsssm.ErrParameterNotFound{})
			},
			wanted: []missingSecret{
				{name: "shared-secret", store: secretStoreSSM},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClients := deploySvcSecretMocks{
				ssm:            mocks.NewMocksecretGetter(ctrl),
				secretsmanager: mocks.NewMocksecretDeleter(ctrl),
			}
			tc.setupMocks(&mockClients)

			opts := deploySvcOpts{
				ssmParamGetter: mockClients.ssm,
				secretsmanager: mockClients.secretsmanager,
			}
			require.ElementsMatch(t, tc.wanted, opts.missingSecrets(context.Background(), mustUnmarshalWorkload(t, tc.inManifest)))
		})
	}
}

type deploySvcSecretMocks struct {
	ssm            *mocks.MocksecretGetter
	secretsmanager *mocks.MocksecretDeleter
}

func mustUnmarshalWorkload(t *testing.T, raw string) any {
	t.Helper()

	mft, err := manifest.UnmarshalWorkload([]byte(raw))
	require.NoError(t, err)
	return mft.Manifest()
}
