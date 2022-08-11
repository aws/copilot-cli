package dockercompose

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/compose-spec/compose-go/types"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestConvertBackendService(t *testing.T) {
	fiveSeconds := types.Duration(5 * time.Second)
	threeSeconds := types.Duration(3 * time.Second)
	oneSecond := types.Duration(time.Second)

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
		"happy path complete": {
			inSvc: types.ServiceConfig{
				Name: "web",

				Command: types.ShellCommand{
					"CMD-SHELL",
					"/bin/nginx",
				},
				Entrypoint: types.ShellCommand{
					"CMD",
					"/bin/sh",
				},
				EnvFile: []string{"/test.env"},
				Environment: map[string]*string{
					"HOST_PATH":    aws.String("/home/nginx"),
					"ENABLE_HTTPS": aws.String("true"),
				},
				Platform: "linux/arm64",
				HealthCheck: &types.HealthCheckConfig{
					Test: []string{
						"CMD",
						"/bin/echo",
					},
					Timeout:     &fiveSeconds,
					Interval:    &oneSecond,
					Retries:     aws.Uint64(100),
					StartPeriod: &threeSeconds,
				},
				Labels: map[string]string{
					"docker.test":  "val",
					"docker.test2": "val2",
				},
				Image: "nginx",
				Build: &types.BuildConfig{
					Context:    "dir",
					Dockerfile: "dir/Dockerfile",
					Args: map[string]*string{
						"GIT_COMMIT": aws.String("323189ab"),
						"ARG2":       aws.String("VAL"),
					},
					CacheFrom: []string{"example.com"},
					Target:    "myapp",
				},
			},
			inPort: 443,

			wantBackendSvc: manifest.BackendServiceConfig{ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
				ImageWithOptionalPort: manifest.ImageWithOptionalPort{
					Image: manifest.Image{
						Location: aws.String("nginx"),
						DockerLabels: map[string]string{
							"docker.test":  "val",
							"docker.test2": "val2",
						},
					},
					Port: aws.Uint16(443),
				},
				HealthCheck: manifest.ContainerHealthCheck{
					Command: []string{
						"CMD",
						"/bin/echo",
					},
					Timeout:     (*time.Duration)(&fiveSeconds),
					Interval:    (*time.Duration)(&oneSecond),
					Retries:     aws.Int(100),
					StartPeriod: (*time.Duration)(&threeSeconds),
				},
			},
				ImageOverride: manifest.ImageOverride{
					Command: manifest.CommandOverride{
						StringSlice: []string{
							"CMD-SHELL",
							"/bin/nginx",
						},
					},
					EntryPoint: manifest.EntryPointOverride{
						StringSlice: []string{
							"CMD",
							"/bin/sh",
						},
					},
				},
				TaskConfig: manifest.TaskConfig{
					Platform: manifest.PlatformArgsOrString{
						PlatformString: (*manifest.PlatformString)(aws.String("linux/arm64")),
					},
					Variables: map[string]string{
						"HOST_PATH":    "/home/nginx",
						"ENABLE_HTTPS": "true",
					},
					EnvFile: aws.String("/test.env"),
				}},
		},
		// TODO: Multiple env files
		// TODO: Env variable with missing values
		// TODO: Platform values
		// TODO: Healthcheck some values nil
		// TODO: Healthcheck disable
		// TODO: Extensions fields on healthcheck, build, service
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
