package dockercompose

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/compose-spec/compose-go/types"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConvertBackendService(t *testing.T) {
	testCases := map[string]struct {
		inSvc  types.ServiceConfig
		inPort uint16

		wantBackendSvc manifest.BackendServiceConfig
		wantIgnored    IgnoredKeys
		wantError      error
	}{
		"happy path trivial image": {
			inSvc: types.ServiceConfig{
				Name:  "web",
				Image: "nginx",
			},
			inPort: 8080,

			wantBackendSvc: manifest.BackendServiceConfig{ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
				ImageWithOptionalPort: manifest.ImageWithOptionalPort{
					Image: manifest.Image{
						Location: aws.String("nginx"),
					},
					Port: aws.Uint16(8080),
				},
			}},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			svc, ignored, err := convertBackendService(&tc.inSvc, tc.inPort)

			if tc.wantError != nil {
				require.EqualError(t, err, tc.wantError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantIgnored, ignored)
				require.Equal(t, tc.wantBackendSvc, *svc)
			}
		})
	}
}
