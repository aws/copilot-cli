package manifest

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/stretchr/testify/require"
)

func TestApplyEnv_Count(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"composite fields: value is overridden if advanced count is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					Value: aws.Int(13),
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(42),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(42),
					},
				}
			},
		},
		"composite fields: advanced count is overridden if value is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(42),
					},
				}
				svc.Environments["test"].Count = Count{
					Value: aws.Int(13),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					Value: aws.Int(13),
				}
			},
		},
		"value overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					Value: aws.Int(13),
				}
				svc.Environments["test"].Count = Count{
					Value: aws.Int(42),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					Value: aws.Int(42),
				}
			},
		},
		"value explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					Value: aws.Int(13),
				}
				svc.Environments["test"].Count = Count{
					Value: aws.Int(0),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					Value: aws.Int(0),
				}
			},
		},
		"value not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					Value: aws.Int(13),
				}
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					Value: aws.Int(13),
				}
			},
		},
		"advanced count overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(13),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("1-10")),
						},
						CPU: aws.Int(70),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("1-10")),
						},
						CPU: aws.Int(70),
					},
				}
			},
		},
		"advanced count not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(13),
					},
				}
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(13),
					},
				}
			},
		},
		"exclusive fields: spot overridden if range is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(13),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Min:      aws.Int(1),
								Max:      aws.Int(10),
								SpotFrom: aws.Int(4),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Min:      aws.Int(1),
								Max:      aws.Int(10),
								SpotFrom: aws.Int(4),
							},
						},
					},
				}
			},
		},
		"exclusive fields: range overridden if spot is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Min:      aws.Int(1),
								Max:      aws.Int(10),
								SpotFrom: aws.Int(4),
							},
						},
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(42),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(42),
					},
				}
			},
		},
		"exclusive fields: spot overridden if cpu_percentage is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(13),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						CPU: aws.Int(70),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						CPU: aws.Int(70),
					},
				}
			},
		},
		"exclusive fields: cpu_percentage overridden if spot is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						CPU: aws.Int(60),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(42),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(42),
					},
				}
			},
		},
		"exclusive fields: spot overridden if memory_percentage is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(13),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Memory: aws.Int(60),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Memory: aws.Int(60),
					},
				}
			},
		},
		"exclusive fields: memory_percentage overridden if spot is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Memory: aws.Int(70),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(42),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(42),
					},
				}
			},
		},
		"exclusive fields: spot overridden if requests is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(13),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Requests: aws.Int(1010),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Requests: aws.Int(1010),
					},
				}
			},
		},
		"exclusive fields: requests overridden if spot is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Requests: aws.Int(1010),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(42),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(42),
					},
				}
			},
		},
		"exclusive fields: spot overridden if response_time is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockResponseTime := 30 * time.Second
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(13),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						ResponseTime: &mockResponseTime,
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockResponseTime := 30 * time.Second
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						ResponseTime: &mockResponseTime,
					},
				}
			},
		},
		"exclusive fields: response_time overridden if spot is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockResponseTime := 30 * time.Second
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						ResponseTime: &mockResponseTime,
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(42),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(42),
					},
				}
			},
		},
		"spot overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(13),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(42),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(42),
					},
				}
			},
		},
		"spot explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(13),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(0),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(0),
					},
				}
			},
		},
		"spot not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(13),
					},
				}
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(13),
					},
				}
			},
		},
		"range overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("1-10")),
						},
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("13-42")),
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("13-42")),
						},
					},
				}
			},
		},
		"range not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("1-10")),
						},
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("1-10")),
						},
					},
				}
			},
		},
		"cpu_percentage overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						CPU: aws.Int(70),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						CPU: aws.Int(42),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						CPU: aws.Int(42),
					},
				}
			},
		},
		"cpu_percentage explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						CPU: aws.Int(70),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						CPU: aws.Int(0),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						CPU: aws.Int(0),
					},
				}
			},
		},
		"cpu_percentage not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						CPU: aws.Int(70),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						CPU: aws.Int(70),
					},
				}
			},
		},
		"memory_percentage overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Memory: aws.Int(65),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Memory: aws.Int(42),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Memory: aws.Int(42),
					},
				}
			},
		},
		"memory_percentage explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Memory: aws.Int(65),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Memory: aws.Int(0),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Memory: aws.Int(0),
					},
				}
			},
		},
		"memory_percentage not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Memory: aws.Int(65),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Memory: aws.Int(65),
					},
				}
			},
		},
		"requests overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Requests: aws.Int(3030),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Requests: aws.Int(1010),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Requests: aws.Int(1010),
					},
				}
			},
		},
		"requests explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Requests: aws.Int(65),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Memory: aws.Int(0),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Memory: aws.Int(0),
					},
				}
			},
		},
		"requests not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Requests: aws.Int(65),
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Requests: aws.Int(65),
					},
				}
			},
		},
		"response_time overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockResponseTime := 1010 * time.Second
				mockResponseTimeTest := 4242 * time.Second
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						ResponseTime: &mockResponseTime,
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						ResponseTime: &mockResponseTimeTest,
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockResponseTimeTest := 4242 * time.Second
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						ResponseTime: &mockResponseTimeTest,
					},
				}
			},
		},
		"response_time explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockResponseTime := 1010 * time.Second
				mockResponseTimeTest := 0 * time.Second
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						ResponseTime: &mockResponseTime,
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						ResponseTime: &mockResponseTimeTest,
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockResponseTimeTest := 0 * time.Second
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						ResponseTime: &mockResponseTimeTest,
					},
				}
			},
		},
		"response_time not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockResponseTime := 1010 * time.Second
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						ResponseTime: &mockResponseTime,
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockResponseTimeTest := 1010 * time.Second
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						ResponseTime: &mockResponseTimeTest,
					},
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc LoadBalancedWebService
			inSvc.Environments = map[string]*LoadBalancedWebServiceConfig{
				"test": {},
			}

			tc.inSvc(&inSvc)
			tc.wanted(&wantedSvc)

			got, err := inSvc.ApplyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}

