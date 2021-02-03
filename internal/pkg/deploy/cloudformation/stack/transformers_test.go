// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"testing"
	"time"

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

func Test_convertAutoscaling(t *testing.T) {
	const (
		mockRange    = "1-100"
		mockRequests = 1000
	)
	mockResponseTime := 512 * time.Millisecond
	testCases := map[string]struct {
		inRange        manifest.Range
		inCPU          int
		inMemory       int
		inRequests     int
		inResponseTime time.Duration

		wanted    *template.AutoscalingOpts
		wantedErr error
	}{
		"invalid range": {
			inRange: "badRange",

			wantedErr: fmt.Errorf("invalid range value badRange. Should be in format of ${min}-${max}"),
		},
		"success": {
			inRange:        mockRange,
			inCPU:          70,
			inMemory:       80,
			inRequests:     mockRequests,
			inResponseTime: mockResponseTime,

			wanted: &template.AutoscalingOpts{
				MaxCapacity:  aws.Int(100),
				MinCapacity:  aws.Int(1),
				CPU:          aws.Float64(70),
				Memory:       aws.Float64(80),
				Requests:     aws.Float64(1000),
				ResponseTime: aws.Float64(0.512),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			a := manifest.Autoscaling{
				Range:        &tc.inRange,
				CPU:          aws.Int(tc.inCPU),
				Memory:       aws.Int(tc.inMemory),
				Requests:     aws.Int(tc.inRequests),
				ResponseTime: &tc.inResponseTime,
			}
			got, err := convertAutoscaling(a)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, got, tc.wanted)
			}
		})
	}
}

func Test_convertHTTPHealthCheck(t *testing.T) {
	testCases := map[string]struct {
		inputPath               *string
		inputHealthyThreshold   *int64
		inputUnhealthyThreshold *int64
		inputInterval           *time.Duration
		inputTimeout            *time.Duration

		wantedOpts template.HTTPHealthCheckOpts
	}{
		"no fields indicated in manifest": {
			inputPath:               nil,
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: nil,
			inputInterval:           nil,
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath: "/",
			},
		},
		"just HealthyThreshold": {
			inputPath:               nil,
			inputHealthyThreshold:   aws.Int64(5),
			inputUnhealthyThreshold: nil,
			inputInterval:           nil,
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:  "/",
				HealthyThreshold: aws.Int64(5),
			},
		},
		"just UnhealthyThreshold": {
			inputPath:               nil,
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: aws.Int64(5),
			inputInterval:           nil,
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:    "/",
				UnhealthyThreshold: aws.Int64(5),
			},
		},
		"just Interval": {
			inputPath:               nil,
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: nil,
			inputInterval:           manifest.Durationp(15 * time.Second),
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath: "/",
				Interval:        aws.Int64(15),
			},
		},
		"just Timeout": {
			inputPath:               nil,
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: nil,
			inputInterval:           nil,
			inputTimeout:            manifest.Durationp(15 * time.Second),

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath: "/",
				Timeout:         aws.Int64(15),
			},
		},
		"all values changed in manifest": {
			inputPath:               aws.String("/road/to/nowhere"),
			inputHealthyThreshold:   aws.Int64(3),
			inputUnhealthyThreshold: aws.Int64(3),
			inputInterval:           manifest.Durationp(60 * time.Second),
			inputTimeout:            manifest.Durationp(60 * time.Second),

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:    "/road/to/nowhere",
				HealthyThreshold:   aws.Int64(3),
				UnhealthyThreshold: aws.Int64(3),
				Interval:           aws.Int64(60),
				Timeout:            aws.Int64(60),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			hc := manifest.HealthCheckArgsOrString{
				HealthCheckPath: tc.inputPath,
				HealthCheckArgs: manifest.HTTPHealthCheckArgs{
					Path:               tc.inputPath,
					HealthyThreshold:   tc.inputHealthyThreshold,
					UnhealthyThreshold: tc.inputUnhealthyThreshold,
					Timeout:            tc.inputTimeout,
					Interval:           tc.inputInterval,
				},
			}
			// WHEN
			actualOpts := convertHTTPHealthCheck(hc)

			// THEN
			require.Equal(t, tc.wantedOpts, actualOpts)
		})
	}
}
