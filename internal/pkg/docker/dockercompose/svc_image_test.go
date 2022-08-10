package dockercompose

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/compose-spec/compose-go/types"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConvertImageConfig(t *testing.T) {
	testCases := map[string]struct {
		inBuild  types.BuildConfig
		inImgLoc string

		wantImage       manifest.Image
		wantIgnoredKeys IgnoredKeys
		wantErr         error
	}{
		"happy path image only": {
			inImgLoc: "nginx",
			wantImage: manifest.Image{
				Location: aws.String("nginx"),
			},
		},
		"happy path image and build": {
			inImgLoc: "nginx",
			inBuild: types.BuildConfig{
				Context:    "test",
				Dockerfile: "Dockerfile.test",
			},
			wantImage: manifest.Image{
				Location: aws.String("nginx"),
			},
		},
		"happy path build only": {
			inBuild: types.BuildConfig{
				Context:    "test",
				Dockerfile: "Dockerfile.test",
			},
			wantImage: manifest.Image{
				Build: manifest.BuildArgsOrString{BuildArgs: manifest.DockerBuildArgs{
					Context:    aws.String("test"),
					Dockerfile: aws.String("Dockerfile.test"),
				}},
			},
		},
		"build with all non-fatal properties": {
			inBuild: types.BuildConfig{
				Context:    "test",
				Dockerfile: "Dockerfile.test",
				Args: map[string]*string{
					"GIT_COMMIT": aws.String("323189ab"),
					"ARG2":       aws.String("VAL"),
				},
				Labels: map[string]string{
					"docker.test": "val",
				},
				CacheFrom: []string{
					"example.com",
				},
				CacheTo: []string{
					"example2.com",
				},
				NoCache:   true,
				Pull:      true,
				Isolation: "none",
				Target:    "myapp",
				Tags: []string{
					"tag",
				},
				Extensions: map[string]interface{}{
					"test": "ext",
				},
			},
			wantIgnoredKeys: []string{
				"build.cache_to",
				"build.no_cache",
				"build.pull",
				"build.isolation",
				"build.tags",
				"build.extensions",
			},
			wantImage: manifest.Image{
				DockerLabels: map[string]string{
					"docker.test": "val",
				},
				Build: manifest.BuildArgsOrString{BuildArgs: manifest.DockerBuildArgs{
					Context:    aws.String("test"),
					Dockerfile: aws.String("Dockerfile.test"),
					Args: map[string]string{
						"GIT_COMMIT": "323189ab",
						"ARG2":       "VAL",
					},
					CacheFrom: []string{"example.com"},
					Target:    aws.String("myapp"),
				}},
			},
		},
		"fatal build.ssh": {
			inBuild: types.BuildConfig{
				SSH: []types.SSHKey{
					{
						ID:   "ssh",
						Path: "/test",
					},
				},
			},
			wantErr: errors.New("`build.ssh` and `build.secrets` are not supported yet, see https://github.com/aws/copilot-cli/issues/2090 for details"),
		},
		"fatal build.secrets": {
			inBuild: types.BuildConfig{
				Secrets: []types.ServiceSecretConfig{
					{
						Source: "/root",
					},
				},
			},
			wantErr: errors.New("`build.ssh` and `build.secrets` are not supported yet, see https://github.com/aws/copilot-cli/issues/2090 for details"),
		},
		"fatal build.extra_hosts": {
			inBuild: types.BuildConfig{
				ExtraHosts: map[string]string{
					"host1": "192.168.1.1",
				},
			},
			wantErr: errors.New("key `build.extra_hosts` is not supported yet, this might break your app"),
		},
		"fatal build.network": {
			inBuild: types.BuildConfig{
				Network: "none",
			},
			wantErr: errors.New("key `build.network` is not supported yet, this might break your app"),
		},
		"fatal missing arg values": {
			inBuild: types.BuildConfig{
				Args: map[string]*string{
					"GIT_COMMIT": nil,
					"ARG2":       aws.String("VAL"),
				},
			},
			wantErr: errors.New("convert build args: some entries are missing values and require user input, this is not supported in Copilot: [GIT_COMMIT]"),
		},
		// TODO
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			img, ignored, err := convertImageConfig(&tc.inBuild, tc.inImgLoc)

			if tc.wantErr != nil {
				require.Error(t, err)
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantIgnoredKeys, ignored)
				require.Equal(t, tc.wantImage, img)
			}
		})
	}
}
