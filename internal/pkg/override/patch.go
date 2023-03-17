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
		switch patch.Operation {
		case "add":
			if err := patch.applyAdd(&root); err != nil {
				return nil, fmt.Errorf("unable to apply %q patch at %q: %w", patch.Operation, patch.Path, err)
			}
		case "remove":
			if err := patch.applyRemove(&root); err != nil {
				return nil, fmt.Errorf("unable to apply %q patch at %q: %w", patch.Operation, patch.Path, err)
			}
		case "replace":
			if err := patch.applyReplace(&root); err != nil {
				return nil, fmt.Errorf("unable to apply %q patch at %q: %w", patch.Operation, patch.Path, err)
			}
		default:
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

// TODO make sure whole document selector works
// follow the JSON pointer pointer down to the node path.
// fix pointer syntax: https://www.rfc-editor.org/rfc/rfc6901#section-3
// TODO figure out how to handle the path being "/" " ".

func (y yamlPatch) applyAdd(node *yaml.Node) error {
	split := strings.Split(strings.TrimSpace(y.Path), "/")

	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) != 1 {
			return fmt.Errorf("don't support multi-doc yaml")
		}

		y.Path = strings.Join(split[1:], "/")
		return y.applyAdd(node.Content[0])
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == split[0] {
				y.Path = strings.Join(split[1:], "/")
				return y.applyAdd(node.Content[i+1])
			}
		}

		if len(split) > 1 {
			return fmt.Errorf("key %q not found in map", split[0])
		}

		if y.Path == "" {
			// merging node with y.Value (which should be a map)
			node.Content = append(node.Content, y.Value.Content...)
			return nil
		}

		// adding this entry to node: split[0]: y.Value
		node.Content = append(node.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: split[0],
		})
		node.Content = append(node.Content, &y.Value)
		return nil
	case yaml.SequenceNode:
		// TODO idx out of range
		idx, err := strconv.Atoi(split[0])
		if len(split) == 1 {
			if split[0] == "-" || split[0] == "" {
				// append to end of sequence
				node.Content = append(node.Content, &y.Value)
				return nil
			}

			if err != nil {
				return fmt.Errorf("expected index in sequence, got %q", split[0])
			}

			// insert in the given index
			node.Content = append(node.Content[:idx], append([]*yaml.Node{&y.Value}, node.Content[idx:]...)...)
			return nil
		}

		if err != nil {
			return fmt.Errorf("expected index in sequence, got %q", split[0])
		}

		y.Path = strings.Join(split[1:], "/")
		return y.applyAdd(node.Content[idx])
	default:
		return fmt.Errorf("invalid node type %#v for path", node.Kind)
	}
}

func (y yamlPatch) applyRemove(node *yaml.Node) error {
	split := strings.Split(strings.TrimSpace(y.Path), "/")
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) != 1 {
			return fmt.Errorf("don't support multi-doc yaml")
		}

		y.Path = strings.Join(split[1:], "/")
		return y.applyRemove(node.Content[0])
	case yaml.MappingNode:
		if len(split) == 1 {
			// remove the final node
			for i := 0; i < len(node.Content); i += 2 {
				if node.Content[i].Value == split[0] {
					node.Content = append(node.Content[:i], node.Content[i+2:]...)
					return nil
				}
			}
		}

		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == split[0] {
				y.Path = strings.Join(split[1:], "/")
				return y.applyRemove(node.Content[i+1])
			}
		}

		return fmt.Errorf("key %q not found in map", split[0])
	case yaml.SequenceNode:
		idx, err := strconv.Atoi(split[0])
		if err != nil {
			return fmt.Errorf("expected index in sequence, got %q", split[0])
		}
		if idx > len(node.Content)-1 {
			return fmt.Errorf("invalid index %d for sequence of length %d", idx, len(node.Content))
		}

		if len(split) > 1 {
			y.Path = strings.Join(split[1:], "/")
			return y.applyRemove(node.Content[idx])
		}

		// remove sequence at index
		node.Content = append(node.Content[:idx], node.Content[idx+1:]...)
		return nil
	default:
		return nil
	}
}

func (y yamlPatch) applyReplace(node *yaml.Node) error {
	if y.Path == "" {
		return node.Encode(y.Value)
	}

	split := strings.Split(strings.TrimSpace(y.Path), "/")
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) != 1 {
			return fmt.Errorf("don't support multi-doc yaml")
		}

		y.Path = strings.Join(split[1:], "/")
		return y.applyReplace(node.Content[0])
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == split[0] {
				y.Path = strings.Join(split[1:], "/")
				return y.applyReplace(node.Content[i+1])
			}
		}

		return fmt.Errorf("key %q not found in map", split[0])
	case yaml.SequenceNode:
		idx, err := strconv.Atoi(split[0])
		if err != nil {
			return fmt.Errorf("expected index in sequence, got %q", split[0])
		}
		if idx > len(node.Content)-1 {
			return fmt.Errorf("invalid index %d for sequence of length %d", idx, len(node.Content))
		}

		y.Path = strings.Join(split[1:], "/")
		return y.applyReplace(node.Content[idx])
	default:
		return nil
	}
}

var errStopFollowingPointer = errors.New("done")

func followPointer(node *yaml.Node, pointer string, visit func(n *yaml.Node, pointer []string) error) error {
	split := strings.Split(pointer, "/")
	// replace each key

	return followPointerHelper(node, split, visit)
}

func followPointerHelper(node *yaml.Node, pointer []string, visit func(n *yaml.Node, pointer []string) error) error {
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

		return followPointerHelper(node, pointer[1:], visit)
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == pointer[0] {
				return followPointerHelper(node.Content[i+1], pointer[1:], visit)
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

		return followPointerHelper(node.Content[idx], pointer[1:], visit)
	default:
		return fmt.Errorf("invalid node type %#v for path", node.Kind)
	}
}
