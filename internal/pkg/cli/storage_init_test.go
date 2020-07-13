// Copyright Amazon, Inc. or its affiliates. All rights reserved.

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestStorageInitOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppName     string
		inStorageType string
		inSvcName     string
		inStorageName string
		inPartition   string
		inSort        string
		inLSISorts    []string

		mockWs    func(m *mocks.MockwsAddonManager)
		mockStore func(m *mocks.Mockstore)

		wantedErr error
	}{
		"no app in workspace": {
			mockWs:    func(m *mocks.MockwsAddonManager) {},
			mockStore: func(m *mocks.Mockstore) {},

			wantedErr: errNoAppInWorkspace,
		},
		"svc not in workspace": {
			mockWs: func(m *mocks.MockwsAddonManager) {
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
			mockWs: func(m *mocks.MockwsAddonManager) {
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
			mockWs: func(m *mocks.MockwsAddonManager) {
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
			mockWs: func(m *mocks.MockwsAddonManager) {
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
			mockWs: func(m *mocks.MockwsAddonManager) {
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
			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().ServiceNames().Return([]string{"frontend"}, nil)
			},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: s3StorageType,
			inSvcName:     "frontend",
			inStorageName: "mybadbucket???",
			wantedErr:     errValueBadFormatWithPeriod,
		},
		"ddb bad character": {
			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().ServiceNames().Return([]string{"frontend"}, nil)
			},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inSvcName:     "frontend",
			inStorageName: "badTable!!!",
			wantedErr:     errValueBadFormatWithPeriodUnderscore,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockWs := mocks.NewMockwsAddonManager(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			tc.mockWs(mockWs)
			tc.mockStore(mockStore)
			opts := initStorageOpts{
				initStorageVars: initStorageVars{
					GlobalOpts: &GlobalOpts{
						appName: tc.inAppName,
					},
					storageType:  tc.inStorageType,
					storageName:  tc.inStorageName,
					storageSvc:   tc.inSvcName,
					partitionKey: tc.inPartition,
					sortKey:      tc.inSort,
					lsiSorts:     tc.inLSISorts,
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

func TestStorageInitOpts_Ask(t *testing.T) {
	const (
		wantedAppName      = "ddos"
		wantedSvcName      = "frontend"
		wantedBucketName   = "coolBucket"
		wantedTableName    = "coolTable"
		wantedPartitionKey = "DogName:S"
		wantedSortKey      = "PhotoId:N"
	)
	testCases := map[string]struct {
		inAppName     string
		inStorageType string
		inSvcName     string
		inStorageName string
		inPartition   string
		inSort        string
		inLSISorts    []string
		inNoLSI       bool
		inNoSort      bool

		mockPrompt func(m *mocks.Mockprompter)
		mockCfg    func(m *mocks.MockwsSelector)

		wantedErr error

		wantedVars *initStorageVars
	}{
		"Asks for storage type": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageName: wantedBucketName,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Eq(storageTypes), gomock.Any()).Return(s3StorageType, nil)
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedErr: nil,
		},
		"error if storage type not gotten": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageName: wantedBucketName,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedErr: fmt.Errorf("select storage type: some error"),
		},
		"asks for storage svc": {
			inAppName:     wantedAppName,
			inStorageName: wantedBucketName,
			inStorageType: s3StorageType,

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockCfg: func(m *mocks.MockwsSelector) {
				m.EXPECT().Service(gomock.Eq(storageInitSvcPrompt), gomock.Any()).Return(wantedSvcName, nil)
			},

			wantedErr: nil,
		},
		"error if svc not returned": {
			inAppName:     wantedAppName,
			inStorageName: wantedBucketName,
			inStorageType: s3StorageType,

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockCfg: func(m *mocks.MockwsSelector) {
				m.EXPECT().Service(gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},

			wantedErr: fmt.Errorf("retrieve local service names: some error"),
		},
		"asks for storage name": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: s3StorageType,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(
					fmt.Sprintf(fmtStorageInitNamePrompt,
						color.HighlightUserInput(s3BucketFriendlyText),
					),
				),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedBucketName, nil)
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedErr: nil,
		},
		"error if storage name not returned": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: s3StorageType,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedErr: fmt.Errorf("input storage name: some error"),
		},
		"asks for partition key if not specified": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inSort:        wantedSortKey,
			inNoLSI:       true,

			mockPrompt: func(m *mocks.Mockprompter) {
				keyPrompt := fmt.Sprintf(fmtStorageInitDDBKeyPrompt,
					color.HighlightUserInput("partition key"),
					color.HighlightUserInput(dynamoDBStorageType),
				)
				keyTypePrompt := fmt.Sprintf(fmtStorageInitDDBKeyTypePrompt, ddbKeyString)
				m.EXPECT().Get(gomock.Eq(keyPrompt),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedPartitionKey, nil)
				m.EXPECT().SelectOne(gomock.Eq(keyTypePrompt),
					gomock.Any(),
					attributeTypesLong,
					gomock.Any(),
				).Return(ddbStringType, nil)
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedErr: nil,
		},
		"error if fail to return partition key": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedErr: fmt.Errorf("get DDB partition key: some error"),
		},
		"error if fail to return partition key type": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inSort:        wantedSortKey,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedPartitionKey, nil)
				m.EXPECT().SelectOne(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedErr: fmt.Errorf("get DDB partition key datatype: some error"),
		},
		"ask for sort key if not specified": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inNoLSI:       true,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					gomock.Eq(storageInitDDBSortKeyConfirm),
					gomock.Any(),
					gomock.Any(),
				).Return(true, nil)
				keyPrompt := fmt.Sprintf(fmtStorageInitDDBKeyPrompt,
					color.HighlightUserInput("sort key"),
					color.HighlightUserInput(dynamoDBStorageType),
				)
				keyTypePrompt := fmt.Sprintf(fmtStorageInitDDBKeyTypePrompt, ddbKeyString)
				m.EXPECT().Get(gomock.Eq(keyPrompt),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedPartitionKey, nil)
				m.EXPECT().SelectOne(gomock.Eq(keyTypePrompt),
					gomock.Any(),
					attributeTypesLong,
					gomock.Any(),
				).Return(ddbStringType, nil)
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedErr: nil,
		},
		"error if fail to confirm add sort key": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					gomock.Eq(storageInitDDBSortKeyConfirm),
					gomock.Any(),
					gomock.Any(),
				).Return(false, errors.New("some error"))
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedErr: fmt.Errorf("confirm DDB sort key: some error"),
		},
		"error if fail to return sort key": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					gomock.Eq(storageInitDDBSortKeyConfirm),
					gomock.Any(),
					gomock.Any(),
				).Return(true, nil)
				m.EXPECT().Get(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedErr: fmt.Errorf("get DDB sort key: some error"),
		},
		"error if fail to return sort key type": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					gomock.Eq(storageInitDDBSortKeyConfirm),
					gomock.Any(),
					gomock.Any(),
				).Return(true, nil)
				m.EXPECT().Get(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedPartitionKey, nil)
				m.EXPECT().SelectOne(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedErr: fmt.Errorf("get DDB sort key datatype: some error"),
		},
		"don't ask for sort key if no-sort specified": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inNoSort:      true,
			inNoLSI:       true,

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockCfg:    func(m *mocks.MockwsSelector) {},

			wantedErr: nil,
		},
		"ok if --no-lsi and --sort-key are both specified": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,
			inNoLSI:       true,

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockCfg:    func(m *mocks.MockwsSelector) {},

			wantedErr: nil,
		},
		"don't ask about LSI if no-sort is specified": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inNoSort:      true,

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockCfg:    func(m *mocks.MockwsSelector) {},

			wantedErr: nil,
		},
		"ask for LSI if not specified": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,

			mockPrompt: func(m *mocks.Mockprompter) {
				lsiTypePrompt := fmt.Sprintf(fmtStorageInitDDBKeyTypePrompt, color.Emphasize("alternate sort key"))
				lsiTypeHelp := fmt.Sprintf(fmtStorageInitDDBKeyTypeHelp, "alternate sort key")
				m.EXPECT().Confirm(
					gomock.Eq(storageInitDDBLSIPrompt),
					gomock.Eq(storageInitDDBLSIHelp),
					gomock.Any(),
				).Return(true, nil)
				m.EXPECT().Get(
					gomock.Eq(storageInitDDBLSINamePrompt),
					gomock.Eq(storageInitDDBLSINameHelp),
					gomock.Any(),
					gomock.Any(),
				).Return("Email", nil)
				m.EXPECT().SelectOne(
					gomock.Eq(lsiTypePrompt),
					gomock.Eq(lsiTypeHelp),
					gomock.Eq(attributeTypesLong),
					gomock.Any(),
				).Return(ddbStringType, nil)
				m.EXPECT().Confirm(
					gomock.Eq(storageInitDDBMoreLSIPrompt),
					gomock.Eq(storageInitDDBLSIHelp),
					gomock.Any(),
				).Return(false, nil)
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedVars: &initStorageVars{
				GlobalOpts: &GlobalOpts{
					appName: wantedAppName,
				},
				storageName: wantedTableName,
				storageSvc:  wantedSvcName,
				storageType: dynamoDBStorageType,

				partitionKey: wantedPartitionKey,
				sortKey:      wantedSortKey,
				noLSI:        false,
				lsiSorts:     []string{"Email:S"},
			},
		},
		"noLSI is set correctly if no lsis specified": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					gomock.Eq(storageInitDDBLSIPrompt),
					gomock.Eq(storageInitDDBLSIHelp),
					gomock.Any(),
				).Return(false, nil)
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedVars: &initStorageVars{
				GlobalOpts: &GlobalOpts{
					appName: wantedAppName,
				},
				storageName: wantedTableName,
				storageSvc:  wantedSvcName,
				storageType: dynamoDBStorageType,

				partitionKey: wantedPartitionKey,
				sortKey:      wantedSortKey,
				noLSI:        true,
			},
		},
		"error if lsi name misspecified": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					gomock.Eq(storageInitDDBLSIPrompt),
					gomock.Any(),
					gomock.Any(),
				).Return(true, nil)
				m.EXPECT().Get(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedErr: fmt.Errorf("get DDB alternate sort key name: some error"),
		},
		"errors if fail to confirm lsi": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(false, errors.New("some error"))
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedErr: fmt.Errorf("confirm add alternate sort key: some error"),
		},
		"error if lsi type misspecified": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(true, nil)
				m.EXPECT().Get(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("cool", nil)
				m.EXPECT().SelectOne(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))

			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedErr: fmt.Errorf("get DDB alternate sort key type: some error"),
		},
		"no error or asks when fully specified": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,
			inLSISorts:    []string{"email:S"},

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockCfg:    func(m *mocks.MockwsSelector) {},

			wantedErr: nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mocks.NewMockprompter(ctrl)
			mockConfig := mocks.NewMockwsSelector(ctrl)
			opts := initStorageOpts{
				initStorageVars: initStorageVars{
					GlobalOpts: &GlobalOpts{
						appName: tc.inAppName,
						prompt:  mockPrompt,
					},
					storageType:  tc.inStorageType,
					storageName:  tc.inStorageName,
					storageSvc:   tc.inSvcName,
					partitionKey: tc.inPartition,
					sortKey:      tc.inSort,
					lsiSorts:     tc.inLSISorts,
					noLSI:        tc.inNoLSI,
					noSort:       tc.inNoSort,
				},
				sel: mockConfig,
			}
			tc.mockPrompt(mockPrompt)
			tc.mockCfg(mockConfig)
			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Nil(t, err)
			}
			if tc.wantedVars != nil {
				tc.wantedVars.prompt = opts.prompt
				require.Equal(t, *tc.wantedVars, opts.initStorageVars)
			}
		})
	}
}

