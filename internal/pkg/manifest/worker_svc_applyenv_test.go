// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyEnv_SubscribeConfig(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *WorkerService)
		wanted func(svc *WorkerService)
	}{
		"topics overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic",
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topicTest",
						},
					},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topicTest",
						},
					},
				}
			},
		},
		"topics explicitly overridden by zero slice": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic",
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{},
				}
			},
		},
		"topics not overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic",
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic",
						},
					},
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc WorkerService
			inSvc.Environments = map[string]*WorkerServiceConfig{
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
