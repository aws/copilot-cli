// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

func TestWorkerSvc_ApplyEnv_Subscribe(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *WorkerService)
		wanted func(svc *WorkerService)
	}{
		"topics overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name:    "topic1",
							Service: "service1",
						},
						{
							Name: "topic2",
						},
						{
							Name:    "topic3",
							Service: "service3",
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
						{
							Name:    "topic2",
							Service: "service2",
						},
						{
							Name:    "topic3",
							Service: "service3.5",
						},
						{
							Name:    "topic4",
							Service: "service4",
						},
					},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
						{
							Name:    "topic2",
							Service: "service2",
						},
						{
							Name:    "topic3",
							Service: "service3.5",
						},
						{
							Name:    "topic4",
							Service: "service4",
						},
					},
				}
			},
		},
		"topics overridden by zero slice": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
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
							Name: "topic1",
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
					},
				}
			},
		},
		"queue overridden": {
			inSvc: func(svc *WorkerService) {
				mockRetention := 50 * time.Second
				mockDelay := 10 * time.Second
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						Retention: &mockRetention,
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						Delay: &mockDelay,
					},
				}
			},
			wanted: func(svc *WorkerService) {
				mockRetention := 50 * time.Second
				mockDelay := 10 * time.Second
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						Retention: &mockRetention,
						Delay:     &mockDelay,
					},
				}
			},
		},
		"queue not overridden": {
			inSvc: func(svc *WorkerService) {
				mockRetention := 50 * time.Second
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						Retention: &mockRetention,
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{}
			},
			wanted: func(svc *WorkerService) {
				mockRetention := 50 * time.Second
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						Retention: &mockRetention,
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

func TestWorkerSvc_ApplyEnv_Subscribe_Queue(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *WorkerService)
		wanted func(svc *WorkerService)
	}{
		"retention overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name:    "topic1",
							Service: "service1",
						},
						{
							Name: "topic2",
						},
						{
							Name:    "topic3",
							Service: "service3",
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
						{
							Name:    "topic2",
							Service: "service2",
						},
						{
							Name:    "topic3",
							Service: "service3.5",
						},
						{
							Name:    "topic4",
							Service: "service4",
						},
					},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
						{
							Name:    "topic2",
							Service: "service2",
						},
						{
							Name:    "topic3",
							Service: "service3.5",
						},
						{
							Name:    "topic4",
							Service: "service4",
						},
					},
				}
			},
		},
		"retention overridden by zero slice": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
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
		"retention not overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
					},
				}
			},
		},
		"delay overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name:    "topic1",
							Service: "service1",
						},
						{
							Name: "topic2",
						},
						{
							Name:    "topic3",
							Service: "service3",
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
						{
							Name:    "topic2",
							Service: "service2",
						},
						{
							Name:    "topic3",
							Service: "service3.5",
						},
						{
							Name:    "topic4",
							Service: "service4",
						},
					},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
						{
							Name:    "topic2",
							Service: "service2",
						},
						{
							Name:    "topic3",
							Service: "service3.5",
						},
						{
							Name:    "topic4",
							Service: "service4",
						},
					},
				}
			},
		},
		"delay overridden by zero slice": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
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
		"delay not overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
					},
				}
			},
		},
		"timeout overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name:    "topic1",
							Service: "service1",
						},
						{
							Name: "topic2",
						},
						{
							Name:    "topic3",
							Service: "service3",
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
						{
							Name:    "topic2",
							Service: "service2",
						},
						{
							Name:    "topic3",
							Service: "service3.5",
						},
						{
							Name:    "topic4",
							Service: "service4",
						},
					},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
						{
							Name:    "topic2",
							Service: "service2",
						},
						{
							Name:    "topic3",
							Service: "service3.5",
						},
						{
							Name:    "topic4",
							Service: "service4",
						},
					},
				}
			},
		},
		"timeout overridden by zero slice": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
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
		"timeout not overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
					},
				}
			},
		},
		"dead_letter overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name:    "topic1",
							Service: "service1",
						},
						{
							Name: "topic2",
						},
						{
							Name:    "topic3",
							Service: "service3",
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
						{
							Name:    "topic2",
							Service: "service2",
						},
						{
							Name:    "topic3",
							Service: "service3.5",
						},
						{
							Name:    "topic4",
							Service: "service4",
						},
					},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
						{
							Name:    "topic2",
							Service: "service2",
						},
						{
							Name:    "topic3",
							Service: "service3.5",
						},
						{
							Name:    "topic4",
							Service: "service4",
						},
					},
				}
			},
		},
		"dead_letter not overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
					},
				}
			},
		},
		"fifo overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name:    "topic1",
							Service: "service1",
						},
						{
							Name: "topic2",
						},
						{
							Name:    "topic3",
							Service: "service3",
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
						{
							Name:    "topic2",
							Service: "service2",
						},
						{
							Name:    "topic3",
							Service: "service3.5",
						},
						{
							Name:    "topic4",
							Service: "service4",
						},
					},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
						{
							Name:    "topic2",
							Service: "service2",
						},
						{
							Name:    "topic3",
							Service: "service3.5",
						},
						{
							Name:    "topic4",
							Service: "service4",
						},
					},
				}
			},
		},
		"fifo not overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name: "topic1",
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

