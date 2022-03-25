// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import "fmt"

type errSvcWithNoAliasDeployingToEnvWithImportedCerts struct {
	name    string
	envName string
}

func (e *errSvcWithNoAliasDeployingToEnvWithImportedCerts) Error() string {
	return fmt.Sprintf("cannot deploy service %s without custom domain to environment %s with certificate imported", e.name, e.envName)
}
