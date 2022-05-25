// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package list

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/list/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestList_JobListWriter(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockError := fmt.Errorf("error")
	mockStore := mocks.NewMockStore(ctrl)
	mockWs := mocks.NewMockWorkspace(ctrl)

	mockAppName := "barnyard"

	testCases := map[string]struct {
		inputAppName   string
		inputWriteJSON bool
		inputListLocal bool

		wantedError   error
		wantedContent string

		mocking func()
	}{
		"should succeed writing human readable": {
			inputAppName:   mockAppName,
			inputWriteJSON: false,

			wantedContent: `Name                Type
----                ----
badgoose            Scheduled Job
farmer              Scheduled Job
`,
			mocking: func() {
				mockStore.EXPECT().
					GetApplication(gomock.Eq("barnyard")).
					Return(&config.Application{}, nil)
				mockStore.
					EXPECT().
					ListJobs(gomock.Eq("barnyard")).
					Return([]*config.Workload{
						{Name: "badgoose", Type: "Scheduled Job"},
						{Name: "farmer", Type: "Scheduled Job"},
					}, nil)
			},
		},
		"should succeed writing json": {
			inputAppName:   mockAppName,
			inputWriteJSON: true,

			wantedContent: `{"jobs":[{"app":"","name":"badgoose","type":"Scheduled Job"},{"app":"","name":"farmer","type":"Scheduled Job"}]}
`,
			mocking: func() {
				mockStore.EXPECT().
					GetApplication(gomock.Eq("barnyard")).
					Return(&config.Application{}, nil)
				mockStore.
					EXPECT().
					ListJobs(gomock.Eq("barnyard")).
					Return([]*config.Workload{
						{Name: "badgoose", Type: "Scheduled Job"},
						{Name: "farmer", Type: "Scheduled Job"},
					}, nil)
			},
		},
		"with bad application name": {
			inputAppName: mockAppName,

			wantedError: fmt.Errorf("get application: error"),

			mocking: func() {
				mockStore.EXPECT().
					GetApplication(gomock.Eq("barnyard")).
					Return(nil, mockError)
				mockStore.
					EXPECT().
					ListJobs(gomock.Eq("barnyard")).
					Times(0)
			},
		},
		"listing local jobs": {
			inputAppName:   mockAppName,
			inputListLocal: true,

			wantedContent: "Name                Type\n----                ----\nbadgoose            Scheduled Job\n",

			mocking: func() {
				mockStore.EXPECT().GetApplication("barnyard").
					Return(&config.Application{}, nil)
				mockStore.EXPECT().ListJobs("barnyard").
					Return([]*config.Workload{
						{Name: "badgoose", Type: "Scheduled Job"},
						{Name: "farmer", Type: "Scheduled Job"},
					}, nil)
				mockWs.EXPECT().ListJobs().Return([]string{"badgoose"}, nil)
			},
		},
		"with failed call to ListJobs": {
			inputAppName: mockAppName,

			wantedError: fmt.Errorf("get job names: error"),

			mocking: func() {
				mockStore.EXPECT().GetApplication("barnyard").
					Return(&config.Application{}, nil)
				mockStore.EXPECT().ListJobs("barnyard").
					Return(nil, mockError)
			},
		},
		"with no local jobs": {
			inputAppName:   mockAppName,
			inputListLocal: true,

			wantedContent: "Name                Type\n----                ----\n",

			mocking: func() {
				mockStore.EXPECT().GetApplication("barnyard").
					Return(&config.Application{}, nil)
				mockStore.EXPECT().ListJobs("barnyard").
					Return([]*config.Workload{
						{Name: "badgoose", Type: "Scheduled Job"},
						{Name: "farmer", Type: "Scheduled Job"},
					}, nil)
				mockWs.EXPECT().ListJobs().Return([]string{}, nil)
			},
		},
		"with no local jobs json": {
			inputAppName:   mockAppName,
			inputWriteJSON: true,
			inputListLocal: true,

			wantedContent: "{\"jobs\":null}\n",

			mocking: func() {
				mockStore.EXPECT().GetApplication("barnyard").
					Return(&config.Application{}, nil)
				mockStore.EXPECT().ListJobs("barnyard").
					Return([]*config.Workload{
						{Name: "badgoose", Type: "Scheduled Job"},
						{Name: "farmer", Type: "Scheduled Job"},
					}, nil)
				mockWs.EXPECT().ListJobs().Return([]string{""}, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			b := &bytes.Buffer{}
			tc.mocking()
			list := &JobListWriter{
				Ws:    mockWs,
				Store: mockStore,
				Out:   b,

				ShowLocalJobs: tc.inputListLocal,
				OutputJSON:    tc.inputWriteJSON,
			}

			// WHEN
			err := list.Write(tc.inputAppName)

			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.Equal(t, tc.wantedContent, b.String())
			}
		})
	}
}

