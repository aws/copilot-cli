// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/term/prompt"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type mockStorageInitValidate struct {
	ws    *mocks.MockwsAddonManager
	store *mocks.Mockstore
}

func TestStorageInitOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppName           string
		inStorageType       string
		inSvcName           string
		inStorageName       string
		inLifecycle         string
		inPartition         string
		inSort              string
		inLSISorts          []string
		inNoSort            bool
		inNoLSI             bool
		inServerlessVersion string
		inEngine            string

		mock      func(m *mockStorageInitValidate)
		mockWs    func(m *mocks.MockwsAddonManager)
		mockStore func(m *mocks.Mockstore)

		wantedErr error
	}{
		"no app in workspace": {
			mock:      func(m *mockStorageInitValidate) {},
			wantedErr: errNoAppInWorkspace,
		},
		"svc not in workspace": {
			mock: func(m *mockStorageInitValidate) {
				m.ws.EXPECT().WorkloadExists(gomock.Eq("frontend")).Return(false, nil)
			},
			inAppName:     "bowie",
			inStorageType: s3StorageType,
			inSvcName:     "frontend",
			inStorageName: "my-bucket",
			wantedErr:     errors.New("workload frontend not found in the workspace"),
		},
		"workspace error": {
			mock: func(m *mockStorageInitValidate) {
				m.ws.EXPECT().WorkloadExists(gomock.Eq("frontend")).Return(false, errors.New("wanted err"))
			},
			inAppName:     "bowie",
			inStorageType: s3StorageType,
			inSvcName:     "frontend",
			inStorageName: "my-bucket",
			wantedErr:     errors.New("check if frontend exists in the workspace: wanted err"),
		},
		"bad lifecycle option": {
			inAppName:   "bowie",
			inLifecycle: "weird input",
			mock:        func(m *mockStorageInitValidate) {},
			wantedErr:   errors.New(`invalid lifecycle; must be one of "workload" or "environment"`),
		},
		"successfully validates valid s3 bucket name": {
			inAppName:     "bowie",
			inStorageType: s3StorageType,
			inStorageName: "my-bucket.4",
			mock:          func(m *mockStorageInitValidate) {},
		},
		"successfully validates valid DDB table name": {
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inStorageName: "my-cool_table.3",
			mock:          func(m *mockStorageInitValidate) {},
		},
		"default to ddb name validation when storage type unspecified": {
			inAppName:     "bowie",
			inStorageType: "",
			inStorageName: "my-cool_table.3",
			mock:          func(m *mockStorageInitValidate) {},
		},
		"s3 bad character": {
			inAppName:     "bowie",
			inStorageType: s3StorageType,
			inStorageName: "mybadbucket???",
			mock:          func(m *mockStorageInitValidate) {},
			wantedErr:     errValueBadFormatWithPeriod,
		},
		"ddb bad character": {
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inStorageName: "badTable!!!",
			mock:          func(m *mockStorageInitValidate) {},
			wantedErr:     errValueBadFormatWithPeriodUnderscore,
		},
		"successfully validates partition key flag": {
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inPartition:   "points:String",
			mock:          func(m *mockStorageInitValidate) {},
		},
		"successfully validates sort key flag": {
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inSort:        "userID:Number",
			mock:          func(m *mockStorageInitValidate) {},
		},
		"successfully validates LSI": {
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inLSISorts:    []string{"userID:Number", "data:Binary"},
			mock:          func(m *mockStorageInitValidate) {},
		},
		"success on providing --no-sort": {
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inNoSort:      true,
			mock:          func(m *mockStorageInitValidate) {},
		},
		"success on providing --no-lsi": {
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inNoLSI:       true,
			mock:          func(m *mockStorageInitValidate) {},
		},
		"fails when --no-lsi and --lsi are both provided": {
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inLSISorts:    []string{"userID:Number"},
			inNoLSI:       true,
			mock:          func(m *mockStorageInitValidate) {},
			wantedErr:     fmt.Errorf("validate LSI configuration: cannot specify --no-lsi and --lsi options at once"),
		},
		"fails when --no-sort and --lsi are both provided": {
			inAppName:     "bowie",
			inStorageType: dynamoDBStorageType,
			inLSISorts:    []string{"userID:Number"},
			inNoSort:      true,
			mock:          func(m *mockStorageInitValidate) {},
			wantedErr:     fmt.Errorf("validate LSI configuration: cannot specify --no-sort and --lsi options at once"),
		},
		"invalid database engine type": {
			inAppName: "meow",
			inEngine:  "mysql",

			mock:      func(m *mockStorageInitValidate) {},
			wantedErr: errors.New("invalid engine type mysql: must be one of \"MySQL\", \"PostgreSQL\""),
		},
		"successfully validates aurora serverless version v1": {
			inAppName:           "bowie",
			inStorageType:       rdsStorageType,
			inServerlessVersion: auroraServerlessVersionV1,
			mock:                func(m *mockStorageInitValidate) {},
		},
		"successfully validates aurora serverless version v2": {
			inAppName:           "bowie",
			inStorageType:       rdsStorageType,
			inServerlessVersion: auroraServerlessVersionV2,
			mock:                func(m *mockStorageInitValidate) {},
		},
		"invalid aurora serverless version": {
			inAppName:           "bowie",
			inStorageType:       rdsStorageType,
			inServerlessVersion: "weird-serverless-version",
			mock:                func(m *mockStorageInitValidate) {},
			wantedErr:           errors.New("invalid Aurora Serverless version weird-serverless-version: must be one of \"v1\", \"v2\""),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := mockStorageInitValidate{
				ws:    mocks.NewMockwsAddonManager(ctrl),
				store: mocks.NewMockstore(ctrl),
			}
			tc.mock(&m)
			opts := initStorageOpts{
				initStorageVars: initStorageVars{
					storageType:             tc.inStorageType,
					storageName:             tc.inStorageName,
					workloadName:            tc.inSvcName,
					lifecycle:               tc.inLifecycle,
					partitionKey:            tc.inPartition,
					sortKey:                 tc.inSort,
					lsiSorts:                tc.inLSISorts,
					noLSI:                   tc.inNoLSI,
					noSort:                  tc.inNoSort,
					auroraServerlessVersion: tc.inServerlessVersion,
					rdsEngine:               tc.inEngine,
				},
				appName: tc.inAppName,
				ws:      m.ws,
				store:   m.store,
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

type mockStorageInitAsk struct {
	prompt *mocks.Mockprompter
	sel    *mocks.MockwsSelector
	ws     *mocks.MockwsAddonManager
}

func TestStorageInitOpts_Ask(t *testing.T) {
	const (
		wantedAppName      = "ddos"
		wantedSvcName      = "frontend"
		wantedBucketName   = "coolBucket"
		wantedTableName    = "coolTable"
		wantedPartitionKey = "DogName:String"
		wantedSortKey      = "PhotoId:Number"
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

		inServerlessVersion string
		inDBEngine          string
		inInitialDBName     string

		mock func(m *mockStorageInitAsk)

		wantedErr  error
		wantedVars *initStorageVars
	}{
		"asks for storage type": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageName: wantedBucketName,

			mock: func(m *mockStorageInitAsk) {
				options := []prompt.Option{
					{
						Value: dynamoDBStorageTypeOption,
						Hint:  "NoSQL",
					},
					{
						Value: s3StorageTypeOption,
						Hint:  "Objects",
					},
					{
						Value: rdsStorageTypeOption,
						Hint:  "SQL",
					},
				}
				m.prompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Eq(options), gomock.Any()).Return(s3StorageType, nil)
			},
		},
		"error if storage type not gotten": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageName: wantedBucketName,

			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("select storage type: some error"),
		},
		"asks for storage workload": {
			inAppName:     wantedAppName,
			inStorageName: wantedBucketName,
			inStorageType: s3StorageType,
			mock: func(m *mockStorageInitAsk) {
				m.sel.EXPECT().Workload(gomock.Eq(storageInitSvcPrompt), gomock.Any()).Return(wantedSvcName, nil)
			},
		},
		"error if svc not returned": {
			inAppName:     wantedAppName,
			inStorageName: wantedBucketName,
			inStorageType: s3StorageType,
			mock: func(m *mockStorageInitAsk) {
				m.sel.EXPECT().Workload(gomock.Any(), gomock.Any()).Return("", errors.New("some error"))

			},
			wantedErr: fmt.Errorf("retrieve local workload names: some error"),
		},
		"asks for storage name": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: s3StorageType,

			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Get(gomock.Eq(fmt.Sprintf(fmtStorageInitNamePrompt, color.HighlightUserInput(s3BucketFriendlyText))),
					gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedBucketName, nil)
			},
		},
		"error if storage name not returned": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: s3StorageType,

			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("input storage name: some error"),
		},
		"no error or asks when fully specified": {
			inAppName:     wantedAppName,
			inSvcName:     wantedSvcName,
			inStorageType: dynamoDBStorageType,
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,
			inLSISorts:    []string{"email:String"},
			mock:          func(m *mockStorageInitAsk) {},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mockStorageInitAsk{
				prompt: mocks.NewMockprompter(ctrl),
				sel:    mocks.NewMockwsSelector(ctrl),
				ws:     mocks.NewMockwsAddonManager(ctrl),
			}
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

					auroraServerlessVersion: tc.inServerlessVersion,
					rdsEngine:               tc.inDBEngine,
					rdsInitialDBName:        tc.inInitialDBName,
				},
				appName: tc.inAppName,
				sel:     m.sel,
				prompt:  m.prompt,
				ws:      m.ws,
			}
			tc.mock(&m)
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

