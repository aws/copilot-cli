// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

// Patch applies overrides configured as JSON Patches,
// as defined in https://www.rfc-editor.org/rfc/rfc6902.
type Patch struct {
	rootAbsPath string   // Absolute path to the overrides/ directory.
	fs          afero.Fs // OS file system.
}

type PatchOpts struct {
	FS afero.Fs // File system interface. If nil, defaults to the OS file system.
}

func WithPatch(root string, opts PatchOpts) *Patch {
	fs := afero.NewOsFs()
	if opts.FS != nil {
		fs = opts.FS
	}

	return &Patch{
		rootAbsPath: root,
		fs:          fs,
	}
}

type yamlPatch struct {
	Operation string `yaml:"op"`

	// Path is in JSON Pointer syntax: https://www.rfc-editor.org/rfc/rfc6901
	Path  string    `yaml:"path"`
	Value yaml.Node `yaml:"value"`
}

func (p *Patch) Override(body []byte) ([]byte, error) {
	patches, err := p.unmarshalPatches()
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
			err = patch.doAdd(&root)
		case "remove":
			err = patch.doRemove(&root)
		case "replace":
			err = patch.applyReplace(&root)
		default:
			return nil, fmt.Errorf("unsupported operation %q", patch.Operation)
		}
		if err != nil {
			return nil, fmt.Errorf("unable to apply %q patch at %q: %w", patch.Operation, patch.Path, err)
		}
	}

	out, err := yaml.Marshal(&root)
	if err != nil {
		return nil, fmt.Errorf("unable to return modified document to []byte: %w", err)
	}
	return out, nil
}

func (p *Patch) unmarshalPatches() ([]yamlPatch, error) {
	var patches []yamlPatch

	files, err := afero.ReadDir(p.fs, p.rootAbsPath)
	if err != nil {
		return nil, fmt.Errorf("read directory %q: %w", p.rootAbsPath, err)
	}

	for _, file := range files {
		path := filepath.Join(p.rootAbsPath, file.Name())
		content, err := afero.ReadFile(p.fs, path)
		if err != nil {
			return nil, fmt.Errorf("read file at %q: %w", path, err)
		}

		var filePatches []yamlPatch
		if err := yaml.Unmarshal(content, &filePatches); err != nil {
			return nil, fmt.Errorf("file at %q does not conform to the YAML patch document schema: %w", path, err)
		}

		patches = append(patches, filePatches...)
	}

	return patches, nil
}

func (y *yamlPatch) doAdd(root *yaml.Node) error {
	return followJSONPointer(root, y.Path, func(node *yaml.Node, pointer []string) error {
		if len(pointer) != 1 {
			return nil
		}

		switch node.Kind {
		case yaml.MappingNode:
			// if the key is in this map, they are trying to replace it
			for i := 0; i < len(node.Content); i += 2 {
				if node.Content[i].Value == pointer[0] {
					return y.encodeAndStop(node.Content[i+1])
				}
			}

			// if the key isn't in this map, then we need to create it for them
			node.Content = append(node.Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: pointer[0],
			})
			node.Content = append(node.Content, &y.Value)
			return errStopFollowingPointer
		case yaml.SequenceNode:
			if pointer[0] == "-" || pointer[0] == "" {
				// add to end of sequence
				node.Content = append(node.Content, &y.Value)
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
			node.Content = append(node.Content[:idx], append([]*yaml.Node{&y.Value}, node.Content[idx:]...)...)
			return errStopFollowingPointer
		}

		return nil
	})
}

func (y *yamlPatch) doRemove(root *yaml.Node) error {
	return followJSONPointer(root, y.Path, func(node *yaml.Node, pointer []string) error {
		if len(pointer) != 1 {
			return nil
		}

		switch node.Kind {
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

func (y *yamlPatch) applyReplace(root *yaml.Node) error {
	return followJSONPointer(root, y.Path, func(node *yaml.Node, pointer []string) error {
		if len(pointer) > 0 {
			return nil
		}
		return y.encodeAndStop(node)
	})
}

func (y *yamlPatch) encodeAndStop(node *yaml.Node) error {
	if err := node.Encode(y.Value); err != nil {
		return err
	}
	return errStopFollowingPointer
}

var errStopFollowingPointer = errors.New("stop following pointer")

func followJSONPointer(root *yaml.Node, pointer string, visit func(node *yaml.Node, pointer []string) error) error {
	split := strings.Split(pointer, "/")
	for i := range split {
		split[i] = strings.ReplaceAll(split[i], "~0", "~")
		split[i] = strings.ReplaceAll(split[i], "~1", "/")
	}

	return followJSONPointerHelper(root, split, visit)
}

func followJSONPointerHelper(node *yaml.Node, pointer []string, visit func(node *yaml.Node, pointer []string) error) error {
	if err := visit(node, pointer); err != nil {
		if errors.Is(err, errStopFollowingPointer) {
			return nil
		}
		return err
	} else if len(pointer) == 0 {
		return nil
	}

	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) != 1 {
			return fmt.Errorf("don't support multi-doc yaml")
		}

		return followJSONPointerHelper(node.Content[0], pointer[1:], visit)
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == pointer[0] {
				return followJSONPointerHelper(node.Content[i+1], pointer[1:], visit)
			}
		}

		return fmt.Errorf("key %q not found in map", pointer[0])
	case yaml.SequenceNode:
		idx, err := strconv.Atoi(pointer[0])
		switch {
		case err != nil:
			return fmt.Errorf("expected index in sequence, got %q", pointer[0])
		case idx > len(node.Content)-1:
			return fmt.Errorf("invalid index %d for sequence of length %d", idx, len(node.Content))
		}

		return followJSONPointerHelper(node.Content[idx], pointer[1:], visit)
	default:
		return fmt.Errorf("invalid node type %#v for path", node.Kind)
	}
}