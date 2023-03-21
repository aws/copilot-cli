// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

const jsonPointerSeparator = "/"

// Patch applies overrides configured as JSON Patches,
// as defined in https://www.rfc-editor.org/rfc/rfc6902.
type Patch struct {
	filePath string   // Absolute path to the overrides/ directory.
	fs       afero.Fs // OS file system.
}

// PatchOpts is optional configuration for initializing a Patch Overrider.
type PatchOpts struct {
	FS afero.Fs // File system interface. If nil, defaults to the OS file system.
}

// WithPatch instantiates a new Patch Overrider with root being the path to the overrides/ directory.
// It supports a single file (cfn.patches.yml) with configured patches.
func WithPatch(filePath string, opts PatchOpts) *Patch {
	fs := afero.NewOsFs()
	if opts.FS != nil {
		fs = opts.FS
	}

	return &Patch{
		filePath: filePath,
		fs:       fs,
	}
}

type yamlPatch struct {
	Operation string `yaml:"op"`

	// Path is in JSON Pointer syntax: https://www.rfc-editor.org/rfc/rfc6901
	Path  string    `yaml:"path"`
	Value yaml.Node `yaml:"value"`
}

// Override returns the overriden CloudFormation template body
// after applying YAML patches to it.
func (p *Patch) Override(body []byte) ([]byte, error) {
	patches, err := unmarshalPatches(p.filePath, p.fs)
	if err != nil {
		return nil, err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}

	for _, patch := range patches {
		var err error
		switch patch.Operation {
		case "add":
			err = patch.applyAdd(&root)
		case "remove":
			err = patch.applyRemove(&root)
		case "replace":
			err = patch.applyReplace(&root)
		default:
			return nil, fmt.Errorf("unsupported operation %q. supported operations are %q, %q, and %q.", patch.Operation, "add", "remove", "replace")
		}
		if err != nil {
			return nil, fmt.Errorf("unable to apply %q patch: %w", patch.Operation, err)
		}
	}

	out, err := yaml.Marshal(&root)
	if err != nil {
		return nil, fmt.Errorf("unable to return modified document to []byte: %w", err)
	}
	return out, nil
}

func unmarshalPatches(path string, fs afero.Fs) ([]yamlPatch, error) {
	content, err := afero.ReadFile(fs, path)
	if err != nil {
		return nil, fmt.Errorf("read file at %q: %w", path, err)
	}

	var patches []yamlPatch
	if err := yaml.Unmarshal(content, &patches); err != nil {
		return nil, fmt.Errorf("file at %q does not conform to the YAML patch document schema: %w", path, err)
	}

	return patches, nil
}

func (p *yamlPatch) applyAdd(root *yaml.Node) error {
	if p.Value.IsZero() {
		return fmt.Errorf("value required")
	}

	return followJSONPointer(root, p.Path, func(node *yaml.Node, pointer []string) error {
		if len(pointer) != 1 {
			return nil
		}

		switch node.Kind {
		case yaml.DocumentNode:
			return p.encodeAndStop(node)
		case yaml.MappingNode:
			// if the key is in this map, they are trying to replace it
			for i := 0; i < len(node.Content); i += 2 {
				if node.Content[i].Value == pointer[0] {
					return p.encodeAndStop(node.Content[i+1])
				}
			}

			// if the key isn't in this map, then we need to create it for them
			node.Content = append(node.Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: pointer[0],
			})
			node.Content = append(node.Content, &p.Value)
			return errStopFollowingPointer
		case yaml.SequenceNode:
			if pointer[0] == "-" {
				// add to end of sequence
				node.Content = append(node.Content, &p.Value)
				return errStopFollowingPointer
			}

			idx, err := strconv.Atoi(pointer[0])
			switch {
			case err != nil:
				return fmt.Errorf("expected index in sequence, got %q", pointer[0])
			case idx < 0 || idx > len(node.Content)-1:
				return fmt.Errorf("invalid index %d for sequence of length %d", idx, len(node.Content))
			}

			// add node at idx
			node.Content = append(node.Content[:idx], append([]*yaml.Node{&p.Value}, node.Content[idx:]...)...)
			return errStopFollowingPointer
		}

		return nil
	})
}

