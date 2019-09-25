// +build e2e

package cli_test

import (
	"flag"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestHelpMessages(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(time.Now().UnixNano())
	RunSpecs(t, "Test help messages displayed when running various archer commands")
}

var cliPath string
var update = flag.Bool("update", false, "update .golden files")

var _ = BeforeSuite(func() {
	var err error
	cliPath, err = filepath.Abs("../../bin/local/archer")
	Expect(err).To(BeNil())

	// ensure the CLI is available to e2e tests
	_, err = os.Stat(cliPath)
	Expect(err).To(BeNil())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
