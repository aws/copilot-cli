// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

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
		inNoSort      bool
		inNoLSI       bool
		inEngine      string

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
				m.EXPECT().WorkloadNames().Return([]string{"bad", "workspace"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {},

			inAppName:     "bowie",
			inStorageType: s3StorageType,
			inSvcName:     "frontend",
			inStorageName: "my-bucket",
			wantedErr:     errors.New("workload frontend not found in the workspace"),
		},
		"workspace error": {
			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().WorkloadNames().Return(nil, errors.New("wanted err"))
			},
			mockStore: func(m *mocks.Mockstore) {},

			inAppName:     "bowie",
			inStorageType: s3StorageType,
			inSvcName:     "frontend",
			inStorageName: "my-bucket",
			wantedErr:     errors.New("retrieve local workload names: wanted err"),
		},
		"successfully validates valid s3 bucket name": {
			mockWs:        func(m *mocks.MockwsAddonManager) {},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: s3StorageType,
			inStorageName: "my-bucket.4",
			wantedErr:     nil,
		},
		"successfully validates valid DDB table name": {
			mockWs:        func(m *mocks.MockwsAddonManager) {},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inStorageName: "my-cool_table.3",
			wantedErr:     nil,
		},
		"default to ddb name validation when storage type unspecified": {
			mockWs:        func(m *mocks.MockwsAddonManager) {},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: "",
			inStorageName: "my-cool_table.3",
			wantedErr:     nil,
		},
		"s3 bad character": {
			mockWs:        func(m *mocks.MockwsAddonManager) {},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: s3StorageType,
			inStorageName: "mybadbucket???",
			wantedErr:     errValueBadFormatWithPeriod,
		},
		"ddb bad character": {
			mockWs:        func(m *mocks.MockwsAddonManager) {},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inStorageName: "badTable!!!",
			wantedErr:     errValueBadFormatWithPeriodUnderscore,
		},
		"successfully validates partition key flag": {
			mockWs:        func(m *mocks.MockwsAddonManager) {},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inPartition:   "points:String",
			wantedErr:     nil,
		},
		"successfully validates sort key flag": {
			mockWs:        func(m *mocks.MockwsAddonManager) {},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inSort:        "userID:Number",
			wantedErr:     nil,
		},
		"successfully validates LSI": {
			mockWs:        func(m *mocks.MockwsAddonManager) {},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inLSISorts:    []string{"userID:Number", "data:Binary"},
			wantedErr:     nil,
		},
		"success on providing --no-sort": {
			mockWs:        func(m *mocks.MockwsAddonManager) {},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inNoSort:      true,
			wantedErr:     nil,
		},
		"success on providing --no-lsi": {
			mockWs:        func(m *mocks.MockwsAddonManager) {},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inNoLSI:       true,
			wantedErr:     nil,
		},
		"fails when --no-lsi and --lsi are both provided": {
			mockWs:        func(m *mocks.MockwsAddonManager) {},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inLSISorts:    []string{"userID:Number"},
			inNoLSI:       true,
			wantedErr:     fmt.Errorf("validate LSI configuration: cannot specify --no-lsi and --lsi options at once"),
		},
		"fails when --no-sort and --lsi are both provided": {
			mockWs:        func(m *mocks.MockwsAddonManager) {},
			mockStore:     func(m *mocks.Mockstore) {},
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inLSISorts:    []string{"userID:Number"},
			inNoSort:      true,
			wantedErr:     fmt.Errorf("validate LSI configuration: cannot specify --no-sort and --lsi options at once"),
		},
		"invalid database engine type": {
			inAppName: "meow",
			inEngine:  "mysql",

			mockWs:        func(m *mocks.MockwsAddonManager) {},
			mockStore:     func(m *mocks.Mockstore) {},

			wantedErr: errors.New("invalid engine type mysql: must be one of \"MySQL\", \"PostgreSQL\""),
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
					storageType:  tc.inStorageType,
					storageName:  tc.inStorageName,
					workloadName: tc.inSvcName,
					partitionKey: tc.inPartition,
					sortKey:      tc.inSort,
					lsiSorts:     tc.inLSISorts,
					noLSI:        tc.inNoLSI,
					noSort:       tc.inNoSort,
					engine:       tc.inEngine,
				},
				appName: tc.inAppName,
				ws:      mockWs,
				store:   mockStore,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
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
		wantedPartitionKey = "DogName:String"
		wantedSortKey      = "PhotoId:Number"

		wantedInitialDBName = "mydb"
		wantedDBEngine      = engineTypePostgreSQL
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

		inDBEngine      string
		inInitialDBName string

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
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Eq(storageTypesFriendly), gomock.Any()).Return(s3StorageType, nil)
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
		"asks for storage workload": {
			inAppName:     wantedAppName,
			inStorageName: wantedBucketName,
			inStorageType: s3StorageType,

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockCfg: func(m *mocks.MockwsSelector) {
				m.EXPECT().Workload(gomock.Eq(storageInitSvcPrompt), gomock.Any()).Return(wantedSvcName, nil)
			},

			wantedErr: nil,
		},
		"error if svc not returned": {
			inAppName:     wantedAppName,
			inStorageName: wantedBucketName,
			inStorageType: s3StorageType,

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockCfg: func(m *mocks.MockwsSelector) {
				m.EXPECT().Workload(gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},

			wantedErr: fmt.Errorf("retrieve local workload names: some error"),
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
					attributeTypes,
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
					attributeTypes,
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
					gomock.Eq(attributeTypes),
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
				storageName:  wantedTableName,
				workloadName: wantedSvcName,
				storageType:  dynamoDBStorageType,

				partitionKey: wantedPartitionKey,
				sortKey:      wantedSortKey,
				noLSI:        false,
				lsiSorts:     []string{"Email:String"},
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
				storageName:  wantedTableName,
				workloadName: wantedSvcName,
				storageType:  dynamoDBStorageType,

				partitionKey: wantedPartitionKey,
				sortKey:      wantedSortKey,
				noLSI:        true,
			},
		},
		"noLSI is set correctly if no sort key": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					gomock.Eq(storageInitDDBSortKeyConfirm),
					gomock.Eq(storageInitDDBSortKeyHelp),
					gomock.Any(),
				).Return(false, nil)
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedVars: &initStorageVars{
				storageName:  wantedTableName,
				workloadName: wantedSvcName,
				storageType:  dynamoDBStorageType,

				partitionKey: wantedPartitionKey,
				noLSI:        true,
				noSort:       true,
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
			inLSISorts:    []string{"email:String"},

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockCfg:    func(m *mocks.MockwsSelector) {},

			wantedErr: nil,
		},
		"asks for engine if not specified": {
			inAppName:       wantedAppName,
			inSvcName:       wantedSvcName,
			inStorageName:   wantedBucketName,

			inStorageType:   rdsStorageType,
			inInitialDBName: wantedInitialDBName,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq(storageInitRDSDBEnginePrompt), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedDBEngine, nil)
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedVars: &initStorageVars{
				storageType:   rdsStorageType,
				storageName:   wantedBucketName,
				workloadName:  wantedSvcName,
				initialDBName: wantedInitialDBName,
				engine:        wantedDBEngine,
			},
		},
		"error if engine not gotten": {
			inAppName:       wantedAppName,
			inSvcName:       wantedSvcName,
			inStorageName:   wantedBucketName,

			inStorageType:   rdsStorageType,
			inInitialDBName: wantedInitialDBName,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(storageInitRDSDBEnginePrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedErr: errors.New("select database engine: some error"),
		},
		"asks for initial database name": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageName: wantedBucketName,

			inStorageType: rdsStorageType,
			inDBEngine:    wantedDBEngine,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(storageInitRDSInitialDBNamePrompt), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedInitialDBName, nil)
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedVars: &initStorageVars{
				storageType:   rdsStorageType,
				storageName:   wantedBucketName,
				workloadName:  wantedSvcName,
				engine:        wantedDBEngine,
				initialDBName: wantedInitialDBName,
			},
		},
		"error if initial database name not gotten": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageName: wantedBucketName,

			inStorageType: rdsStorageType,
			inDBEngine:    wantedDBEngine,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(storageInitRDSInitialDBNamePrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			mockCfg: func(m *mocks.MockwsSelector) {},

			wantedErr: fmt.Errorf("input initial database name: some error"),
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
					storageType:  tc.inStorageType,
					storageName:  tc.inStorageName,
					workloadName: tc.inSvcName,
					partitionKey: tc.inPartition,
					sortKey:      tc.inSort,
					lsiSorts:     tc.inLSISorts,
					noLSI:        tc.inNoLSI,
					noSort:       tc.inNoSort,

					engine:        tc.inDBEngine,
					initialDBName: tc.inInitialDBName,
				},
				appName: tc.inAppName,
				sel:     mockConfig,
				prompt:  mockPrompt,
			}
			tc.mockPrompt(mockPrompt)
			tc.mockCfg(mockConfig)
			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
			if tc.wantedVars != nil {
				require.Equal(t, *tc.wantedVars, opts.initStorageVars)
			}
		})
	}
}

