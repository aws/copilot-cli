// Copyright Amazon, Inc. or its affiliates. All rights reserved.

package cli

import (
	"errors"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestStorageInitOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppName     string
		inStorageType string
		inSvcName     string
		inStorageName string

		mockWs    func(m *mocks.MockwsSvcReader)
		mockStore func(m *mocks.Mockstore)

		wantedErr error
	}{
		"no app in workspace": {
			mockWs:    func(m *mocks.MockwsSvcReader) {},
			mockStore: func(m *mocks.Mockstore) {},

			wantedErr: errNoAppInWorkspace,
		},
		"svc not in workspace": {
			mockWs: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Return([]string{"bad", "workspace"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {},

			inAppName:     "bowie",
			inStorageType: s3StorageType,
			inSvcName:     "frontend",
			inStorageName: "my-bucket",
			wantedErr:     errors.New("service frontend not found in the workspace"),
		},
		"workspace error": {
			mockWs: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Return(nil, errors.New("wanted err"))
			},
			mockStore: func(m *mocks.Mockstore) {},

			inAppName:     "bowie",
			inStorageType: s3StorageType,
			inSvcName:     "frontend",
			inStorageName: "my-bucket",
			wantedErr:     errors.New("retrieve local service names: wanted err"),
		},
		"happy path s3": {
			mockWs: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Return([]string{"frontend"}, nil)
			},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: s3StorageType,
			inSvcName:     "frontend",
			inStorageName: "my-bucket.4",
			wantedErr:     nil,
		},
		"happy path ddb": {
			mockWs: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Return([]string{"frontend"}, nil)
			},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inSvcName:     "frontend",
			inStorageName: "my-cool_table.3",
			wantedErr:     nil,
		},
		"default to ddb name validation when storage type unspecified": {
			mockWs: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Return([]string{"frontend"}, nil)
			},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: "",
			inSvcName:     "frontend",
			inStorageName: "my-cool_table.3",
			wantedErr:     nil,
		},
		"s3 bad character": {
			mockWs: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Return([]string{"frontend"}, nil)
			},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: s3StorageType,
			inSvcName:     "frontend",
			inStorageName: "mybadbucket???",
			wantedErr:     errS3ValueBadCharacter,
		},
		"ddb bad character": {
			mockWs: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Return([]string{"frontend"}, nil)
			},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inSvcName:     "frontend",
			inStorageName: "badTable!!!",
			wantedErr:     errDDBValueBadFormat,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockWs := mocks.NewMockwsSvcReader(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			tc.mockWs(mockWs)
			tc.mockStore(mockStore)
			opts := initStorageOpts{
				initStorageVars: initStorageVars{
					GlobalOpts: &GlobalOpts{
						appName: tc.inAppName,
					},
					StorageType: tc.inStorageType,
					StorageName: tc.inStorageName,
					StorageSvc:  tc.inSvcName,
				},
				ws:    mockWs,
				store: mockStore,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}
