// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addon contains the service to manage addons.
package addon

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDynamoDBTemplate_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, ddb *DynamoDBTemplate)

		wantedBinary []byte
		wantedError  error
	}{
		"error parsing template": {
			mockDependencies: func(ctrl *gomock.Controller, ddb *DynamoDBTemplate) {
				m := mocks.NewMockParser(ctrl)
				ddb.parser = m
				m.EXPECT().Parse("mockPath", *ddb, gomock.Any()).Return(nil, errors.New("some error"))
			},

			wantedError: errors.New("some error"),
		},
		"returns rendered content": {
			mockDependencies: func(ctrl *gomock.Controller, ddb *DynamoDBTemplate) {
				m := mocks.NewMockParser(ctrl)
				ddb.parser = m
				m.EXPECT().Parse("mockPath", *ddb, gomock.Any()).Return(&template.Content{Buffer: bytes.NewBufferString("hello")}, nil)

			},

			wantedBinary: []byte("hello"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			addon := &DynamoDBTemplate{
				tmplPath: "mockPath",
			}
			tc.mockDependencies(ctrl, addon)

			// WHEN
			b, err := addon.MarshalBinary()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedBinary, b)
		})
	}
}

func TestS3Template_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, s3 *S3Template)

		wantedBinary []byte
		wantedError  error
	}{
		"error parsing template": {
			mockDependencies: func(ctrl *gomock.Controller, s3 *S3Template) {
				m := mocks.NewMockParser(ctrl)
				s3.parser = m
				m.EXPECT().Parse("mockPath", *s3, gomock.Any()).Return(nil, errors.New("some error"))
			},

			wantedError: errors.New("some error"),
		},
		"returns rendered content": {
			mockDependencies: func(ctrl *gomock.Controller, s3 *S3Template) {
				m := mocks.NewMockParser(ctrl)
				s3.parser = m
				m.EXPECT().Parse("mockPath", *s3, gomock.Any()).Return(&template.Content{Buffer: bytes.NewBufferString("hello")}, nil)

			},

			wantedBinary: []byte("hello"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			addon := &S3Template{
				tmplPath: "mockPath",
			}
			tc.mockDependencies(ctrl, addon)

			// WHEN
			b, err := addon.MarshalBinary()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedBinary, b)
		})
	}
}

func TestRDSTemplate_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		version string
		engine  string

		mockDependencies func(ctrl *gomock.Controller, r *RDSTemplate)

		wantedBinary []byte
		wantedError  error
	}{
		"error parsing template": {
			version: auroraServerlessVersionV1,
			engine:  RDSEngineTypePostgreSQL,
			mockDependencies: func(ctrl *gomock.Controller, r *RDSTemplate) {
				m := mocks.NewMockParser(ctrl)
				r.parser = m
				m.EXPECT().Parse(gomock.Any(), *r, gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("some error"),
		},
		"renders postgresql v1 content": {
			version: auroraServerlessVersionV1,
			engine:  RDSEngineTypePostgreSQL,
			mockDependencies: func(ctrl *gomock.Controller, r *RDSTemplate) {
				m := mocks.NewMockParser(ctrl)
				r.parser = m
				m.EXPECT().Parse(gomock.Eq("mockPath"), *r, gomock.Any()).
					Return(&template.Content{Buffer: bytes.NewBufferString("psql")}, nil)

			},
			wantedBinary: []byte("psql"),
		},
		"renders mysql v1 content": {
			version: auroraServerlessVersionV1,
			engine:  RDSEngineTypeMySQL,
			mockDependencies: func(ctrl *gomock.Controller, r *RDSTemplate) {
				m := mocks.NewMockParser(ctrl)
				r.parser = m
				m.EXPECT().Parse(gomock.Eq("mockPath"), *r, gomock.Any()).
					Return(&template.Content{Buffer: bytes.NewBufferString("mysql")}, nil)

			},
			wantedBinary: []byte("mysql"),
		},
		"renders postgresql v2 content": {
			version: auroraServerlessVersionV2,
			engine:  RDSEngineTypePostgreSQL,
			mockDependencies: func(ctrl *gomock.Controller, r *RDSTemplate) {
				m := mocks.NewMockParser(ctrl)
				r.parser = m
				m.EXPECT().Parse(gomock.Eq("mockPath"), *r, gomock.Any()).
					Return(&template.Content{Buffer: bytes.NewBufferString("psql")}, nil)

			},
			wantedBinary: []byte("psql"),
		},
		"renders mysql v2 content": {
			version: auroraServerlessVersionV2,
			engine:  RDSEngineTypeMySQL,
			mockDependencies: func(ctrl *gomock.Controller, r *RDSTemplate) {
				m := mocks.NewMockParser(ctrl)
				r.parser = m
				m.EXPECT().Parse(gomock.Eq("mockPath"), *r, gomock.Any()).
					Return(&template.Content{Buffer: bytes.NewBufferString("mysql")}, nil)

			},
			wantedBinary: []byte("mysql"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			addon := &RDSTemplate{
				RDSProps: RDSProps{
					Engine: tc.engine,
				},
				tmplPath: "mockPath",
			}
			tc.mockDependencies(ctrl, addon)

			// WHEN
			b, err := addon.MarshalBinary()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedBinary, b)
		})
	}
}

