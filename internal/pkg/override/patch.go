package override

import (
	"fmt"
	"path/filepath"
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
		node, err := getNode(&root, patch.Path)
		if err != nil {
			return nil, fmt.Errorf("unable to apply patch with operation %q at %q: %w", patch.Operation, patch.Path, err)
		}

		fmt.Printf("before apply: %+v\n", nodeString(node))
		if err := patch.apply(node); err != nil {
			return nil, fmt.Errorf("unable to apply patch with operation %q at %q: %w", patch.Operation, patch.Path, err)
		}

		fmt.Printf("after apply: %+v\n", nodeString(node))
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

func getNode(node *yaml.Node, path string) (*yaml.Node, error) {
	fmt.Printf("getNode(%+v)\n", node)
	// follow the JSON pointer pointer down to the node path.
	// TODO stop at the parent of the final node
	// fix pointer syntax: https://www.rfc-editor.org/rfc/rfc6901#section-3
	// TODO figure out how to handle the path being "/" " ".
	split := strings.Split(strings.TrimSpace(path), "/")

	switch node.Kind {
	case yaml.DocumentNode:
		if path == "" {
			return node, nil
		}

		if len(node.Content) != 1 {
			return nil, fmt.Errorf("don't support multi-doc yaml")
		}

		return getNode(node.Content[0], strings.Join(split[1:], "/"))
	case yaml.MappingNode:
		// base case, this is the node
		if path == "" {
			return node, nil
		}

		// find the requested node
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == split[0] {
				fmt.Printf("calling getNode() on %v\n", node.Content[i].Value)
				return getNode(node.Content[i+1], strings.Join(split[1:], "/"))
			}
		}

		return nil, fmt.Errorf("bad")
	case yaml.SequenceNode:
		// TODO: "-" case
	default:
		// error
		return nil, fmt.Errorf("bad kind: %#v", node.Kind)
	}

	return nil, nil
}

func (y yamlPatch) apply(node *yaml.Node) error {
	fmt.Printf("value: %+v\n", &y.Value)
	fmt.Printf("value: %+v\n", nodeString(&y.Value))
	switch y.Operation {
	case "add":
		node.Content = append(node.Content, y.Value.Content...)
	}

	return nil
}
