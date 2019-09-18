// +build e2e

package cli_test

import (
	"math/rand"
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

var _ = BeforeSuite(func() {
	// ensure the e2e tests are performed on the latest code changes by
	// compiling CLI from source
	var err error
	cliPath, err = gexec.Build("../../cmd/archer/main.go")
	Expect(err).Should(BeNil())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