func TestStorageInitOpts_AskDDB(t *testing.T) {
	const (
		wantedSvcName      = "frontend"
		wantedTableName    = "coolTable"
		wantedPartitionKey = "DogName:String"
		wantedSortKey      = "PhotoId:Number"
	)
	testCases := map[string]struct {
		inPartition string
		inSort      string
		inLSISorts  []string
		inNoLSI     bool
		inNoSort    bool

		inServerlessVersion string
		inDBEngine          string
		inInitialDBName     string

		mock func(m *mockStorageInitAsk)

		wantedErr  error
		wantedVars *initStorageVars
	}{
		"asks for partition key if not specified": {
			inSort:  wantedSortKey,
			inNoLSI: true,

			mock: func(m *mockStorageInitAsk) {
				keyPrompt := fmt.Sprintf(fmtStorageInitDDBKeyPrompt,
					color.HighlightUserInput("partition key"),
					color.HighlightUserInput(dynamoDBStorageType),
				)
				keyTypePrompt := fmt.Sprintf(fmtStorageInitDDBKeyTypePrompt, ddbKeyString)
				m.prompt.EXPECT().Get(gomock.Eq(keyPrompt),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedPartitionKey, nil)
				m.prompt.EXPECT().SelectOne(gomock.Eq(keyTypePrompt),
					gomock.Any(),
					attributeTypes,
					gomock.Any(),
				).Return(ddbStringType, nil)
			},
		},
		"error if fail to return partition key": {
			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Get(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get DDB partition key: some error"),
		},
		"error if fail to return partition key type": {
			inSort: wantedSortKey,
			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Get(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedPartitionKey, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get DDB partition key datatype: some error"),
		},
		"ask for sort key if not specified": {
			inPartition: wantedPartitionKey,
			inNoLSI:     true,
			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Confirm(
					gomock.Eq(storageInitDDBSortKeyConfirm),
					gomock.Any(),
					gomock.Any(),
				).Return(true, nil)
				keyPrompt := fmt.Sprintf(fmtStorageInitDDBKeyPrompt,
					color.HighlightUserInput("sort key"),
					color.HighlightUserInput(dynamoDBStorageType),
				)
				keyTypePrompt := fmt.Sprintf(fmtStorageInitDDBKeyTypePrompt, ddbKeyString)
				m.prompt.EXPECT().Get(gomock.Eq(keyPrompt),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedPartitionKey, nil)
				m.prompt.EXPECT().SelectOne(gomock.Eq(keyTypePrompt),
					gomock.Any(),
					attributeTypes,
					gomock.Any(),
				).Return(ddbStringType, nil)
			},
		},
		"error if fail to confirm add sort key": {
			inPartition: wantedPartitionKey,
			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Confirm(
					gomock.Eq(storageInitDDBSortKeyConfirm),
					gomock.Any(),
					gomock.Any(),
				).Return(false, errors.New("some error"))
			},
			wantedErr: fmt.Errorf("confirm DDB sort key: some error"),
		},
		"error if fail to return sort key": {
			inPartition: wantedPartitionKey,
			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Confirm(
					gomock.Eq(storageInitDDBSortKeyConfirm),
					gomock.Any(),
					gomock.Any(),
				).Return(true, nil)
				m.prompt.EXPECT().Get(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get DDB sort key: some error"),
		},
		"error if fail to return sort key type": {
			inPartition: wantedPartitionKey,
			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Confirm(
					gomock.Eq(storageInitDDBSortKeyConfirm),
					gomock.Any(),
					gomock.Any(),
				).Return(true, nil)
				m.prompt.EXPECT().Get(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedPartitionKey, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get DDB sort key datatype: some error"),
		},
		"don't ask for sort key if no-sort specified": {
			inPartition: wantedPartitionKey,
			inNoSort:    true,
			inNoLSI:     true,
			mock:        func(m *mockStorageInitAsk) {},
		},
		"ok if --no-lsi and --sort-key are both specified": {
			inPartition: wantedPartitionKey,
			inSort:      wantedSortKey,
			inNoLSI:     true,
			mock:        func(m *mockStorageInitAsk) {},
		},
		"don't ask about LSI if no-sort is specified": {
			inPartition: wantedPartitionKey,
			inNoSort:    true,
			mock:        func(m *mockStorageInitAsk) {},
		},
		"ask for LSI if not specified": {
			inPartition: wantedPartitionKey,
			inSort:      wantedSortKey,
			mock: func(m *mockStorageInitAsk) {
				lsiTypePrompt := fmt.Sprintf(fmtStorageInitDDBKeyTypePrompt, color.Emphasize("alternate sort key"))
				lsiTypeHelp := fmt.Sprintf(fmtStorageInitDDBKeyTypeHelp, "alternate sort key")
				m.prompt.EXPECT().Confirm(
					gomock.Eq(storageInitDDBLSIPrompt),
					gomock.Eq(storageInitDDBLSIHelp),
					gomock.Any(),
				).Return(true, nil)
				m.prompt.EXPECT().Get(
					gomock.Eq(storageInitDDBLSINamePrompt),
					gomock.Eq(storageInitDDBLSINameHelp),
					gomock.Any(),
					gomock.Any(),
				).Return("Email", nil)
				m.prompt.EXPECT().SelectOne(
					gomock.Eq(lsiTypePrompt),
					gomock.Eq(lsiTypeHelp),
					gomock.Eq(attributeTypes),
					gomock.Any(),
				).Return(ddbStringType, nil)
				m.prompt.EXPECT().Confirm(
					gomock.Eq(storageInitDDBMoreLSIPrompt),
					gomock.Eq(storageInitDDBLSIHelp),
					gomock.Any(),
				).Return(false, nil)
			},
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
			inPartition: wantedPartitionKey,
			inSort:      wantedSortKey,
			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Confirm(
					gomock.Eq(storageInitDDBLSIPrompt),
					gomock.Eq(storageInitDDBLSIHelp),
					gomock.Any(),
				).Return(false, nil)
			},
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
			inPartition: wantedPartitionKey,
			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Confirm(
					gomock.Eq(storageInitDDBSortKeyConfirm),
					gomock.Eq(storageInitDDBSortKeyHelp),
					gomock.Any(),
				).Return(false, nil)
			},
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
			inPartition: wantedPartitionKey,
			inSort:      wantedSortKey,
			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Confirm(
					gomock.Eq(storageInitDDBLSIPrompt),
					gomock.Any(),
					gomock.Any(),
				).Return(true, nil)
				m.prompt.EXPECT().Get(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get DDB alternate sort key name: some error"),
		},
		"errors if fail to confirm lsi": {
			inPartition: wantedPartitionKey,
			inSort:      wantedSortKey,
			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Confirm(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(false, errors.New("some error"))
			},
			wantedErr: fmt.Errorf("confirm add alternate sort key: some error"),
		},
		"error if lsi type misspecified": {
			inPartition: wantedPartitionKey,
			inSort:      wantedSortKey,
			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Confirm(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(true, nil)
				m.prompt.EXPECT().Get(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("cool", nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))

			},
			wantedErr: fmt.Errorf("get DDB alternate sort key type: some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mockStorageInitAsk{
				prompt: mocks.NewMockprompter(ctrl),
			}
			tc.mock(&m)
			opts := initStorageOpts{
				initStorageVars: initStorageVars{
					storageType:  dynamoDBStorageType,
					storageName:  wantedTableName,
					workloadName: wantedSvcName,
					partitionKey: tc.inPartition,
					sortKey:      tc.inSort,
					lsiSorts:     tc.inLSISorts,
					noLSI:        tc.inNoLSI,
					noSort:       tc.inNoSort,
				},
				appName: "ddos",
				prompt:  m.prompt,
			}
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

func TestStorageInitOpts_AskRDS(t *testing.T) {
	const (
		wantedSvcName     = "frontend"
		wantedClusterName = "cookie"

		wantedServerlessVersion = auroraServerlessVersionV2
		wantedInitialDBName     = "mydb"
		wantedDBEngine          = engineTypePostgreSQL
	)
	testCases := map[string]struct {
		inStorageName string
		inPartition   string
		inSort        string
		inLSISorts    []string
		inNoLSI       bool
		inNoSort      bool

		inServerlessVersion string
		inDBEngine          string
		inInitialDBName     string

		mock func(m *mockStorageInitAsk)

		wantedErr  error
		wantedVars *initStorageVars
	}{
		"asks for cluster name for RDS storage": {
			inServerlessVersion: wantedServerlessVersion,
			inDBEngine:          wantedDBEngine,
			inInitialDBName:     wantedInitialDBName,

			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Get(
					gomock.Eq("What would you like to name this Database Cluster?"),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedClusterName, nil)
				m.ws.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(workspace.WorkloadManifest("type: Load Balanced Web Service"), nil)
			},
			wantedVars: &initStorageVars{
				storageType:             rdsStorageType,
				storageName:             wantedClusterName,
				workloadName:            wantedSvcName,
				auroraServerlessVersion: wantedServerlessVersion,
				rdsEngine:               wantedDBEngine,
				rdsInitialDBName:        wantedInitialDBName,
			},
		},
		"asks for engine if not specified": {
			inStorageName:       wantedClusterName,
			inServerlessVersion: wantedServerlessVersion,
			inInitialDBName:     wantedInitialDBName,

			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().SelectOne(gomock.Eq(storageInitRDSDBEnginePrompt), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedDBEngine, nil)
				m.ws.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(workspace.WorkloadManifest("type: Load Balanced Web Service"), nil)

			},
			wantedVars: &initStorageVars{
				storageType:             rdsStorageType,
				storageName:             wantedClusterName,
				workloadName:            wantedSvcName,
				auroraServerlessVersion: wantedServerlessVersion,
				rdsInitialDBName:        wantedInitialDBName,
				rdsEngine:               wantedDBEngine,
			},
		},
		"error if engine not gotten": {
			inStorageName:       wantedClusterName,
			inServerlessVersion: wantedServerlessVersion,
			inInitialDBName:     wantedInitialDBName,

			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().SelectOne(storageInitRDSDBEnginePrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
				m.ws.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(workspace.WorkloadManifest("type: Load Balanced Web Service"), nil)

			},
			wantedErr: errors.New("select database engine: some error"),
		},
		"asks for initial database name": {
			inStorageName:       wantedClusterName,
			inServerlessVersion: wantedServerlessVersion,
			inDBEngine:          wantedDBEngine,

			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Get(gomock.Eq(storageInitRDSInitialDBNamePrompt), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedInitialDBName, nil)
				m.ws.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(workspace.WorkloadManifest("type: Load Balanced Web Service"), nil)
			},
			wantedVars: &initStorageVars{
				storageType:             rdsStorageType,
				storageName:             wantedClusterName,
				workloadName:            wantedSvcName,
				auroraServerlessVersion: wantedServerlessVersion,
				rdsEngine:               wantedDBEngine,
				rdsInitialDBName:        wantedInitialDBName,
			},
		},
		"error if initial database name not gotten": {
			inStorageName:       wantedClusterName,
			inServerlessVersion: wantedServerlessVersion,
			inDBEngine:          wantedDBEngine,

			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Get(storageInitRDSInitialDBNamePrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
				m.ws.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(workspace.WorkloadManifest("type: Load Balanced Web Service"), nil)

			},
			wantedErr: fmt.Errorf("input initial database name: some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mockStorageInitAsk{
				prompt: mocks.NewMockprompter(ctrl),
				ws:     mocks.NewMockwsAddonManager(ctrl),
			}
			opts := initStorageOpts{
				initStorageVars: initStorageVars{
					storageType:  rdsStorageType,
					workloadName: wantedSvcName,
					storageName:  tc.inStorageName,

					auroraServerlessVersion: tc.inServerlessVersion,
					rdsEngine:               tc.inDBEngine,
					rdsInitialDBName:        tc.inInitialDBName,
				},
				appName: "ddos",
				prompt:  m.prompt,
				ws:      m.ws,
			}
			tc.mock(&m)
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

		inPartition string
		inSort      string
		inLSISorts  []string
		inNoLSI     bool
		inNoSort    bool

		inServerlessVersion string
		inEngine            string
		inInitialDBName     string
		inParameterGroup    string

		inLifecycle string

		mockWs         func(m *mocks.MockwsAddonManager)
		mockStore      func(m *mocks.Mockstore)
		mockWkldAbsent bool

		wantedErr error
	}{
		"happy calls for wkld S3": {
			inAppName:     wantedAppName,
			inStorageType: s3StorageType,
			inSvcName:     wantedSvcName,
			inStorageName: "my-bucket",
			inLifecycle:   lifecycleWorkloadLevel,

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Worker Service"), nil)
				m.EXPECT().WorkloadAddonFilePath(gomock.Eq(wantedSvcName), gomock.Eq("my-bucket.yml")).Return("mockPath")
				m.EXPECT().Write(gomock.Any(), "mockPath").Return("/frontend/addons/my-bucket.yml", nil)
			},

			wantedErr: nil,
		},
		"happy calls for wkld DDB": {
			inAppName:     wantedAppName,
			inStorageType: dynamoDBStorageType,
			inSvcName:     wantedSvcName,
			inStorageName: "my-table",
			inNoLSI:       true,
			inNoSort:      true,
			inPartition:   wantedPartitionKey,
			inLifecycle:   lifecycleWorkloadLevel,

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Backend Service"), nil)
				m.EXPECT().WorkloadAddonFilePath(gomock.Eq(wantedSvcName), gomock.Eq("my-table.yml")).Return("mockPath")
				m.EXPECT().Write(gomock.Any(), "mockPath").Return("/frontend/addons/my-table.yml", nil)
			},

			wantedErr: nil,
		},
		"happy calls for wkld DDB with LSI": {
			inAppName:     wantedAppName,
			inStorageType: dynamoDBStorageType,
			inSvcName:     wantedSvcName,
			inStorageName: "my-table",
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,
			inLSISorts:    []string{"goodness:Number"},
			inLifecycle:   lifecycleWorkloadLevel,

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Backend Service"), nil)
				m.EXPECT().WorkloadAddonFilePath(gomock.Eq(wantedSvcName), gomock.Eq("my-table.yml")).Return("mockPath")
				m.EXPECT().Write(gomock.Any(), "mockPath").Return("/frontend/addons/my-table.yml", nil)
			},

			wantedErr: nil,
		},
		"happy calls for wkld RDS with LBWS": {
			inSvcName:           wantedSvcName,
			inStorageType:       rdsStorageType,
			inStorageName:       "mycluster",
			inServerlessVersion: auroraServerlessVersionV1,
			inEngine:            engineTypeMySQL,
			inParameterGroup:    "mygroup",
			inLifecycle:         lifecycleWorkloadLevel,

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Load Balanced Web Service"), nil)
				m.EXPECT().WorkloadAddonFilePath(gomock.Eq(wantedSvcName), gomock.Eq("mycluster.yml")).Return("mockPath")
				m.EXPECT().Write(gomock.Any(), "mockPath").Return("/frontend/addons/mycluster.yml", nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments(gomock.Any()).AnyTimes()
			},
			wantedErr: nil,
		},
		"happy calls for wkld RDS with a RDWS": {
			inSvcName:           wantedSvcName,
			inStorageType:       rdsStorageType,
			inStorageName:       "mycluster",
			inServerlessVersion: auroraServerlessVersionV1,
			inEngine:            engineTypeMySQL,
			inParameterGroup:    "mygroup",
			inLifecycle:         lifecycleWorkloadLevel,

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Request-Driven Web Service"), nil)
				m.EXPECT().WorkloadAddonFilePath(gomock.Eq(wantedSvcName), gomock.Eq("mycluster.yml")).Return("mockTmplPath")
				m.EXPECT().Write(gomock.Any(), "mockTmplPath").Return("/frontend/addons/mycluster.yml", nil)
				m.EXPECT().WorkloadAddonFilePath(gomock.Eq(wantedSvcName), gomock.Eq("addons.parameters.yml")).Return("mockParamsPath")
				m.EXPECT().Write(gomock.Any(), "mockParamsPath").Return("/frontend/addons/addons.parameters.yml", nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments(gomock.Any()).AnyTimes()
			},
			wantedErr: nil,
		},
		"happy calls for env S3": {
			inAppName:     wantedAppName,
			inStorageType: s3StorageType,
			inSvcName:     wantedSvcName,
			inStorageName: "my-bucket",
			inLifecycle:   lifecycleEnvironmentLevel,

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Worker Service"), nil)
				m.EXPECT().EnvAddonFilePath(gomock.Eq("my-bucket.yml")).Return("mockEnvTemplatePath")
				m.EXPECT().WorkloadAddonFilePath(gomock.Eq(wantedSvcName), gomock.Eq("my-bucket-access-policy.yml")).Return("mockWkldTemplatePath")
				m.EXPECT().Write(gomock.Any(), "mockEnvTemplatePath").Return("mockEnvTemplatePath", nil)
				m.EXPECT().Write(gomock.Any(), "mockWkldTemplatePath").Return("mockWkldTemplatePath", nil)
			},
		},
		"happy calls for env DDB": {
			inAppName:     wantedAppName,
			inStorageType: dynamoDBStorageType,
			inSvcName:     wantedSvcName,
			inStorageName: "my-table",
			inNoLSI:       true,
			inNoSort:      true,
			inPartition:   wantedPartitionKey,
			inLifecycle:   lifecycleEnvironmentLevel,

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Worker Service"), nil)
				m.EXPECT().EnvAddonFilePath(gomock.Eq("my-table.yml")).Return("mockEnvTemplatePath")
				m.EXPECT().WorkloadAddonFilePath(gomock.Eq(wantedSvcName), gomock.Eq("my-table-access-policy.yml")).Return("mockWkldTemplatePath")
				m.EXPECT().Write(gomock.Any(), "mockEnvTemplatePath").Return("mockEnvTemplatePath", nil)
				m.EXPECT().Write(gomock.Any(), "mockWkldTemplatePath").Return("mockWkldTemplatePath", nil)
			},
		},
		"happy calls for env DDB with LSI": {
			inAppName:     wantedAppName,
			inStorageType: dynamoDBStorageType,
			inSvcName:     wantedSvcName,
			inStorageName: "my-table",
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,
			inLSISorts:    []string{"goodness:Number"},
			inLifecycle:   lifecycleEnvironmentLevel,

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Backend Service"), nil)
				m.EXPECT().EnvAddonFilePath(gomock.Eq("my-table.yml")).Return("mockEnvTemplatePath")
				m.EXPECT().WorkloadAddonFilePath(gomock.Eq(wantedSvcName), gomock.Eq("my-table-access-policy.yml")).Return("mockWkldTemplatePath")
				m.EXPECT().Write(gomock.Any(), "mockEnvTemplatePath").Return("mockEnvTemplatePath", nil)
				m.EXPECT().Write(gomock.Any(), "mockWkldTemplatePath").Return("mockWkldTemplatePath", nil)
			},
		},
		"happy calls for env RDS with LBWS": {
			inSvcName:           wantedSvcName,
			inStorageType:       rdsStorageType,
			inStorageName:       "mycluster",
			inServerlessVersion: auroraServerlessVersionV1,
			inEngine:            engineTypeMySQL,
			inParameterGroup:    "mygroup",
			inLifecycle:         lifecycleEnvironmentLevel,

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Load Balanced Web Service"), nil)
				m.EXPECT().EnvAddonFilePath(gomock.Eq("mycluster.yml")).Return("mockEnvTemplatePath")
				m.EXPECT().EnvAddonFilePath(gomock.Eq("addons.parameters.yml")).Return("mockEnvParametersPath")
				m.EXPECT().Write(gomock.Any(), "mockEnvTemplatePath").Return("mockEnvTemplatePath", nil)
				m.EXPECT().Write(gomock.Any(), "mockEnvParametersPath").Return("mockEnvParametersPath", nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments(gomock.Any()).AnyTimes()
			},
			wantedErr: nil,
		},
		"happy calls for env RDS with a RDWS": {
			inSvcName:           wantedSvcName,
			inStorageType:       rdsStorageType,
			inStorageName:       "mycluster",
			inServerlessVersion: auroraServerlessVersionV1,
			inEngine:            engineTypeMySQL,
			inParameterGroup:    "mygroup",
			inLifecycle:         lifecycleEnvironmentLevel,

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Request-Driven Web Service"), nil)
				m.EXPECT().EnvAddonFilePath(gomock.Eq("mycluster.yml")).Return("mockEnvTemplatePath")
				m.EXPECT().EnvAddonFilePath(gomock.Eq("addons.parameters.yml")).Return("mockEnvParametersPath")
				m.EXPECT().WorkloadAddonFilePath(gomock.Eq(wantedSvcName), gomock.Eq("mycluster-ingress.yml")).Return("mockWkldTmplPath")
				m.EXPECT().WorkloadAddonFilePath(gomock.Eq(wantedSvcName), gomock.Eq("addons.parameters.yml")).Return("mockWkldParamsPath")
				m.EXPECT().Write(gomock.Any(), "mockEnvTemplatePath").Return("mockEnvTemplatePath", nil)
				m.EXPECT().Write(gomock.Any(), "mockEnvParametersPath").Return("mockEnvParametersPath", nil)
				m.EXPECT().Write(gomock.Any(), "mockWkldTmplPath").Return("mockWkldTmplPath", nil)
				m.EXPECT().Write(gomock.Any(), "mockWkldParamsPath").Return("mockWkldParamsPath", nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments(gomock.Any()).AnyTimes()
			},
		},
		"do not attempt to write workload ingress for an env RDS if workload is not in the workspace": {
			inSvcName:           wantedSvcName,
			inStorageType:       rdsStorageType,
			inStorageName:       "mycluster",
			inServerlessVersion: auroraServerlessVersionV1,
			inEngine:            engineTypeMySQL,
			inParameterGroup:    "mygroup",
			inLifecycle:         lifecycleEnvironmentLevel,
			mockWkldAbsent:      true,
			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Request-Driven Web Service"), nil)
				m.EXPECT().EnvAddonFilePath(gomock.Eq("mycluster.yml")).Return("mockEnvPath")
				m.EXPECT().EnvAddonFilePath(gomock.Eq("addons.parameters.yml")).Return("mockEnvPath")
				m.EXPECT().Write(gomock.Any(), gomock.Not(gomock.Eq("mockWkldPath"))).Return("mockEnvTemplatePath", nil).Times(2)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments(gomock.Any()).AnyTimes()
			},
		},
		"error addon exists": {
			inAppName:     wantedAppName,
			inStorageType: s3StorageType,
			inSvcName:     wantedSvcName,
			inStorageName: "my-bucket",
			inLifecycle:   lifecycleWorkloadLevel,

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Load Balanced Web Service"), nil)
				m.EXPECT().WorkloadAddonFilePath(gomock.Eq(wantedSvcName), gomock.Eq("my-bucket.yml")).Return("mockPath")
				m.EXPECT().Write(gomock.Any(), "mockPath").Return("/frontend/addons/my-bucket.yml", nil).Return("", fileExistsError)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments(gomock.Any()).AnyTimes()
			},

			wantedErr: fmt.Errorf("addon file already exists: %w", fileExistsError),
		},
		"unexpected read workload manifest error handled": {
			inAppName:     wantedAppName,
			inStorageType: s3StorageType,
			inSvcName:     wantedSvcName,
			inStorageName: "my-bucket",
			inLifecycle:   lifecycleWorkloadLevel,

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, errors.New("some error"))
			},

			wantedErr: errors.New("read manifest for frontend: some error"),
		},
		"unexpected write addon error handled": {
			inAppName:     wantedAppName,
			inStorageType: s3StorageType,
			inSvcName:     wantedSvcName,
			inStorageName: "my-bucket",
			inLifecycle:   lifecycleWorkloadLevel,

			mockWs: func(m *mocks.MockwsAddonManager) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Load Balanced Web Service"), nil)
				m.EXPECT().WorkloadAddonFilePath(gomock.Eq(wantedSvcName), gomock.Eq("my-bucket.yml")).Return("mockPath")
				m.EXPECT().Write(gomock.Any(), "mockPath").Return("", errors.New("some error"))
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
			mockStore := mocks.NewMockstore(ctrl)
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

					auroraServerlessVersion: tc.inServerlessVersion,
					rdsEngine:               tc.inEngine,
					rdsParameterGroup:       tc.inParameterGroup,

					lifecycle: tc.inLifecycle,
				},
				appName:        tc.inAppName,
				ws:             mockAddon,
				store:          mockStore,
				workloadExists: !tc.mockWkldAbsent,
			}
			tc.mockWs(mockAddon)
			if tc.mockStore != nil {
				tc.mockStore(mockStore)
			}

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
