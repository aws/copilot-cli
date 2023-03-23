// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
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

	for i, patch := range patches {
		var err error
		switch patch.Operation {
		case "add":
			err = patch.applyAdd(&root)
		case "remove":
			err = patch.applyRemove(&root)
		case "replace":
			err = patch.applyReplace(&root)
		default:
			return nil, fmt.Errorf("unsupported operation %q: supported operations are %q, %q, and %q.", patch.Operation, "add", "remove", "replace")
		}
		if err != nil {
			return nil, fmt.Errorf("unable to apply the %q patch at index %d: %w", patch.Operation, i, err)
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

type yamlPatch struct {
	Operation string `yaml:"op"`

	// Path is in JSON Pointer syntax: https://www.rfc-editor.org/rfc/rfc6901
	Path  string    `yaml:"path"`
	Value yaml.Node `yaml:"value"`
}

func (p *yamlPatch) applyAdd(root *yaml.Node) error {
	if p.Value.IsZero() {
		return fmt.Errorf("value required")
	}

	pointer := p.Pointer()
	parent, err := getNode(root, pointer.Parent(), nil)
	if err != nil {
		return err
	}

	switch parent.Kind {
	case yaml.DocumentNode:
		return parent.Encode(p.Value)
	case yaml.MappingNode:
		i, err := findInMap(parent, pointer.FinalKey(), pointer.Parent())
		if err == nil {
			// if the key is in this map, they are trying to replace it
			return parent.Content[i+1].Encode(p.Value)
		}

		// if the key isn't in this map, then we need to create it for them
		parent.Content = append(parent.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: pointer.FinalKey(),
		})
		parent.Content = append(parent.Content, &p.Value)
	case yaml.SequenceNode:
		if pointer.FinalKey() == "-" {
			// add to end of sequence
			parent.Content = append(parent.Content, &p.Value)
			return nil
		}

		idx, err := idxOrError(pointer.FinalKey(), len(parent.Content), pointer.Parent())
		if err != nil {
			return err
		}

		// add node at idx
		parent.Content = append(parent.Content[:idx], append([]*yaml.Node{&p.Value}, parent.Content[idx:]...)...)
	default:
		return &errInvalidNodeKind{
			pointer: pointer.Parent(),
			kind:    parent.Kind,
		}
	}

	return nil
}

func (p *yamlPatch) applyRemove(root *yaml.Node) error {
	pointer := p.Pointer()
	parent, err := getNode(root, pointer.Parent(), nil)
	if err != nil {
		return err
	}

	switch parent.Kind {
	case yaml.DocumentNode:
		// make sure we are encoding zero into node
		p.Value = yaml.Node{}
		return parent.Encode(p.Value)
	case yaml.MappingNode:
		i, err := findInMap(parent, pointer.FinalKey(), pointer.Parent())
		if err != nil {
			return err
		}

		parent.Content = append(parent.Content[:i], parent.Content[i+2:]...)
	case yaml.SequenceNode:
		idx, err := idxOrError(pointer.FinalKey(), len(parent.Content)-1, pointer.Parent())
		if err != nil {
			return err
		}

		parent.Content = append(parent.Content[:idx], parent.Content[idx+1:]...)
	default:
		return &errInvalidNodeKind{
			pointer: pointer.Parent(),
			kind:    parent.Kind,
		}
	}

	return nil
}

func (p *yamlPatch) applyReplace(root *yaml.Node) error {
	if p.Value.IsZero() {
		return fmt.Errorf("value required")
	}

	pointer := p.Pointer()
	node, err := getNode(root, pointer, nil)
	if err != nil {
		return err
	}

	return node.Encode(p.Value)
}

type pointer []string

// Parent returns a pointer to the parent of p.
func (p pointer) Parent() pointer {
	if len(p) == 0 {
		return nil
	}
	return p[:len(p)-1]
}

func (p pointer) FinalKey() string {
	if len(p) == 0 {
		return ""
	}
	return p[len(p)-1]
}

func (y yamlPatch) Pointer() pointer {
	split := strings.Split(y.Path, "/")
	for i := range split {
		// apply replacements as described https://www.rfc-editor.org/rfc/rfc6901#section-4
		split[i] = strings.ReplaceAll(split[i], "~1", "/")
		split[i] = strings.ReplaceAll(split[i], "~0", "~")
	}

	return split
}

// findInMap returns the index of the _key_ node in a mapping node's Content.
// The index of the _value_ node is the returned index+1.
//
// If key is not in the map, an error is returned.
func findInMap(node *yaml.Node, key string, traversed pointer) (int, error) {
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return i, nil
		}
	}

	return 0, fmt.Errorf("key %q: %q not found in map", strings.Join(traversed, jsonPointerSeparator), key)
}

func getNode(node *yaml.Node, remaining, traversed pointer) (*yaml.Node, error) {
	if len(remaining) == 0 {
		return node, nil
	}

	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) == 0 {
			return nil, fmt.Errorf("invalid yaml document node with no content") // shouldn't ever happen
		}

		return getNode(node.Content[0], remaining[1:], append(traversed, remaining[0]))
	case yaml.MappingNode:
		i, err := findInMap(node, remaining[0], traversed)
		if err != nil {
			return nil, err
		}

		return getNode(node.Content[i+1], remaining[1:], append(traversed, remaining[0]))
	case yaml.SequenceNode:
		idx, err := idxOrError(remaining[0], len(node.Content)-1, traversed)
		if err != nil {
			return nil, err
		}

		return getNode(node.Content[idx], remaining[1:], append(traversed, remaining[0]))
	default:
		return nil, &errInvalidNodeKind{
			pointer: traversed,
			kind:    node.Kind,
		}
	}
}

func idxOrError(key string, maxIdx int, traversed pointer) (int, error) {
	idx, err := strconv.Atoi(key)
	switch {
	case err != nil:
		return 0, fmt.Errorf("key %q: expected index in sequence, got %q", strings.Join(traversed, jsonPointerSeparator), key)
	case idx < 0 || idx > maxIdx:
		return 0, fmt.Errorf("key %q: index %d out of bounds for sequence of length %d", strings.Join(traversed, jsonPointerSeparator), idx, maxIdx)
	}

	return idx, nil
}

type errInvalidNodeKind struct {
	pointer pointer
	kind    yaml.Kind
}

func (e *errInvalidNodeKind) Error() string {
	return fmt.Sprintf("key %q: invalid node type %s", strings.Join(e.pointer, jsonPointerSeparator), nodeKindStringer(e.kind))
}

type nodeKindStringer yaml.Kind

func (k nodeKindStringer) String() string {
	switch yaml.Kind(k) {
	case yaml.DocumentNode:
		return "document"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return fmt.Sprintf("%#v", k)
	}
}
