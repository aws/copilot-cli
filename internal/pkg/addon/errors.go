// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"errors"
	"fmt"

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

	FirstFileName  string
	SecondFileName string
}

func (e *errKeyAlreadyExists) Error() string {
	return fmt.Sprintf(`"%s" defined in "%s" at Ln %d, Col %d is different than in "%s" at Ln %d, Col %d`,
		e.Key, e.FirstFileName, e.First.Line, e.First.Column, e.SecondFileName, e.Second.Line, e.Second.Column)
}

// errMetadataAlreadyExists occurs if two addons have the same key in their "Metadata" section but with different values.
type errMetadataAlreadyExists struct {
	*errKeyAlreadyExists
}

func (e *errMetadataAlreadyExists) Error() string {
	return fmt.Sprintf(`metadata key %s`, e.errKeyAlreadyExists.Error())
}

// errParameterAlreadyExists occurs if two addons have the same parameter logical ID but with different values.
type errParameterAlreadyExists struct {
	*errKeyAlreadyExists
}

func (e *errParameterAlreadyExists) Error() string {
	return fmt.Sprintf(`parameter logical ID %s`, e.errKeyAlreadyExists.Error())
}

// errMappingAlreadyExists occurs if the named values for the same Mappings key have different values.
type errMappingAlreadyExists struct {
	*errKeyAlreadyExists
}

func (e *errMappingAlreadyExists) Error() string {
	return fmt.Sprintf(`mapping %s`, e.errKeyAlreadyExists.Error())
}

// errConditionAlreadyExists occurs if two addons have the same Conditions key but with different values.
type errConditionAlreadyExists struct {
	*errKeyAlreadyExists
}

func (e *errConditionAlreadyExists) Error() string {
	return fmt.Sprintf(`condition %s`, e.errKeyAlreadyExists.Error())
}

// errResourceAlreadyExists occurs if two addons have the same resource under Resources but with different values.
type errResourceAlreadyExists struct {
	*errKeyAlreadyExists
}

func (e *errResourceAlreadyExists) Error() string {
	return fmt.Sprintf(`resource %s`, e.errKeyAlreadyExists.Error())
}

// errOutputAlreadyExists occurs if two addons have the same output under Outputs but with different values.
type errOutputAlreadyExists struct {
	*errKeyAlreadyExists
}

func (e *errOutputAlreadyExists) Error() string {
	return fmt.Sprintf(`output %s`, e.errKeyAlreadyExists.Error())
}

// wrapKeyAlreadyExistsErr wraps the err if its an errKeyAlreadyExists error with additional cfn section metadata.
// If the error is not an errKeyAlreadyExists, then return it as is.
func wrapKeyAlreadyExistsErr(section cfnSection, merged, newTpl *cfnTemplate, err error) error {
	var keyExistsErr *errKeyAlreadyExists
	if !errors.As(err, &keyExistsErr) {
		return err
	}

	keyExistsErr.FirstFileName = merged.templateNameFor[keyExistsErr.First]
	keyExistsErr.SecondFileName = newTpl.name
	switch section {
	case metadataSection:
		return &errMetadataAlreadyExists{
			keyExistsErr,
		}
	case parametersSection:
		return &errParameterAlreadyExists{
			keyExistsErr,
		}
	case mappingsSection:
		return &errMappingAlreadyExists{
			keyExistsErr,
		}
	case conditionsSection:
		return &errConditionAlreadyExists{
			keyExistsErr,
		}
	case resourcesSection:
		return &errResourceAlreadyExists{
			keyExistsErr,
		}
	case outputsSection:
		return &errOutputAlreadyExists{
			keyExistsErr,
		}
	default:
		return keyExistsErr
	}
}
