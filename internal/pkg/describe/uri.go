// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import "fmt"

// URI returns the WebServiceURI to identify this service uniquely given an environment name.
func (d *RDWebServiceDescriber) URI(envName string) (string, error) {
	err := d.initServiceDescriber(envName)
	if err != nil {
		return "", err
	}

	serviceURL, err := d.envSvcDescribers[envName].ServiceURL()
	if err != nil {
		return "", fmt.Errorf("get outputs for service %s: %w", d.svc, err)
	}

	return serviceURL, nil
}