func TestRDSParams_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, r *RDSParams)

		wantedBinary []byte
		wantedError  error
	}{
		"error parsing template": {
			mockDependencies: func(ctrl *gomock.Controller, r *RDSParams) {
				m := mocks.NewMockParser(ctrl)
				r.parser = m
				m.EXPECT().Parse(gomock.Any(), *r, gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("some error"),
		},
		"renders param file with expected path": {
			mockDependencies: func(ctrl *gomock.Controller, r *RDSParams) {
				m := mocks.NewMockParser(ctrl)
				r.parser = m
				r.tmplPath = "mockPath"
				m.EXPECT().Parse(gomock.Eq("mockPath"), *r, gomock.Any()).
					Return(&template.Content{Buffer: bytes.NewBufferString("bloop")}, nil)

			},
			wantedBinary: []byte("bloop"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			params := &RDSParams{}
			tc.mockDependencies(ctrl, params)

			// WHEN
			b, err := params.MarshalBinary()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedBinary, b)
		})
	}
}

func TestDDBAttributeFromKey(t *testing.T) {
	testCases := map[string]struct {
		input     string
		wantName  string
		wantType  string
		wantError error
	}{
		"good case": {
			input:     "userID:S",
			wantName:  "userID",
			wantType:  "S",
			wantError: nil,
		},
		"bad case": {
			input:     "userID",
			wantError: fmt.Errorf("parse attribute from key: %s", "userID"),
		},
		"non-ideal input": {
			input:     "userId_cool-table.d:binary",
			wantName:  "userId_cool-table.d",
			wantType:  "B",
			wantError: nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := DDBAttributeFromKey(tc.input)
			if tc.wantError != nil {
				require.EqualError(t, err, tc.wantError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantName, *got.Name)
				require.Equal(t, tc.wantType, *got.DataType)
			}
		})
	}
}

func TestNewLSI(t *testing.T) {
	testPartitionKey := "Email"
	testSortKey := "Goodness"
	testCases := map[string]struct {
		inPartitionKey string
		inLSIs         []string
		wantedLSI      []DDBLocalSecondaryIndex
		wantError      error
	}{
		"happy case": {
			inPartitionKey: "Email",
			inLSIs:         []string{"Goodness:N"},
			wantedLSI: []DDBLocalSecondaryIndex{
				{
					Name:         &testSortKey,
					PartitionKey: &testPartitionKey,
					SortKey:      &testSortKey,
				},
			},
		},
		"no error getting attribute": {
			inPartitionKey: "Email",
			inLSIs:         []string{"goodness"},
			wantError:      fmt.Errorf("parse attribute from key: goodness"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := newLSI(tc.inPartitionKey, tc.inLSIs)
			if tc.wantError != nil {
				require.EqualError(t, err, tc.wantError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedLSI, got)
			}
		})
	}
}

func TestBuildPartitionKey(t *testing.T) {
	wantDataType := "S"
	wantName := "userID"
	testCases := map[string]struct {
		input            string
		wantPartitionKey string
		wantAttributes   []DDBAttribute
		wantError        error
	}{
		"good case": {
			input:            "userID:S",
			wantPartitionKey: wantName,
			wantAttributes: []DDBAttribute{
				{
					DataType: &wantDataType,
					Name:     &wantName,
				},
			},
			wantError: nil,
		},
		"error getting attribute": {
			input:     "userID",
			wantError: fmt.Errorf("parse attribute from key: userID"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			props := DynamoDBProps{}
			err := props.BuildPartitionKey(tc.input)
			if tc.wantError != nil {
				require.EqualError(t, err, tc.wantError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantPartitionKey, *props.PartitionKey)
				require.Equal(t, tc.wantAttributes, props.Attributes)
			}
		})
	}
}

