// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package route53

import (
	"fmt"
)

// ErrDomainHostedZoneNotFound occurs when the domain hosted zone is not found.
type ErrDomainHostedZoneNotFound struct {
	domainName string
}

func (e *ErrDomainHostedZoneNotFound) Error() string {
	return fmt.Sprintf("hosted zone is not found for domain %s", e.domainName)
}

// ErrDomainNotFound occurs when the domain is not found in the account.
type ErrDomainNotFound struct {
	domainName string
}

func (e *ErrDomainNotFound) Error() string {
	return fmt.Sprintf("domain %s is not found in the account", e.domainName)
}