func TestStorageInitOpts_Execute(t *testing.T) {
	const (
		wantedAppName      = "ddos"
		wantedSvcName      = "frontend"
		wantedPartitionKey = "DogName:String"
		wantedSortKey      = "PhotoId:Number"
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

		inEngine            string
		inInitialDBName     string
		inParameterGroup    string

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
			inLSISorts:    []string{"goodness:Number"},

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().WriteAddon(gomock.Any(), wantedSvcName, "my-table").Return("/frontend/addons/my-table.yml", nil)
			},

			wantedErr: nil,
		},
		"happy calls for RDS": {
			inSvcName: wantedSvcName,

			inStorageType:      rdsStorageType,
			inStorageName:      "mycluster",
			inEngine:           engineTypeMySQL,
			inParameterGroup:    "mygroup",

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().WriteAddon(gomock.Any(), wantedSvcName, "mycluster").Return("/frontend/addons/mycluster.yml", nil)
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
					storageType:  tc.inStorageType,
					storageName:  tc.inStorageName,
					workloadName: tc.inSvcName,
					partitionKey: tc.inPartition,
					sortKey:      tc.inSort,
					lsiSorts:     tc.inLSISorts,
					noLSI:        tc.inNoLSI,
					noSort:       tc.inNoSort,

					engine:         tc.inEngine,
					parameterGroup: tc.inParameterGroup,
				},
				appName: tc.inAppName,
				ws:      mockAddon,
			}
			tc.mockWs(mockAddon)
			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}

}