func TestBuildSortKey(t *testing.T) {
	wantDataType := "S"
	wantName := "userID"
	testCases := map[string]struct {
		inSortKey      string
		inNoSort       bool
		wantSortKey    string
		wantHasSortKey bool
		wantAttributes []DDBAttribute
		wantError      error
	}{
		"with sort key": {
			inSortKey:   "userID:S",
			inNoSort:    false,
			wantSortKey: wantName,
			wantAttributes: []DDBAttribute{
				{
					DataType: &wantDataType,
					Name:     &wantName,
				},
			},
			wantHasSortKey: true,
			wantError:      nil,
		},
		"with noSort specified": {
			inNoSort:       true,
			inSortKey:      "userID:S",
			wantSortKey:    "",
			wantHasSortKey: false,
		},
		"no sort key without noSort specified": {
			inNoSort:       false,
			inSortKey:      "",
			wantSortKey:    "",
			wantHasSortKey: false,
		},
		"error getting attribute": {
			inSortKey: "userID",
			wantError: fmt.Errorf("parse attribute from key: userID"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			props := DynamoDBProps{}
			got, err := props.BuildSortKey(tc.inNoSort, tc.inSortKey)
			if tc.wantError != nil {
				require.EqualError(t, err, tc.wantError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantAttributes, props.Attributes)
				require.Equal(t, tc.wantHasSortKey, got)
				if tc.wantSortKey == "" {
					require.Nil(t, props.SortKey)
				} else {
					require.Equal(t, tc.wantSortKey, *props.SortKey)
				}
			}
		})
	}
}

func TestBuildLocalSecondaryIndex(t *testing.T) {
	wantSortKey := "userID"
	wantPartitionKey := "email"
	wantLSIName := "points"
	wantLSIType := "N"

	testCases := map[string]struct {
		inPartitionKey   *string
		inSortKey        *string
		inNoLSI          bool
		inLSISorts       []string
		wantedLSI        []DDBLocalSecondaryIndex
		wantedAttributes []DDBAttribute
		wantedHasLSI     bool
		wantedError      error
	}{
		"error if no partition key": {
			inPartitionKey: nil,
			wantedError:    fmt.Errorf("partition key not specified"),
		},
		"no LSI if sort key not specified": {
			inPartitionKey: &wantPartitionKey,
			inSortKey:      nil,
			inLSISorts:     []string{"points:N"},
			wantedHasLSI:   false,
		},
		"no LSI if noLSI specified": {
			inPartitionKey: &wantPartitionKey,
			inSortKey:      &wantSortKey,
			inLSISorts:     []string{"points:N"},
			inNoLSI:        true,
			wantedHasLSI:   false,
		},
		"no LSI if length of LSIs is 0": {
			inPartitionKey: &wantPartitionKey,
			inSortKey:      &wantSortKey,
			inNoLSI:        false,
			wantedHasLSI:   false,
		},
		"LSI specified correctly": {
			inPartitionKey: &wantPartitionKey,
			inSortKey:      &wantSortKey,
			inNoLSI:        false,
			inLSISorts:     []string{wantLSIName + ":" + wantLSIType},
			wantedHasLSI:   true,
			wantedLSI: []DDBLocalSecondaryIndex{
				{
					Name:         &wantLSIName,
					PartitionKey: &wantPartitionKey,
					SortKey:      &wantLSIName,
				},
			},
			wantedAttributes: []DDBAttribute{
				{
					DataType: &wantLSIType,
					Name:     &wantLSIName,
				},
			},
			wantedError: nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			props := DynamoDBProps{}
			props.PartitionKey = tc.inPartitionKey
			props.SortKey = tc.inSortKey
			got, err := props.BuildLocalSecondaryIndex(tc.inNoLSI, tc.inLSISorts)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedHasLSI, props.HasLSI)
				require.Equal(t, tc.wantedHasLSI, got)
				for idx, att := range props.Attributes {
					require.Equal(t, *tc.wantedAttributes[idx].DataType, *att.DataType)
					require.Equal(t, *tc.wantedAttributes[idx].Name, *att.Name)
				}
				for idx, lsi := range props.LSIs {
					require.Equal(t, *tc.wantedLSI[idx].Name, *lsi.Name)
					require.Equal(t, *tc.wantedLSI[idx].PartitionKey, *lsi.PartitionKey)
					require.Equal(t, *tc.wantedLSI[idx].SortKey, *lsi.SortKey)
				}
			}
		})
	}
}