func (p *yamlPatch) applyRemove(root *yaml.Node) error {
	return followJSONPointer(root, p.Path, func(node *yaml.Node, pointer []string) error {
		if len(pointer) != 1 {
			return nil
		}

		switch node.Kind {
		case yaml.DocumentNode:
			// make sure we are encoding zero into node
			p.Value = yaml.Node{}
			return p.encodeAndStop(node)
		case yaml.MappingNode:
			for i := 0; i < len(node.Content); i += 2 {
				if node.Content[i].Value == pointer[0] {
					node.Content = append(node.Content[:i], node.Content[i+2:]...)
					return errStopFollowingPointer
				}
			}
		case yaml.SequenceNode:
			idx, err := strconv.Atoi(pointer[0])
			switch {
			case err != nil:
				return fmt.Errorf("expected index in sequence, got %q", pointer[0])
			case idx < 0 || idx > len(node.Content)-1:
				return fmt.Errorf("invalid index %d for sequence of length %d", idx, len(node.Content))
			}

			node.Content = append(node.Content[:idx], node.Content[idx+1:]...)
			return errStopFollowingPointer
		}

		return nil
	})
}

func (p *yamlPatch) applyReplace(root *yaml.Node) error {
	if p.Value.IsZero() {
		return fmt.Errorf("value required")
	}

	return followJSONPointer(root, p.Path, func(node *yaml.Node, pointer []string) error {
		if len(pointer) > 0 {
			return nil
		}
		return p.encodeAndStop(node)
	})
}

func (p *yamlPatch) encodeAndStop(node *yaml.Node) error {
	if err := node.Encode(p.Value); err != nil {
		return err
	}
	return errStopFollowingPointer
}

var errStopFollowingPointer = errors.New("stop following pointer")

type visitNodeFunc func(node *yaml.Node, remaining []string) error

func followJSONPointer(root *yaml.Node, pointer string, visit visitNodeFunc) error {
	split := strings.Split(pointer, "/")
	for i := range split {
		// apply replacements as described https://www.rfc-editor.org/rfc/rfc6901#section-4
		split[i] = strings.ReplaceAll(split[i], "~1", "/")
		split[i] = strings.ReplaceAll(split[i], "~0", "~")
	}

	return followJSONPointerHelper(root, nil, split, visit)
}

func followJSONPointerHelper(node *yaml.Node, traversed, remaining []string, visit visitNodeFunc) error {
	if err := visit(node, remaining); err != nil {
		if errors.Is(err, errStopFollowingPointer) {
			return nil
		}
		return fmt.Errorf("key %q: %w", strings.Join(traversed, jsonPointerSeparator), err)
	} else if len(remaining) == 0 {
		return nil
	}

	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) == 0 {
			return nil // weird, but ok ¯\_(ツ)_/¯
		}

		return followJSONPointerHelper(node.Content[0], append(traversed, remaining[0]), remaining[1:], visit)
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == remaining[0] {
				return followJSONPointerHelper(node.Content[i+1], append(traversed, remaining[0]), remaining[1:], visit)
			}
		}

		return fmt.Errorf("key %q: %q not found in map", strings.Join(traversed, jsonPointerSeparator), remaining[0])
	case yaml.SequenceNode:
		if remaining[0] == "-" {
			return followJSONPointerHelper(node.Content[len(node.Content)-1], append(traversed, remaining[0]), remaining[1:], visit)
		}

		idx, err := strconv.Atoi(remaining[0])
		switch {
		case err != nil:
			return fmt.Errorf("key %q: expected index in sequence, got %q", strings.Join(traversed, jsonPointerSeparator), remaining[0])
		case idx < 0 || idx > len(node.Content)-1:
			return fmt.Errorf("key %q: index %d out of bounds for sequence of length %d", strings.Join(traversed, jsonPointerSeparator), idx, len(node.Content))
		}

		return followJSONPointerHelper(node.Content[idx], append(traversed, remaining[0]), remaining[1:], visit)
	default:
		return fmt.Errorf("key %q: invalid node type %#v", strings.Join(traversed, jsonPointerSeparator), node.Kind)
	}
}
