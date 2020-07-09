package multi_svc_app_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var appName string

/**
The multi svc suite runs through several tests focusing on creating multiple
services in one app.
*/
func TestMultiSvcApp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Multiple Svc Suite (one workspace)")
}

var _ = BeforeSuite(func() {
	ecsCli, err := client.NewCLI()
	cli = ecsCli
	Expect(err).NotTo(HaveOccurred())
	appName = fmt.Sprintf("e2e-multisvc-%d", time.Now().Unix())
})

var _ = AfterSuite(func() {
	_, err := cli.AppDelete(map[string]string{"test": "default"})
	Expect(err).NotTo(HaveOccurred())
})

func BeforeAll(fn func()) {
	first := true
	BeforeEach(func() {
		if first {
			fn()
			first = false
		}
	})
}
