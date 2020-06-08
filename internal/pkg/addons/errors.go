// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"gopkg.in/yaml.v3"
)

// ErrDirNotExist occurs when an addons directory for a service does not exist.
type ErrDirNotExist struct {
	SvcName   string
	ParentErr error
}

func (e *ErrDirNotExist) Error() string {
	return fmt.Sprintf("read addons directory for service %s: %v", e.SvcName, e.ParentErr)
}

type errKeyAlreadyExists struct {
	Key    string
	First  *yaml.Node
	Second *yaml.Node
}

func (e *errKeyAlreadyExists) Error() string {
	return fmt.Sprintf("key %s already exists with a different definition", e.Key)
}

// errMetadataKeyAlreadyExists occurs if two addons have the same key in their "Metadata" section but with different values.
type errMetadataKeyAlreadyExists struct {
	*errKeyAlreadyExists
}

func (e *errMetadataKeyAlreadyExists) Error() string {
	return fmt.Sprintf("metadata key %s already exists with a different definition", e.Key)
}

// HumanError returns a string that explains the error with human-friendly details.
func (e *errMetadataKeyAlreadyExists) HumanError() string {
	fout, _ := yaml.Marshal(e.First)
	sout, _ := yaml.Marshal(e.Second)
	return fmt.Sprintf(`The "Metadata" key %s exists with two different values under addons:
%s
and
%s`,
		color.HighlightCode(e.Key),
		color.HighlightCode(strings.TrimSpace(string(fout))),
		color.HighlightCode(strings.TrimSpace(string(sout))))
}

// wrapKeyAlreadyExistsErr wraps the err if its an errKeyAlreadyExists error with additional cfn section metadata.
// If the error is not an errKeyAlreadyExists, then return it as is.
func wrapKeyAlreadyExistsErr(section cfnSection, err error) error {
	var keyExistsErr *errKeyAlreadyExists
	if !errors.As(err, &keyExistsErr) {
		return err
	}
	switch section {
	case metadataSection:
		return &errMetadataKeyAlreadyExists{
			errKeyAlreadyExists: keyExistsErr,
		}
	default:
		return err
	}
}
