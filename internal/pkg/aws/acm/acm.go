// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package acm provides a client to make API requests to AWS Certificate Manager.
package acm

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acm"

	"github.com/aws/aws-sdk-go/aws/session"
)

type api interface {
	DescribeCertificate(input *acm.DescribeCertificateInput) (*acm.DescribeCertificateOutput, error)
}

// ACM wraps an AWS Certificate Manager client.
type ACM struct {
	client api
}

// New returns an ACM struct configured against the input session.
func New(s *session.Session) *ACM {
	return &ACM{
		client: acm.New(s),
	}
}

// ValidateCertAliases validates if aliases are all valid against the provided ACM certificates.
func (a *ACM) ValidateCertAliases(aliases []string, certs []string) error {
	validAliases := make(map[string]bool)
	// TODO: Parallelize `findValidAliasesAgainstCert` if needed.
	for _, cert := range certs {
		validCertAliases, err := a.findValidAliasesAgainstCert(aliases, cert)
		if err != nil {
			return err
		}
		for _, alias := range validCertAliases {
			validAliases[alias] = true
		}
	}
	for _, alias := range aliases {
		if !validAliases[alias] {
			return fmt.Errorf("%s is not a valid domain against %s", alias, strings.Join(certs, ","))
		}
	}
	return nil
}

func (a *ACM) findValidAliasesAgainstCert(aliases []string, cert string) ([]string, error) {
	resp, err := a.client.DescribeCertificate(&acm.DescribeCertificateInput{
		CertificateArn: aws.String(cert),
	})
	if err != nil {
		return nil, fmt.Errorf("describe certificate %s: %w", cert, err)
	}
	domainSet := make(map[string]bool)
	domainSet[aws.StringValue(resp.Certificate.DomainName)] = true
	for _, san := range resp.Certificate.SubjectAlternativeNames {
		domainSet[aws.StringValue(san)] = true
	}
	var validAliases []string
	for _, alias := range aliases {
		// See https://docs.aws.amazon.com/acm/latest/userguide/acm-certificate.html
		wildCardMatchedAlias := "*" + alias[strings.Index(alias, "."):]
		if domainSet[alias] || domainSet[wildCardMatchedAlias] {
			validAliases = append(validAliases, alias)
		}
	}
	return validAliases, nil
}
