// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
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

type mockRoute53 struct {
	route53iface.Route53API
	t                         *testing.T
	mockListHostedZonesByName func(t *testing.T, in *route53.ListHostedZonesByNameInput) (*route53.ListHostedZonesByNameOutput, error)
}

func (m *mockRoute53) ListHostedZonesByName(in *route53.ListHostedZonesByNameInput) (*route53.ListHostedZonesByNameOutput, error) {
	return m.mockListHostedZonesByName(m.t, in)
}
