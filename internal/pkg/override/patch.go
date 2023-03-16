package override

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

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
		if err := patch.apply(&root); err != nil {
			return nil, fmt.Errorf("unable to apply patch with operation %q at %q: %w", patch.Operation, patch.Path, err)
		}
	}

	// marshal back to []byte
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

func nodeString(node *yaml.Node) string {
	var v interface{}
	if err := node.Decode(&v); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%+v", v)
}

func printSplit(split []string) {
	fmt.Printf("split: [")
	for i, s := range split {
		fmt.Printf("%q", s)
		if i < len(split)-1 {
			fmt.Printf(", ")
		}
	}
	fmt.Printf("]\n")
}

// add's target is any map/sequence node
// replace's target is a map/sequence/scalar node
// remove's target is a map/sequence node

// replace needs the target map/sequence node
// remove path is the VALUE node. node should be the parent of that node.

func (y yamlPatch) apply(root *yaml.Node) error {
	switch y.Operation {
	case "add":
		node, err := getNode(root, y.Path)
		if err != nil {
			return fmt.Errorf("unable to apply patch with operation %q at %q: %w", y.Operation, y.Path, err)
		}

		node.Content = append(node.Content, y.Value.Content...)
	case "replace":
		node, err := getNode(root, y.Path)
		if err != nil {
			return fmt.Errorf("unable to apply patch with operation %q at %q: %w", y.Operation, y.Path, err)
		}

		node.Encode(y.Value)
	case "remove":
		split := strings.Split(y.Path, "/")
		if len(split) == 0 {
			// TODO remove the whole thing i guess?
			root.Content = nil
			return nil
		}

		key := split[len(split)-1]

		node, err := getNode(root, strings.Join(split[:len(split)-1], "/"))
		if err != nil {
			return fmt.Errorf("unable to apply patch with operation %q at %q: %w", y.Operation, y.Path, err)
		}

		switch node.Kind {
		case yaml.MappingNode:
			for i := 0; i < len(node.Content); i += 2 {
				if node.Content[i].Value == key {
					node.Content = append(node.Content[:i], node.Content[i+2:]...)
					return nil
				}
			}

			return fmt.Errorf("non existant key %q in map", key)
		case yaml.SequenceNode:
			// TODO
		default:
			fmt.Printf("can't remove from yaml node of type %v\n", node.Kind)
		}
	}

	return nil
}

func getNode(node *yaml.Node, path string) (*yaml.Node, error) {
	if path == "" {
		return node, nil
	}

	// follow the JSON pointer pointer down to the node path.
	// fix pointer syntax: https://www.rfc-editor.org/rfc/rfc6901#section-3
	// TODO figure out how to handle the path being "/" " ".
	split := strings.Split(strings.TrimSpace(path), "/")

	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) != 1 {
			return nil, fmt.Errorf("don't support multi-doc yaml")
		}

		return getNode(node.Content[0], strings.Join(split[1:], "/"))
	case yaml.MappingNode:
		// find the requested node
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == split[0] {
				return getNode(node.Content[i+1], strings.Join(split[1:], "/"))
			}
		}

		return nil, fmt.Errorf("key %q not found in map", split[0])
	case yaml.SequenceNode:
		// this key _should_ be a number or "-"
		var idx int
		if split[0] == "-" {
			idx = len(node.Content)
			// insert a new node to represent the last index
			node.Content = append(node.Content, &yaml.Node{
				Kind: yaml.ScalarNode,
			})
		} else {
			var err error
			idx, err = strconv.Atoi(split[0])
			if err != nil {
				return nil, fmt.Errorf("expected index in sequence, got %q", split[0])
			}
		}

		fmt.Printf("returning node at idx: %v\n", idx)
		return getNode(node.Content[idx], strings.Join(split[1:], "/"))
	default:
		// error
		return nil, fmt.Errorf("invalid node type %#v for path", node.Kind)
	}
}
