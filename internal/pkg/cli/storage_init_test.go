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

type mockStorageInitValidate struct {
	ws    *mocks.MockwsReadWriter
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
		wantedErr error
	}{
		"no app in workspace": {
			mock:      func(m *mockStorageInitValidate) {},
			wantedErr: errNoAppInWorkspace,
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
				ws:    mocks.NewMockwsReadWriter(ctrl),
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
	ws     *mocks.MockwsReadWriter
}

func TestStorageInitOpts_Ask(t *testing.T) {
	const (
		wantedAppName    = "bowie"
		wantedSvcName    = "frontend"
		wantedBucketName = "cool-bucket"
	)
	testCases := map[string]struct {
		inStorageType string
		inSvcName     string
		inStorageName string
		inLifecycle   string

		mock func(m *mockStorageInitAsk)

		wantedErr  error
		wantedVars *initStorageVars
	}{
		"invalid storage type": {
			inStorageType: "box",
			inSvcName:     "frontend",
			mock:          func(m *mockStorageInitAsk) {},
			wantedErr:     errors.New(`invalid storage type box: must be one of "DynamoDB", "S3", "Aurora"`),
		},
		"asks for storage type": {
			inSvcName:     wantedSvcName,
			inStorageName: wantedBucketName,
			inLifecycle:   lifecycleWorkloadLevel,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil).AnyTimes()
				m.prompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(s3StorageType, nil)
			},
			wantedVars: &initStorageVars{
				storageType:  s3StorageType,
				storageName:  wantedBucketName,
				workloadName: wantedSvcName,
				lifecycle:    lifecycleWorkloadLevel,
			},
		},
		"error if storage type not gotten": {
			inSvcName:     wantedSvcName,
			inStorageName: wantedBucketName,

			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("select storage type: some error"),
		},
		"error checking if svc is in workspace": {
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Eq("frontend")).Return(false, errors.New("wanted err"))
			},
			inStorageType: s3StorageType,
			inSvcName:     "frontend",
			inStorageName: "my-bucket",
			wantedErr:     errors.New("check if frontend exists in the workspace: wanted err"),
		},
		"invalid svc not in workspace": {
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Eq("frontend")).Return(false, nil)
			},
			inStorageType: s3StorageType,
			inSvcName:     "frontend",
			inStorageName: "my-bucket",
			wantedErr:     errors.New("workload frontend not found in the workspace"),
		},
		"asks for storage workload": {
			inStorageName: wantedBucketName,
			inStorageType: s3StorageType,
			inLifecycle:   lifecycleWorkloadLevel,
			mock: func(m *mockStorageInitAsk) {
				m.sel.EXPECT().Workload(gomock.Eq(storageInitSvcPrompt), gomock.Any()).Return(wantedSvcName, nil)
			},
			wantedVars: &initStorageVars{
				storageType:  s3StorageType,
				storageName:  wantedBucketName,
				workloadName: wantedSvcName,
				lifecycle:    lifecycleWorkloadLevel,
			},
		},
		"error if svc not returned": {
			inStorageName: wantedBucketName,
			inStorageType: s3StorageType,
			mock: func(m *mockStorageInitAsk) {
				m.sel.EXPECT().Workload(gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("retrieve local workload names: some error"),
		},
		"successfully validates valid s3 bucket name": {
			inSvcName:     "frontend",
			inStorageType: s3StorageType,
			inStorageName: "my-bucket.4",
			inLifecycle:   lifecycleEnvironmentLevel,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil).AnyTimes()
			},
		},
		"invalid s3 bucket name": {
			inSvcName:     "frontend",
			inStorageType: s3StorageType,
			inStorageName: "mybadbucket???",
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil).AnyTimes()
			},
			wantedErr: fmt.Errorf("validate storage name: %w", errValueBadFormatWithPeriod),
		},
		"asks for storage name": {
			inSvcName:     wantedSvcName,
			inStorageType: s3StorageType,
			inLifecycle:   lifecycleWorkloadLevel,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
				m.prompt.EXPECT().Get(gomock.Eq(fmt.Sprintf(fmtStorageInitNamePrompt, color.HighlightUserInput(s3BucketFriendlyText))),
					gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedBucketName, nil)
			},
			wantedVars: &initStorageVars{
				storageType:  s3StorageType,
				storageName:  wantedBucketName,
				workloadName: wantedSvcName,
				lifecycle:    lifecycleWorkloadLevel,
			},
		},
		"error if storage name not returned": {
			inSvcName:     wantedSvcName,
			inStorageType: s3StorageType,

			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
				m.prompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("input storage name: some error"),
		},
		"invalid lifecycle": {
			inSvcName:     wantedSvcName,
			inStorageType: s3StorageType,
			inStorageName: wantedBucketName,
			inLifecycle:   "immortal",
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
			},
			wantedErr: fmt.Errorf(`invalid lifecycle; must be one of "workload" or "environment"`),
		},
		"infer lifecycle to be workload level if the addon is found as a workload addon": {
			inSvcName:     wantedSvcName,
			inStorageType: s3StorageType,
			inStorageName: wantedBucketName,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
				m.ws.EXPECT().WorkloadAddonFileAbsPath(wantedSvcName, fmt.Sprintf("%s.yml", wantedBucketName)).Return("mockWorkloadAddonPath")
				m.ws.EXPECT().ReadFile("mockWorkloadAddonPath").Return([]byte(""), nil)
			},
			wantedVars: &initStorageVars{
				storageType:  s3StorageType,
				storageName:  wantedBucketName,
				workloadName: wantedSvcName,
				lifecycle:    lifecycleWorkloadLevel,
			},
		},
		"error reading workload addon": {
			inSvcName:     wantedSvcName,
			inStorageType: s3StorageType,
			inStorageName: wantedBucketName,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
				m.ws.EXPECT().WorkloadAddonFileAbsPath(wantedSvcName, fmt.Sprintf("%s.yml", wantedBucketName)).Return("mockWorkloadAddonPath")
				m.ws.EXPECT().ReadFile("mockWorkloadAddonPath").Return([]byte(""), errors.New("some error"))
			},
			wantedErr: fmt.Errorf("check if %s addon exists for %s in workspace: some error", wantedBucketName, wantedSvcName),
		},
		"infer lifecycle to be env level if the addon is found as an env addon": {
			inSvcName:     wantedSvcName,
			inStorageType: s3StorageType,
			inStorageName: wantedBucketName,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
				m.ws.EXPECT().WorkloadAddonFileAbsPath(wantedSvcName, fmt.Sprintf("%s.yml", wantedBucketName)).Return("mockWorkloadAddonPath")
				m.ws.EXPECT().ReadFile("mockWorkloadAddonPath").Return([]byte(""), &workspace.ErrFileNotExists{})
				m.ws.EXPECT().EnvAddonFileAbsPath(fmt.Sprintf("%s.yml", wantedBucketName)).Return("mockEnvAddonPath")
				m.ws.EXPECT().ReadFile("mockEnvAddonPath").Return([]byte(""), nil)
			},
			wantedVars: &initStorageVars{
				storageType:  s3StorageType,
				storageName:  wantedBucketName,
				workloadName: wantedSvcName,
				lifecycle:    lifecycleEnvironmentLevel,
			},
		},
		"error reading environment addon": {
			inSvcName:     wantedSvcName,
			inStorageType: s3StorageType,
			inStorageName: wantedBucketName,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
				m.ws.EXPECT().WorkloadAddonFileAbsPath(wantedSvcName, fmt.Sprintf("%s.yml", wantedBucketName)).Return("mockWorkloadAddonPath")
				m.ws.EXPECT().ReadFile("mockWorkloadAddonPath").Return([]byte(""), &workspace.ErrFileNotExists{})
				m.ws.EXPECT().EnvAddonFileAbsPath(fmt.Sprintf("%s.yml", wantedBucketName)).Return("mockEnvAddonPath")
				m.ws.EXPECT().ReadFile("mockEnvAddonPath").Return([]byte(""), errors.New("some error"))
			},
			wantedErr: fmt.Errorf("check if %s exists as an environment addon in workspace: some error", wantedBucketName),
		},
		"asks for lifecycle": {
			inSvcName:     wantedSvcName,
			inStorageType: s3StorageType,
			inStorageName: wantedBucketName,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
				m.ws.EXPECT().WorkloadAddonFileAbsPath(wantedSvcName, fmt.Sprintf("%s.yml", wantedBucketName)).Return("mockWorkloadAddonPath")
				m.ws.EXPECT().ReadFile("mockWorkloadAddonPath").Return([]byte(""), &workspace.ErrFileNotExists{})
				m.ws.EXPECT().EnvAddonFileAbsPath(fmt.Sprintf("%s.yml", wantedBucketName)).Return("mockEnvAddonPath")
				m.ws.EXPECT().ReadFile("mockEnvAddonPath").Return([]byte(""), &workspace.ErrFileNotExists{})
				m.prompt.EXPECT().SelectOne(storageInitLifecyclePrompt, gomock.Any(), gomock.Any()).Return(lifecycleWorkloadFriendlyTextOption, nil)
			},
			wantedVars: &initStorageVars{
				storageType:  s3StorageType,
				storageName:  wantedBucketName,
				workloadName: wantedSvcName,
				lifecycle:    lifecycleWorkloadLevel,
			},
		},
		"error if lifecycle not gotten": {
			inSvcName:     wantedSvcName,
			inStorageType: s3StorageType,
			inStorageName: wantedBucketName,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
				m.ws.EXPECT().WorkloadAddonFileAbsPath(wantedSvcName, fmt.Sprintf("%s.yml", wantedBucketName)).Return("mockWorkloadAddonPath")
				m.ws.EXPECT().ReadFile("mockWorkloadAddonPath").Return([]byte(""), &workspace.ErrFileNotExists{})
				m.ws.EXPECT().EnvAddonFileAbsPath(fmt.Sprintf("%s.yml", wantedBucketName)).Return("mockEnvAddonPath")
				m.ws.EXPECT().ReadFile("mockEnvAddonPath").Return([]byte(""), &workspace.ErrFileNotExists{})
				m.prompt.EXPECT().SelectOne(storageInitLifecyclePrompt, gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: errors.New("ask for lifecycle: some error"),
		},
		"no error or asks when fully specified": {
			inSvcName:     wantedSvcName,
			inStorageType: s3StorageType,
			inStorageName: wantedBucketName,
			inLifecycle:   lifecycleEnvironmentLevel,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil).AnyTimes()
			},
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
				ws:     mocks.NewMockwsReadWriter(ctrl),
			}
			opts := initStorageOpts{
				initStorageVars: initStorageVars{
					storageType:  tc.inStorageType,
					storageName:  tc.inStorageName,
					workloadName: tc.inSvcName,
					lifecycle:    tc.inLifecycle,
				},
				appName: wantedAppName,
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
		wantedTableName    = "my-cool_table.3"
		wantedPartitionKey = "DogName:String"
		wantedSortKey      = "PhotoId:Number"
	)
	testCases := map[string]struct {
		inStorageName string
		inPartition   string
		inSort        string
		inLSISorts    []string
		inNoLSI       bool
		inNoSort      bool

		mock func(m *mockStorageInitAsk)

		wantedErr  error
		wantedVars *initStorageVars
	}{
		"invalid ddb name": {
			inStorageName: "badTable!!!",
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
			},
			wantedErr: fmt.Errorf("validate storage name: %w", errValueBadFormatWithPeriodUnderscore),
		},
		"invalid partition key": {
			inStorageName: wantedTableName,
			inPartition:   "bipartite",
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
			},
			wantedErr: errors.New("validate partition key: value must be of the form <name>:<T> where T is one of S, N, or B"),
		},
		"asks for partition key if not specified": {
			inStorageName: wantedTableName,
			inSort:        wantedSortKey,
			inNoLSI:       true,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
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
			inStorageName: wantedTableName,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
				m.prompt.EXPECT().Get(gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get DDB partition key: some error"),
		},
		"error if fail to return partition key type": {
			inStorageName: wantedTableName,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
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
		"invalid sort key": {
			inStorageName: wantedTableName,
			inPartition:   wantedSortKey,
			inSort:        "allsortofstuff",
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
			},
			wantedErr: errors.New("validate sort key: value must be of the form <name>:<T> where T is one of S, N, or B"),
		},
		"ask for sort key if not specified": {
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inNoLSI:       true,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
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
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
				m.prompt.EXPECT().Confirm(
					gomock.Eq(storageInitDDBSortKeyConfirm),
					gomock.Any(),
					gomock.Any(),
				).Return(false, errors.New("some error"))
			},
			wantedErr: fmt.Errorf("confirm DDB sort key: some error"),
		},
		"error if fail to return sort key": {
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
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
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
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
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inNoSort:      true,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
				m.prompt.EXPECT().Confirm(
					gomock.Eq(storageInitDDBSortKeyConfirm),
					gomock.Any(),
					gomock.Any(),
				).Times(0)
			},
		},
		"ok if --no-lsi and --sort-key are both specified": {
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,
			inNoLSI:       true,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
			},
		},
		"don't ask about LSI if no-sort is specified": {
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inNoSort:      true,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
				m.prompt.EXPECT().Confirm(
					gomock.Eq(storageInitDDBLSIPrompt),
					gomock.Eq(storageInitDDBLSIHelp),
					gomock.Any(),
				).Times(0)
			},
		},
		"ask for LSI if not specified": {
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
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
				lifecycle:    lifecycleWorkloadLevel,

				partitionKey: wantedPartitionKey,
				sortKey:      wantedSortKey,
				noLSI:        false,
				lsiSorts:     []string{"Email:String"},
			},
		},
		"noLSI is set correctly if no lsis specified": {
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
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
				lifecycle:    lifecycleWorkloadLevel,

				partitionKey: wantedPartitionKey,
				sortKey:      wantedSortKey,
				noLSI:        true,
			},
		},
		"noLSI is set correctly if no sort key": {
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
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
				lifecycle:    lifecycleWorkloadLevel,

				partitionKey: wantedPartitionKey,
				noLSI:        true,
				noSort:       true,
			},
		},
		"error if lsi name misspecified": {
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
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
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
				m.prompt.EXPECT().Confirm(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(false, errors.New("some error"))
			},
			wantedErr: fmt.Errorf("confirm add alternate sort key: some error"),
		},
		"error if lsi type misspecified": {
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
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
		"do not ask for ddb config when fully specified": {
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inSort:        wantedSortKey,
			inLSISorts:    []string{"userID:Number", "data:Binary"},
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
			},
		},
		"successfully validate flags with non-config": {
			inStorageName: wantedTableName,
			inPartition:   wantedPartitionKey,
			inNoSort:      true,
			inNoLSI:       true,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mockStorageInitAsk{
				ws:     mocks.NewMockwsReadWriter(ctrl),
				prompt: mocks.NewMockprompter(ctrl),
			}
			tc.mock(&m)
			opts := initStorageOpts{
				initStorageVars: initStorageVars{
					storageType:  dynamoDBStorageType,
					storageName:  tc.inStorageName,
					workloadName: wantedSvcName,
					partitionKey: tc.inPartition,
					sortKey:      tc.inSort,
					lsiSorts:     tc.inLSISorts,
					noLSI:        tc.inNoLSI,
					noSort:       tc.inNoSort,
					lifecycle:    lifecycleWorkloadLevel,
				},
				appName: "ddos",
				prompt:  m.prompt,
				ws:      m.ws,
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
		"invalid cluster name": {
			inStorageName: "wow!such name..:doge",
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(workspace.WorkloadManifest("type: Load Balanced Web Service"), nil)
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
			},
			wantedErr: errors.New("validate storage name: value must start with a letter and followed by alphanumeric letters only"),
		},
		"asks for cluster name for RDS storage": {
			inDBEngine:      wantedDBEngine,
			inInitialDBName: wantedInitialDBName,

			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
				m.prompt.EXPECT().Get(
					gomock.Eq("What would you like to name this Database Cluster?"),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedClusterName, nil)
				m.ws.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(workspace.WorkloadManifest("type: Load Balanced Web Service"), nil)
			},
			wantedVars: &initStorageVars{
				storageType:  rdsStorageType,
				storageName:  wantedClusterName,
				workloadName: wantedSvcName,
				lifecycle:    lifecycleEnvironmentLevel,

				auroraServerlessVersion: wantedServerlessVersion,
				rdsEngine:               wantedDBEngine,
				rdsInitialDBName:        wantedInitialDBName,
			},
		},
		"error if cluster name not gotten": {
			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Get(
					gomock.Eq("What would you like to name this Database Cluster?"),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
				m.ws.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(workspace.WorkloadManifest("type: Load Balanced Web Service"), nil)
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
			},
			wantedErr: errors.New("input storage name: some error"),
		},
		"invalid database engine type": {
			inStorageName: wantedClusterName,
			inDBEngine:    "mysql",
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(workspace.WorkloadManifest("type: Load Balanced Web Service"), nil)
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
			},
			wantedErr: errors.New("invalid engine type mysql: must be one of \"MySQL\", \"PostgreSQL\""),
		},
		"asks for engine if not specified": {
			inStorageName:   wantedClusterName,
			inInitialDBName: wantedInitialDBName,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
				m.ws.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(workspace.WorkloadManifest("type: Load Balanced Web Service"), nil)
				m.prompt.EXPECT().SelectOne(gomock.Eq(storageInitRDSDBEnginePrompt), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedDBEngine, nil)

			},
			wantedVars: &initStorageVars{
				storageType:  rdsStorageType,
				storageName:  wantedClusterName,
				workloadName: wantedSvcName,
				lifecycle:    lifecycleEnvironmentLevel,

				auroraServerlessVersion: wantedServerlessVersion,
				rdsInitialDBName:        wantedInitialDBName,
				rdsEngine:               wantedDBEngine,
			},
		},
		"error if engine not gotten": {
			inStorageName:   wantedClusterName,
			inInitialDBName: wantedInitialDBName,
			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().SelectOne(storageInitRDSDBEnginePrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
				m.ws.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(workspace.WorkloadManifest("type: Load Balanced Web Service"), nil)
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)

			},
			wantedErr: errors.New("select database engine: some error"),
		},
		"invalid initial database name": {
			inStorageName:   wantedClusterName,
			inDBEngine:      wantedDBEngine,
			inInitialDBName: "wow!suchweird??name!",

			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(workspace.WorkloadManifest("type: Load Balanced Web Service"), nil)
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
			},
			wantedErr: errors.New("invalid database name wow!suchweird??name!: must contain only alphanumeric characters and underscore; should start with a letter"),
		},
		"asks for initial database name": {
			inStorageName: wantedClusterName,
			inDBEngine:    wantedDBEngine,

			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Get(gomock.Eq(storageInitRDSInitialDBNamePrompt), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedInitialDBName, nil)
				m.ws.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(workspace.WorkloadManifest("type: Load Balanced Web Service"), nil)
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
			},
			wantedVars: &initStorageVars{
				storageType:  rdsStorageType,
				storageName:  wantedClusterName,
				workloadName: wantedSvcName,
				lifecycle:    lifecycleEnvironmentLevel,

				auroraServerlessVersion: wantedServerlessVersion,
				rdsEngine:               wantedDBEngine,
				rdsInitialDBName:        wantedInitialDBName,
			},
		},
		"error if initial database name not gotten": {
			inStorageName: wantedClusterName,
			inDBEngine:    wantedDBEngine,

			mock: func(m *mockStorageInitAsk) {
				m.prompt.EXPECT().Get(storageInitRDSInitialDBNamePrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
				m.ws.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(workspace.WorkloadManifest("type: Load Balanced Web Service"), nil)
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)

			},
			wantedErr: fmt.Errorf("input initial database name: some error"),
		},
		"successfully validate rds with full config": {
			inStorageName:   wantedClusterName,
			inDBEngine:      wantedDBEngine,
			inInitialDBName: wantedInitialDBName,
			mock: func(m *mockStorageInitAsk) {
				m.ws.EXPECT().WorkloadExists(gomock.Any()).Return(true, nil)
				m.ws.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(workspace.WorkloadManifest("type: Load Balanced Web Service"), nil)
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mockStorageInitAsk{
				prompt: mocks.NewMockprompter(ctrl),
				ws:     mocks.NewMockwsReadWriter(ctrl),
			}
			opts := initStorageOpts{
				initStorageVars: initStorageVars{
					storageType:  rdsStorageType,
					workloadName: wantedSvcName,
					storageName:  tc.inStorageName,
					lifecycle:    lifecycleEnvironmentLevel,

					auroraServerlessVersion: wantedServerlessVersion,
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

		mockWS         func(m *mocks.MockwsReadWriter)
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

			mockWS: func(m *mocks.MockwsReadWriter) {
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

			mockWS: func(m *mocks.MockwsReadWriter) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Worker Service"), nil)
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

			mockWS: func(m *mocks.MockwsReadWriter) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Worker Service"), nil)
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

			mockWS: func(m *mocks.MockwsReadWriter) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Worker Service"), nil)
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

			mockWS: func(m *mocks.MockwsReadWriter) {
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

			mockWS: func(m *mocks.MockwsReadWriter) {
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

			mockWS: func(m *mocks.MockwsReadWriter) {
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

			mockWS: func(m *mocks.MockwsReadWriter) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Worker Service"), nil)
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

			mockWS: func(m *mocks.MockwsReadWriter) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Load-Balanced Web Service"), nil)
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

			mockWS: func(m *mocks.MockwsReadWriter) {
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
			mockWS: func(m *mocks.MockwsReadWriter) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Worker Service"), nil)
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

			mockWS: func(m *mocks.MockwsReadWriter) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Worker Service"), nil)
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
			mockWS: func(m *mocks.MockwsReadWriter) {
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

			mockWS: func(m *mocks.MockwsReadWriter) {
				m.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte("type: Worker Service"), nil)
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

			mockStore := mocks.NewMockstore(ctrl)
			mockWS := mocks.NewMockwsReadWriter(ctrl)
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
				ws:             mockWS,
				store:          mockStore,
				workloadExists: !tc.mockWkldAbsent,
			}
			tc.mockWS(mockWS)
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
