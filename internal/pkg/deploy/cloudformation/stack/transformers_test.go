//
package stack

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/stretchr/testify/require"
)

func Test_convertSidecar(t *testing.T) {
	mockImage := aws.String("mockImage")
	mockMap := map[string]string{"foo": "bar"}
	mockCredsParam := aws.String("mockCredsParam")
	testCases := map[string]struct {
		inPort string

		wanted    *template.SidecarOpts
		wantedErr error
	}{
		"invalid port": {
			inPort: "b/a/d/P/o/r/t",

			wantedErr: fmt.Errorf("cannot parse port mapping from b/a/d/P/o/r/t"),
		},
		"good port without protocol": {
			inPort: "2000",

			wanted: &template.SidecarOpts{
				Name:       aws.String("foo"),
				Port:       aws.String("2000"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockMap,
				Variables:  mockMap,
			},
		},
		"good port with protocol": {
			inPort: "2000/udp",

			wanted: &template.SidecarOpts{
				Name:       aws.String("foo"),
				Port:       aws.String("2000"),
				Protocol:   aws.String("udp"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockMap,
				Variables:  mockMap,
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			sidecar := manifest.Sidecar{
				Sidecars: map[string]*manifest.SidecarConfig{
					"foo": {
						CredsParam: mockCredsParam,
						Image:      mockImage,
						Secrets:    mockMap,
						Variables:  mockMap,
						Port:       aws.String(tc.inPort),
					},
				},
			}
			got, err := convertSidecar(sidecar)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, got[0], tc.wanted)
			}
		})
	}
}
