// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package acm provides a client to make API requests to AWS Certificate Manager.
package acm

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"golang.org/x/sync/errgroup"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
)

const (
	waitForFindValidAliasesTimeout = 10 * time.Second
)

type api interface {
	DescribeCertificateWithContext(ctx aws.Context, input *acm.DescribeCertificateInput, opts ...request.Option) (*acm.DescribeCertificateOutput, error)
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
	AllowedDomainsCert := make(map[string][]string)
	ctx, cancelWait := context.WithTimeout(context.Background(), waitForFindValidAliasesTimeout)
	defer cancelWait()
	g, ctx := errgroup.WithContext(ctx)
	var mux sync.Mutex
	for i := range certs {
		cert := certs[i]
		g.Go(func() error {
			validCertAliases, domainsperCert, err := a.findValidAliasesAndAllowedDomainsAgainstCert(ctx, aliases, cert)
			if err != nil {
				return err
			}
			mux.Lock()
			defer mux.Unlock()
			for _, alias := range validCertAliases {
				validAliases[alias] = true
			}
			for certARN, domains := range domainsperCert {
				AllowedDomainsCert[certARN] = domains
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	for _, alias := range aliases {
		if !validAliases[alias] {
			for certARN, domains := range AllowedDomainsCert {
				log.Infoln(fmt.Sprintf("Your imported Certificate '%s' protects the following domains:", certARN))
				for _, domain := range domains {
					log.Infoln(domain)
				}
			}
			return fmt.Errorf("%s is not a valid domain against %s", alias, strings.Join(certs, ","))
		}
	}
	return nil
}

func (a *ACM) findValidAliasesAndAllowedDomainsAgainstCert(ctx context.Context, aliases []string, cert string) ([]string, map[string][]string, error) {
	resp, err := a.client.DescribeCertificateWithContext(ctx, &acm.DescribeCertificateInput{
		CertificateArn: aws.String(cert),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("describe certificate %s: %w", cert, err)
	}
	domainSet := make(map[string]bool)
	domainsperCert := make(map[string][]string)
	domainSet[aws.StringValue(resp.Certificate.DomainName)] = true
	for _, san := range resp.Certificate.SubjectAlternativeNames {
		domainSet[aws.StringValue(san)] = true
		domainsperCert[cert] = append(domainsperCert[cert], aws.StringValue(san))
	}
	var validAliases []string
	for _, alias := range aliases {
		// See https://docs.aws.amazon.com/acm/latest/userguide/acm-certificate.html
		wildCardMatchedAlias := "*" + alias[strings.Index(alias, "."):]
		if domainSet[alias] || domainSet[wildCardMatchedAlias] {
			validAliases = append(validAliases, alias)
		}
	}
	return validAliases, domainsperCert, nil
}
