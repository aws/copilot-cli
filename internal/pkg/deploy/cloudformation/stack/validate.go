// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
)

// container dependency status constants.
const (
	dependsOnStart    = "start"
	dependsOnComplete = "complete"
	dependsOnSuccess  = "success"
)

// Empty field errors.
var (
	errNoFSID          = errors.New("volume field `efs.id` cannot be empty")
	errNoContainerPath = errors.New("`path` cannot be empty")
	errNoSourceVolume  = errors.New("`source_volume` cannot be empty")
	errEmptyEFSConfig  = errors.New("bad EFS configuration: `efs` cannot be empty")
)

// Conditional errors.
var (
	errAccessPointWithRootDirectory = errors.New("`root_directory` must be empty or \"/\" when `access_point` is specified")
	errAccessPointWithoutIAM        = errors.New("`iam` must be true when `access_point` is specified")
	errUIDWithNonManagedFS          = errors.New("UID and GID cannot be specified with non-managed EFS")
	errInvalidUIDGIDConfig          = errors.New("must specify both UID and GID, or neither")
	errInvalidEFSConfig             = errors.New("bad EFS configuration: cannot specify both bool and config")
	errReservedUID                  = errors.New("UID must not be 0")
	errCircularDependency           = errors.New("bad dependency name: circular dependency present")
	errInvalidContainer             = errors.New("container dependency does not exist")
	errInvalidDependsOnStatus       = errors.New("container dependency status must be one of < start | complete | success >")
	errEssentialContainerStatus     = errors.New("essential container dependencies can only have status 'start'")
)

var (
	// Container dependency status options
	DependsOnStatus = []string{dependsOnStart, dependsOnComplete, dependsOnSuccess}
)

// Validate that paths contain only an approved set of characters to guard against command injection.
// We can accept 0-9A-Za-z-_.
func validatePath(input string, maxLength int) error {
	if len(input) > maxLength {
		return fmt.Errorf("path must be less than %d bytes in length", maxLength)
	}
	if len(input) == 0 {
		return nil
	}
	m := pathRegexp.FindStringSubmatch(input)
	if len(m) == 0 {
		return fmt.Errorf("paths can only contain the characters a-zA-Z0-9.-_/")
	}
	return nil
}

func validateStorageConfig(in *manifest.Storage) error {
	if in == nil {
		return nil
	}
	return validateVolumes(in.Volumes)
}

func validateVolumes(in map[string]manifest.Volume) error {
	for name, v := range in {
		if err := validateVolume(name, v); err != nil {
			return err
		}
	}
	return nil
}

func validateVolume(name string, in manifest.Volume) error {
	if err := validateMountPointConfig(in); err != nil {
		return fmt.Errorf("validate container configuration for volume %s: %w", name, err)
	}
	if err := validateEFSConfig(in); err != nil {
		return fmt.Errorf("validate EFS configuration for volume %s: %w", name, err)
	}
	return nil
}

func validateMountPointConfig(in manifest.Volume) error {
	// containerPath must be specified.
	path := aws.StringValue(in.ContainerPath)
	if path == "" {
		return errNoContainerPath
	}
	if err := validateContainerPath(path); err != nil {
		return fmt.Errorf("validate container path %s: %w", path, err)
	}
	return nil
}

func validateSidecarMountPoints(in []manifest.SidecarMountPoint) error {
	if in == nil {
		return nil
	}
	for _, mp := range in {
		if aws.StringValue(mp.ContainerPath) == "" {
			return errNoContainerPath
		}
		if aws.StringValue(mp.SourceVolume) == "" {
			return errNoSourceVolume
		}
	}
	return nil
}

func validateSidecarDependsOn(in manifest.SidecarConfig, sidecarName string, sidecars map[string]*manifest.SidecarConfig, workloadName string) error {
	if in.DependsOn == nil {
		return nil
	}

	for name, status := range in.DependsOn {
		// status must be one of < start | complete | success >
		if !isValidStatus(status) {
			return errInvalidDependsOnStatus
		}
		// essential containers must have 'start' as a status
		if name == workloadName || sidecars[name].Essential == nil || aws.BoolValue(sidecars[name].Essential) {
			if status != "start" {
				return errEssentialContainerStatus
			}
		}
	}

	return nil
}

func validateNoCircularDependencies(sidecars map[string]*manifest.SidecarConfig, img manifest.Image, workloadName string) error {
	used := make(map[string]bool)
	path := make(map[string]bool)
	dependencies, err := buildDependencyGraph(sidecars, img, workloadName)

	// don't let nonexistent containers pass
	if err != nil {
		return err
	}

	// don't let circular dependencies happen
	for node := range dependencies.nodes {
		if !used[node] && hasCycles(dependencies, used, path, node) {
			return errCircularDependency
		}
	}

	return nil
}

func hasCycles(graph *Graph, used map[string]bool, path map[string]bool, currNode string) bool {
	used[currNode] = true
	path[currNode] = true

	for _, node := range graph.nodes[currNode] {
		if !used[node] && hasCycles(graph, used, path, node) {
			return true
		} else if path[node] {
			return true
		}
	}

	path[currNode] = false
	return false
}