func TestConstructors(t *testing.T) {
	t.Run("marshaler for workload-level S3", func(t *testing.T) {
		out := WorkloadS3Template(&S3Props{})
		require.Equal(t, s3TemplatePath, out.tmplPath)
	})

	t.Run("marshaler for env-level S3", func(t *testing.T) {
		out := EnvS3Template(&S3Props{})
		require.Equal(t, envS3TemplatePath, out.tmplPath)
	})

	t.Run("marshaler for the access policy of an env-level S3", func(t *testing.T) {
		out := EnvS3AccessPolicyTemplate(&AccessPolicyProps{})
		require.Equal(t, envS3AccessPolicyTemplatePath, out.tmplPath)
	})

	t.Run("marshaler for workload-level ddb", func(t *testing.T) {
		out := WorkloadDDBTemplate(&DynamoDBProps{})
		require.Equal(t, dynamoDbTemplatePath, out.tmplPath)
	})

	t.Run("marshaler for env-level ddb", func(t *testing.T) {
		out := EnvDDBTemplate(&DynamoDBProps{})
		require.Equal(t, envDynamoDBTemplatePath, out.tmplPath)
	})

	t.Run("marshaler for the access policy of an env-level ddb", func(t *testing.T) {
		out := EnvDDBAccessPolicyTemplate(&AccessPolicyProps{})
		require.Equal(t, envDynamoDBAccessPolicyTemplatePath, out.tmplPath)
	})

	t.Run("marshaler for non-RDWS workload-level aurora serverless v1", func(t *testing.T) {
		out := WorkloadServerlessV1Template(RDSProps{})
		require.Equal(t, rdsTemplatePath, out.tmplPath)
	})

	t.Run("marshaler for non-RDWS workload-level aurora serverless v2", func(t *testing.T) {
		out := WorkloadServerlessV2Template(RDSProps{})
		require.Equal(t, rdsV2TemplatePath, out.tmplPath)
	})

	t.Run("parameter marshaler for RDWS workload-level aurora serverless v1/v2", func(t *testing.T) {
		out := RDWSParamsForRDS()
		require.Equal(t, rdsRDWSParamsPath, out.tmplPath)
	})

	t.Run("marshaler for RDWS workload-level aurora serverless v1", func(t *testing.T) {
		out := RDWSServerlessV1Template(RDSProps{})
		require.Equal(t, rdsRDWSTemplatePath, out.tmplPath)
	})

	t.Run("marshaler for RDWS workload-level aurora serverless v2", func(t *testing.T) {
		out := RDWSServerlessV2Template(RDSProps{})
		require.Equal(t, rdsV2RDWSTemplatePath, out.tmplPath)
	})

	t.Run("parameter marshaler for env-level aurora accessible by a workload", func(t *testing.T) {
		out := EnvParamsForRDS()
		require.Equal(t, envRDSParamsPath, out.tmplPath)
	})

	t.Run("marshaler for env-level aurora accessible by a non-RDWS", func(t *testing.T) {
		out := EnvServerlessTemplate(RDSProps{})
		require.Equal(t, envRDSTemplatePath, out.tmplPath)
	})

	t.Run("marshaler for env-level aurora accessible by an RDWS", func(t *testing.T) {
		out := EnvServerlessForRDWSTemplate(RDSProps{})
		require.Equal(t, envRDSForRDWSTemplatePath, out.tmplPath)
	})

	t.Run("parameter marshaler for the ingress attached to an RDWS for an env-level aurora", func(t *testing.T) {
		out := RDWSParamsForEnvRDS()
		require.Equal(t, envRDSIngressForRDWSParamsPath, out.tmplPath)
	})

	t.Run("marshaler for the ingress attached to an RDWS for an env-level aurora", func(t *testing.T) {
		out := EnvServerlessRDWSIngressTemplate(RDSIngressProps{})
		require.Equal(t, envRDSIngressForRDWSTemplatePath, out.tmplPath)
	})
}
