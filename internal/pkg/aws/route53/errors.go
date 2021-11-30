// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package route53

import (
	"errors"
	"fmt"
)

// ErrNoDefaultCluster occurs when the default cluster is not found.
var ErrDomainNotExist = errors.New("domain does not exist")

// ErrDomainNotFound occurs when the domain is not found in the account.
type ErrDomainNotFound struct {
	domainName string
}

func (e *ErrDomainNotFound) Error() string {
	return fmt.Sprintf("domain %s is not found in the account", e.domainName)
}