func buildDependencyGraph(sidecars map[string]*manifest.SidecarConfig, img manifest.Image, workloadName string) (*Graph, error) {
	dependencyGraph := Graph{nodes: make(map[string][]string)}

	// sidecar dependencies
	for name, sidecar := range sidecars {
		// add any existing dependency to the graph
		if len(sidecar.DependsOn) != 0 {
			for dep := range sidecar.DependsOn {
				// containers being depended on must exist
				if sidecars[dep] == nil && dep != workloadName {
					return nil, errInvalidContainer
				}

				dependencyGraph.Add(name, dep)
			}
		}
	}

	// add any image dependencies to the graph
	if len(img.DependsOn) != 0 {
		for dep := range img.DependsOn {
			// containers being depended on must exist
			if sidecars[dep] == nil && dep != workloadName {
				return nil, errInvalidContainer
			}

			dependencyGraph.Add(workloadName, dep)
		}
	}

	return &dependencyGraph, nil
}

type Graph struct {
	nodes map[string][]string
}

func (graph *Graph) Add(fromNode, toNode string) {
	hasNode := false

	// add origin node if doesn't exist
	if graph.nodes[fromNode] == nil {
		graph.nodes[fromNode] = []string{}
	}

	// check if edge exists between from and to nodes
	for _, node := range graph.nodes[fromNode] {
		if node == toNode {
			hasNode = true
		}
	}

	// add edge if not there already
	if !hasNode {
		graph.nodes[fromNode] = append(graph.nodes[fromNode], toNode)
	}
}

func validateImageDependsOn(img manifest.Image, sidecars map[string]*manifest.SidecarConfig, workloadName string) error {
	if img.DependsOn == nil {
		return nil
	}

	for name, status := range img.DependsOn {
		// status must be one of < start | complete | success >
		if !isValidStatus(status) {
			return errInvalidDependsOnStatus
		}
		// essential containers must have 'start' as a status
		if sidecars != nil {
			if sidecars[name].Essential == nil || *sidecars[name].Essential {
				if status != "start" {
					return errEssentialContainerStatus
				}
			}
		}
	}

	return validateNoCircularDependencies(sidecars, img, workloadName)
}

func isValidStatus(s string) bool {
	for _, allowed := range DependsOnStatus {
		if s == allowed {
			return true
		}
	}

	return false
}

func validateEFSConfig(in manifest.Volume) error {
	// EFS is implicitly disabled. We don't use the attached EmptyVolume function here
	// because it may hide invalid config.
	if in.EFS == nil {
		return nil
	}

	// EFS cannot have both Enabled and nonempty Advanced config.
	if aws.BoolValue(in.EFS.Enabled) && !in.EFS.Advanced.IsEmpty() {
		return errInvalidEFSConfig
	}

	// EFS can be disabled explicitly.
	if in.EFS.Disabled() {
		return nil
	}

	// EFS cannot be an empty map.
	if in.EFS.Enabled == nil && in.EFS.Advanced.IsEmpty() {
		return errEmptyEFSConfig
	}

	// UID and GID are mutually exclusive with any other fields.
	if !(in.EFS.Advanced.EmptyBYOConfig() || in.EFS.Advanced.EmptyUIDConfig()) {
		return errUIDWithNonManagedFS
	}

	// Check that required fields for BYO EFS are satisfied.
	if !in.EFS.Advanced.EmptyBYOConfig() && !in.EFS.Advanced.IsEmpty() {
		if aws.StringValue(in.EFS.Advanced.FileSystemID) == "" {
			return errNoFSID
		}
	}

	if err := validateRootDirPath(aws.StringValue(in.EFS.Advanced.RootDirectory)); err != nil {
		return err
	}

	if err := validateAuthConfig(in.EFS.Advanced); err != nil {
		return err
	}

	if err := validateUIDGID(in.EFS.Advanced.UID, in.EFS.Advanced.GID); err != nil {
		return err
	}

	return nil
}

func validateAuthConfig(in manifest.EFSVolumeConfiguration) error {
	if in.AuthConfig == nil {
		return nil
	}
	rd := aws.StringValue(in.RootDirectory)
	if !(rd == "" || rd == "/") && in.AuthConfig.AccessPointID != nil {
		return errAccessPointWithRootDirectory
	}

	if in.AuthConfig.AccessPointID != nil && !aws.BoolValue(in.AuthConfig.IAM) {
		return errAccessPointWithoutIAM
	}

	return nil
}

func validateUIDGID(uid, gid *uint32) error {
	if uid == nil && gid == nil {
		return nil
	}
	if uid != nil && gid == nil {
		return errInvalidUIDGIDConfig
	}
	if uid == nil && gid != nil {
		return errInvalidUIDGIDConfig
	}
	// Check for root UID.
	if aws.Uint32Value(uid) == 0 {
		return errReservedUID
	}
	return nil
}

func validateRootDirPath(input string) error {
	return validatePath(input, maxEFSPathLength)
}

func validateContainerPath(input string) error {
	return validatePath(input, maxDockerContainerPathLength)
}
