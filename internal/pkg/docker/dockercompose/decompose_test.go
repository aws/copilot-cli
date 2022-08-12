package dockercompose

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCompose_ValidateKeys(t *testing.T) {
	path := filepath.Join("testdata", "unsupported-keys.yml")
	cfg, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("read file %s: %s\n", path, err.Error())
		return
	}

	// TODO: Actual test case
	_, _, err = decomposeService(cfg, "test")
	if err != nil {
		fmt.Printf("decompose service: %s\n", err.Error())
	}

	fmt.Println("Done")
}