func TestStorageInitOpts_Execute(t *testing.T) {
	const (
		wantedAppName      = "ddos"
		wantedSvcName      = "frontend"
		wantedBucketName   = "coolBucket"
		wantedTableName    = "coolTable"
		wantedPartitionKey = "DogName:S"
		wantedSortKey      = "PhotoId:N"
	)
	fileExistsError := &workspace.ErrFileExists{FileName: "my-file"}
	testCases := map[string]struct {
		inAppName     string
		inStorageType string
		inSvcName     string
		inStorageName string
		inPartition   string
		inSort        string
		inLSISorts    []string
		inNoLSI       bool
		inNoSort      bool

		mockWs func(m *mocks.MockwsAddonManager)

		wantedErr error
	}{
		"happy calls for S3": {
			inAppName:     wantedAppName,
			inStorageType: s3StorageType,
			inSvcName:     wantedSvcName,
			inStorageName: "my-bucket",

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().WriteAddon(gomock.Any(), wantedSvcName, "my-bucket").Return("/frontend/addons/my-bucket.yml", nil)
			},

			wantedErr: nil,
		},
		"happy calls for DDB": {
			inAppName:     wantedAppName,
			inStorageType: dynamoDBStorageType,
			inSvcName:     wantedSvcName,
			inStorageName: "my-table",
			inNoLSI:       true,
			inNoSort:      true,
			inPartition:   wantedPartitionKey,

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().WriteAddon(gomock.Any(), wantedSvcName, "my-table").Return("/frontend/addons/my-table.yml", nil)
			},

			wantedErr: nil,
		},
		"happy calls for DDB with LSI": {
			inAppName:     wantedAppName,
			inStorageType: dynamoDBStorageType,
			inSvcName:     wantedSvcName,
			inStorageName: "my-table",
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,
			inLSISorts:    []string{"goodness:N"},

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().WriteAddon(gomock.Any(), wantedSvcName, "my-table").Return("/frontend/addons/my-table.yml", nil)
			},

			wantedErr: nil,
		},
		"error addon exists": {
			inAppName:     wantedAppName,
			inStorageType: s3StorageType,
			inSvcName:     wantedSvcName,
			inStorageName: "my-bucket",

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().WriteAddon(gomock.Any(), wantedSvcName, "my-bucket").Return("", fileExistsError)
			},

			wantedErr: fmt.Errorf("addon already exists: %w", fileExistsError),
		},
		"unrecognized error handled": {
			inAppName:     wantedAppName,
			inStorageType: s3StorageType,
			inSvcName:     wantedSvcName,
			inStorageName: "my-bucket",

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().WriteAddon(gomock.Any(), wantedSvcName, "my-bucket").Return("", errors.New("some error"))
			},

			wantedErr: fmt.Errorf("some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockAddon := mocks.NewMockwsAddonManager(ctrl)
			opts := initStorageOpts{
				initStorageVars: initStorageVars{
					GlobalOpts: &GlobalOpts{
						appName: tc.inAppName,
					},
					storageType:  tc.inStorageType,
					storageName:  tc.inStorageName,
					storageSvc:   tc.inSvcName,
					partitionKey: tc.inPartition,
					sortKey:      tc.inSort,
					lsiSorts:     tc.inLSISorts,
					noLSI:        tc.inNoLSI,
					noSort:       tc.inNoSort,
				},
				ws: mockAddon,
			}
			tc.mockWs(mockAddon)
			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}

}
