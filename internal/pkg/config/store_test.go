// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
)

type mockSSM struct {
	ssmiface.SSMAPI
	t                       *testing.T
	mockPutParameter        func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error)
	mockGetParametersByPath func(t *testing.T, param *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error)
	mockGetParameter        func(t *testing.T, param *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
	mockDeleteParameter     func(t *testing.T, param *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error)
}

func (m *mockSSM) PutParameter(in *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
	return m.mockPutParameter(m.t, in)
}

func (m *mockSSM) GetParametersByPath(in *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error) {
	return m.mockGetParametersByPath(m.t, in)
}

func (m *mockSSM) GetParameter(in *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
	return m.mockGetParameter(m.t, in)
}

func (m *mockSSM) DeleteParameter(in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error) {
	return m.mockDeleteParameter(m.t, in)
}

type mockIdentityService struct {
	mockIdentityServiceGet func() (identity.Caller, error)
}

func (m mockIdentityService) Get() (identity.Caller, error) {
	return m.mockIdentityServiceGet()
}