func TestWorkerSvc_ApplyEnv_DeadLetterQueue(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *WorkerService)
		wanted func(svc *WorkerService)
	}{
		"tries overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						DeadLetter: &DeadLetterQueue{
							Tries: aws.Uint16(3),
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						DeadLetter: &DeadLetterQueue{
							Tries: aws.Uint16(42),
						},
					},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						DeadLetter: &DeadLetterQueue{
							Tries: aws.Uint16(42),
						},
					},
				}
			},
		},
		"tries explicitly overridden by zero value": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						DeadLetter: &DeadLetterQueue{
							Tries: aws.Uint16(3),
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						DeadLetter: &DeadLetterQueue{
							Tries: aws.Uint16(0),
						},
					},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						DeadLetter: &DeadLetterQueue{
							Tries: aws.Uint16(0),
						},
					},
				}
			},
		},
		"tries not overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						DeadLetter: &DeadLetterQueue{
							Tries: aws.Uint16(3),
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						DeadLetter: &DeadLetterQueue{
							Tries: aws.Uint16(3),
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

func TestWorkerSvc_ApplyEnv_FIFOOrBool(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *WorkerService)
		wanted func(svc *WorkerService)
	}{
		"composite: fifo overridden if enabled is not nil": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							FIFO: FIFOQueue{
								HighThroughput: aws.Bool(true),
							},
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							Enabled: aws.Bool(true),
						},
					},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							Enabled: aws.Bool(true),
						},
					},
				}
			},
		},
		"composite: enabled overridden if fifo is not nil": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							Enabled: aws.Bool(true),
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							FIFO: FIFOQueue{
								HighThroughput: aws.Bool(true),
							},
						},
					},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							FIFO: FIFOQueue{
								HighThroughput: aws.Bool(true),
							},
						},
					},
				}
			},
		},
		"high_throughput overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							FIFO: FIFOQueue{
								HighThroughput: aws.Bool(false),
							},
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							FIFO: FIFOQueue{
								HighThroughput: aws.Bool(true),
							},
						},
					},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							FIFO: FIFOQueue{
								HighThroughput: aws.Bool(true),
							},
						},
					},
				}
			},
		},
		"high_throughput overridden explicitly overridden by zero value": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							FIFO: FIFOQueue{
								HighThroughput: aws.Bool(true),
							},
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							FIFO: FIFOQueue{
								HighThroughput: aws.Bool(false),
							},
						},
					},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							FIFO: FIFOQueue{
								HighThroughput: aws.Bool(false),
							},
						},
					},
				}
			},
		},
		"high_throughput overridden not overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							FIFO: FIFOQueue{
								HighThroughput: aws.Bool(true),
							},
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							FIFO: FIFOQueue{
								HighThroughput: aws.Bool(true),
							},
						},
					},
				}
			},
		},
		"enabled overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							Enabled: aws.Bool(false),
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							Enabled: aws.Bool(true),
						},
					},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							Enabled: aws.Bool(true),
						},
					},
				}
			},
		},
		"enabled overridden explicitly overridden by zero value": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							Enabled: aws.Bool(true),
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							Enabled: aws.Bool(false),
						},
					},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							Enabled: aws.Bool(false),
						},
					},
				}
			},
		},
		"enabled overridden not overridden": {
			inSvc: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							Enabled: aws.Bool(true),
						},
					},
				}
				svc.Environments["test"].Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{},
				}
			},
			wanted: func(svc *WorkerService) {
				svc.Subscribe = &SubscribeConfig{
					Queue: &SQSQueue{
						FIFO: &FIFOOrBool{
							Enabled: aws.Bool(true),
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
