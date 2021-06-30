package manifest

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
)

func Test_Fancy(t *testing.T) {
	in := LoadBalancedWebService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(LoadBalancedWebServiceType),
		},
		LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
			ImageConfig: ImageWithPortAndHealthcheck{
				ImageWithPort: ImageWithPort{
					Image: Image{
						Build: BuildArgsOrString{
							BuildArgs: DockerBuildArgs{
								Dockerfile: aws.String("./Dockerfile"),
							},
						},
					},
					Port: aws.Uint16(80),
				},
			},
			RoutingRule: RoutingRule{
				Path: aws.String("/awards/*"),
				HealthCheck: HealthCheckArgsOrString{
					HealthCheckPath: aws.String("/"),
				},
			},
			TaskConfig: TaskConfig{
				CPU:    aws.Int(1024),
				Memory: aws.Int(1024),
				Count: Count{
					Value: aws.Int(1),
				},
				Variables: map[string]string{
					"LOG_LEVEL":      "DEBUG",
					"DDB_TABLE_NAME": "awards",
				},
				Secrets: map[string]string{
					"GITHUB_TOKEN": "1111",
					"TWILIO_TOKEN": "1111",
				},
				Storage: &Storage{
					Volumes: map[string]Volume{
						"myEFSVolume": {
							MountPointOpts: MountPointOpts{
								ContainerPath: aws.String("/path/to/files"),
								ReadOnly:      aws.Bool(false),
							},
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String("fs-1234"),
									AuthConfig: &AuthorizationConfig{
										IAM:           aws.Bool(true),
										AccessPointID: aws.String("ap-1234"),
									},
								},
							},
						},
					},
				},
			},
			Sidecars: map[string]*SidecarConfig{
				"xray": {
					Port:       aws.String("2000"),
					Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
					CredsParam: aws.String("some arn"),
				},
			},
			Logging: &Logging{
				ConfigFile: aws.String("mockConfigFile"),
			},
			Network: &NetworkConfig{
				VPC: &vpcConfig{
					Placement:      stringP("public"),
					SecurityGroups: []string{"sg-123"},
				},
			},
		},
		Environments: map[string]*LoadBalancedWebServiceConfig{
			"prod-iad": {
				ImageConfig: ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Image: Image{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Dockerfile: aws.String("./RealDockerfile"),
								},
							},
						},
						Port: aws.Uint16(5000),
					},
				},
				RoutingRule: RoutingRule{
					TargetContainer: aws.String("xray"),
				},
				TaskConfig: TaskConfig{
					CPU: aws.Int(2046),
					Count: Count{
						Value: aws.Int(0),
					},
					Variables: map[string]string{
						"DDB_TABLE_NAME": "awards-prod",
					},
					Storage: &Storage{
						Volumes: map[string]Volume{
							"myEFSVolume": {
								EFS: &EFSConfigOrBool{
									Advanced: EFSVolumeConfiguration{
										FileSystemID: aws.String("fs-5678"),
										AuthConfig: &AuthorizationConfig{
											AccessPointID: aws.String("ap-5678"),
										},
									},
								},
							},
						},
					},
				},
				Sidecars: map[string]*SidecarConfig{
					"xray": {
						Port: aws.String("2000/udp"),
						MountPoints: []SidecarMountPoint{
							{
								SourceVolume: aws.String("myEFSVolume"),
								MountPointOpts: MountPointOpts{
									ReadOnly:      aws.Bool(true),
									ContainerPath: aws.String("/var/www"),
								},
							},
						},
					},
				},
				Logging: &Logging{
					SecretOptions: map[string]string{
						"FOO": "BAR",
					},
				},
				Network: &NetworkConfig{
					VPC: &vpcConfig{
						SecurityGroups: []string{"sg-456", "sg-789"},
					},
				},
			},
		},
	}

	dst := reflect.ValueOf(in)
	for i, n := 0, dst.NumField(); i < n; i++ {
		fmt.Println(dst.Type().Field(i))
	}
}
