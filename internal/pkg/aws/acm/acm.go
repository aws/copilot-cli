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
	"github.com/dustin/go-humanize/english"
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
	domainsOfCert := make(map[string][]string)
	ctx, cancelWait := context.WithTimeout(context.Background(), waitForFindValidAliasesTimeout)
	defer cancelWait()
	g, ctx := errgroup.WithContext(ctx)
	var mux sync.Mutex
	for i := range certs {
		cert := certs[i]
		g.Go(func() error {
			domains, err := a.validDomainsOfCert(ctx, cert)
			if err != nil {
				return err
			}
			validCertAliases := filterValidAliases(domains, aliases)
			mux.Lock()
			defer mux.Unlock()
			domainsOfCert[cert] = domains
			for _, alias := range validCertAliases {
				validAliases[alias] = true
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	for _, alias := range aliases {
		if !validAliases[alias] {
			return &errInValidAliasAgainstCert{
				certs:         certs,
				alias:         alias,
				domainsOfCert: domainsOfCert,
			}
		}
	}
	return nil
}

func (a *ACM) validDomainsOfCert(ctx context.Context, cert string) ([]string, error) {
	resp, err := a.client.DescribeCertificateWithContext(ctx, &acm.DescribeCertificateInput{
		CertificateArn: aws.String(cert),
	})
	if err != nil {
		return nil, fmt.Errorf("describe certificate %s: %w", cert, err)
	}
	var domainsOfCert []*string
	domainsOfCert = append(domainsOfCert, resp.Certificate.SubjectAlternativeNames...)
	return aws.StringValueSlice(domainsOfCert), err
}

func filterValidAliases(domains []string, aliases []string) []string {
	domainSet := make(map[string]bool, len(domains))
	for _, v := range domains {
		domainSet[v] = true
	}
	var validAliases []string
	for _, alias := range aliases {
		// See https://docs.aws.amazon.com/acm/latest/userguide/acm-certificate.html
		wildCardMatchedAlias := "*" + alias[strings.Index(alias, "."):]
		if domainSet[alias] || domainSet[wildCardMatchedAlias] {
			validAliases = append(validAliases, alias)
		}
	}
	return validAliases
}

type errInValidAliasAgainstCert struct {
	certs         []string
	alias         string
	domainsOfCert map[string][]string
}

func (e *errInValidAliasAgainstCert) Error() string {
	return fmt.Sprintf("%s is not a valid domain against %s", e.alias, strings.Join(e.certs, ","))
}

func (e *errInValidAliasAgainstCert) RecommendActions() string {
	var logMsg string
	logMsg = fmt.Sprintf("Please use aliases that are protected by %s your imported:\n", english.Plural(len(e.certs), "certificate", ""))
	for cert, sans := range e.domainsOfCert {
		logMsg += fmt.Sprintf("%q: %s\n", cert, english.WordSeries(sans, ","))
	}
	return logMsg
}