func TestList_SvcListWriter(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockError := fmt.Errorf("error")
	mockStore := mocks.NewMockStore(ctrl)
	mockWs := mocks.NewMockWorkspace(ctrl)

	mockAppName := "barnyard"

	testCases := map[string]struct {
		inputAppName   string
		inputWriteJSON bool
		inputListLocal bool

		wantedError   error
		wantedContent string

		mocking func()
	}{
		"should succeed writing human readable": {
			inputAppName:   mockAppName,
			inputWriteJSON: false,

			wantedContent: "Name                Type\n----                ----\ntrough              Backend Service\ngaggle              Load Balanced Web Service\n",
			mocking: func() {
				mockStore.EXPECT().
					GetApplication(gomock.Eq("barnyard")).
					Return(&config.Application{}, nil)
				mockStore.
					EXPECT().
					ListServices(gomock.Eq("barnyard")).
					Return([]*config.Workload{
						{Name: "trough", Type: "Backend Service"},
						{Name: "gaggle", Type: "Load Balanced Web Service"},
					}, nil)
			},
		},
		"should succeed writing json": {
			inputAppName:   mockAppName,
			inputWriteJSON: true,

			wantedContent: `{"services":[{"app":"","name":"trough","type":"Backend Service"},{"app":"","name":"gaggle","type":"Load Balanced Web Service"}]}
`,
			mocking: func() {
				mockStore.EXPECT().
					GetApplication(gomock.Eq("barnyard")).
					Return(&config.Application{}, nil)
				mockStore.
					EXPECT().
					ListServices(gomock.Eq("barnyard")).
					Return([]*config.Workload{
						{Name: "trough", Type: "Backend Service"},
						{Name: "gaggle", Type: "Load Balanced Web Service"},
					}, nil)
			},
		},
		"with bad application name": {
			inputAppName: mockAppName,

			wantedError: fmt.Errorf("get application: error"),

			mocking: func() {
				mockStore.EXPECT().
					GetApplication(gomock.Eq("barnyard")).
					Return(nil, mockError)
				mockStore.
					EXPECT().
					ListServices(gomock.Eq("barnyard")).
					Times(0)
			},
		},
		"listing local services": {
			inputAppName:   mockAppName,
			inputListLocal: true,

			wantedContent: "Name                Type\n----                ----\ntrough              Backend Service\n",

			mocking: func() {
				mockStore.EXPECT().GetApplication("barnyard").
					Return(&config.Application{}, nil)
				mockStore.EXPECT().ListServices("barnyard").
					Return([]*config.Workload{
						{Name: "trough", Type: "Backend Service"},
						{Name: "gaggle", Type: "Load Balanced Web Service"},
					}, nil)
				mockWs.EXPECT().ListServices().Return([]string{"trough"}, nil)
			},
		},
		"with failed call to ListJobs": {
			inputAppName: mockAppName,

			wantedError: fmt.Errorf("get service names: error"),

			mocking: func() {
				mockStore.EXPECT().GetApplication("barnyard").
					Return(&config.Application{}, nil)
				mockStore.EXPECT().ListServices("barnyard").
					Return(nil, mockError)
			},
		},
		"with no local services": {
			inputAppName:   mockAppName,
			inputListLocal: true,

			wantedContent: "Name                Type\n----                ----\n",

			mocking: func() {
				mockStore.EXPECT().GetApplication("barnyard").
					Return(&config.Application{}, nil)
				mockStore.EXPECT().ListServices("barnyard").
					Return([]*config.Workload{
						{Name: "trough", Type: "Backend Service"},
						{Name: "gaggle", Type: "Load Balanced Web Service"},
					}, nil)
				mockWs.EXPECT().ListServices().Return([]string{}, nil)
			},
		},
		"with no local services json": {
			inputAppName:   mockAppName,
			inputWriteJSON: true,
			inputListLocal: true,

			wantedContent: "{\"services\":null}\n",

			mocking: func() {
				mockStore.EXPECT().GetApplication("barnyard").
					Return(&config.Application{}, nil)
				mockStore.EXPECT().ListServices("barnyard").
					Return([]*config.Workload{
						{Name: "trough", Type: "Backend Service"},
						{Name: "gaggle", Type: "Load Balanced Web Service"},
					}, nil)
				mockWs.EXPECT().ListServices().Return([]string{}, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			b := &bytes.Buffer{}
			tc.mocking()
			list := &SvcListWriter{
				Ws:    mockWs,
				Store: mockStore,
				Out:   b,

				ShowLocalSvcs: tc.inputListLocal,
				OutputJSON:    tc.inputWriteJSON,
			}

			// WHEN
			err := list.Write(tc.inputAppName)

			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.Equal(t, tc.wantedContent, b.String())
			}
		})
	}
}