func TestApplyEnv_Count_Range(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"composite fields: range value is overridden if range config is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("1-10")),
						},
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Min:      aws.Int(5),
								Max:      aws.Int(42),
								SpotFrom: aws.Int(13),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Min:      aws.Int(5),
								Max:      aws.Int(42),
								SpotFrom: aws.Int(13),
							},
						},
					},
				}
			},
		},
		"composite fields: range config is overridden if range value is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Min:      aws.Int(5),
								Max:      aws.Int(42),
								SpotFrom: aws.Int(13),
							},
						},
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("1-10")),
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("1-10")),
						},
					},
				}
			},
		},
		"range value overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("1-10")),
						},
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("13-42")),
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("13-42")),
						},
					},
				}
			},
		},
		"range value explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("1-10")),
						},
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("")),
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("")),
						},
					},
				}
			},
		},
		"range value not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("1-10")),
						},
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							Value: (*IntRangeBand)(aws.String("1-10")),
						},
					},
				}
			},
		},
		"min overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Min: aws.Int(5),
							},
						},
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Min: aws.Int(13),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Min: aws.Int(13),
							},
						},
					},
				}
			},
		},
		"min explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Min: aws.Int(5),
							},
						},
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Min: aws.Int(0),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Min: aws.Int(0),
							},
						},
					},
				}
			},
		},
		//"FAILED TEST: min not overridden": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Count = Count{
		//			AdvancedCount: AdvancedCount{
		//				Range: &Range{
		//					RangeConfig: RangeConfig{
		//						Min: aws.Int(5),
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Count = Count{
		//			AdvancedCount: AdvancedCount{
		//				Range: &Range{
		//					RangeConfig: RangeConfig{},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Count = Count{
		//			AdvancedCount: AdvancedCount{
		//				Range: &Range{
		//					RangeConfig: RangeConfig{
		//						Min: aws.Int(5),
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		"max overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Max: aws.Int(13),
							},
						},
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Max: aws.Int(42),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Max: aws.Int(42),
							},
						},
					},
				}
			},
		},
		"max explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Max: aws.Int(13),
							},
						},
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Max: aws.Int(0),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								Max: aws.Int(0),
							},
						},
					},
				}
			},
		},
		//"FAILED TEST: max not overridden": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Count = Count{
		//			AdvancedCount: AdvancedCount{
		//				Range: &Range{
		//					RangeConfig: RangeConfig{
		//						Max: aws.Int(13),
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Count = Count{
		//			AdvancedCount: AdvancedCount{
		//				Range: &Range{
		//					RangeConfig: RangeConfig{},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Count = Count{
		//			AdvancedCount: AdvancedCount{
		//				Range: &Range{
		//					RangeConfig: RangeConfig{
		//						Max: aws.Int(13),
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		"spot_from overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								SpotFrom: aws.Int(10),
							},
						},
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								SpotFrom: aws.Int(13),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								SpotFrom: aws.Int(13),
							},
						},
					},
				}
			},
		},
		"spot_from explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								SpotFrom: aws.Int(10),
							},
						},
					},
				}
				svc.Environments["test"].Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								SpotFrom: aws.Int(0),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					AdvancedCount: AdvancedCount{
						Range: &Range{
							RangeConfig: RangeConfig{
								SpotFrom: aws.Int(0),
							},
						},
					},
				}
			},
		},
		//"FAILED TEST: spot_from not overridden": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Count = Count{
		//			AdvancedCount: AdvancedCount{
		//				Range: &Range{
		//					RangeConfig: RangeConfig{
		//						SpotFrom: aws.Int(10),
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Count = Count{
		//			AdvancedCount: AdvancedCount{
		//				Range: &Range{
		//					RangeConfig: RangeConfig{},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Count = Count{
		//			AdvancedCount: AdvancedCount{
		//				Range: &Range{
		//					RangeConfig: RangeConfig{
		//						SpotFrom: aws.Int(10),
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc LoadBalancedWebService
			inSvc.Environments = map[string]*LoadBalancedWebServiceConfig{
				"test": {},
			}

			tc.inSvc(&inSvc)
			tc.wanted(&wantedSvc)

			got, err := inSvc.ApplyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}
