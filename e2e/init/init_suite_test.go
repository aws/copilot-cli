package init_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/e2e/internal/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var projectName string

/**
The Init Suite runs through the ecs-preview init workflow for a brand new
project. It creates a single environment, deploys an app to it, and then
tears it down.
*/
func TestInit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Init Suite")
}

var _ = BeforeSuite(func() {
	ecsCli, err := client.NewCLI()
	cli = ecsCli
	Expect(err).NotTo(HaveOccurred())
	projectName = fmt.Sprintf("e2e-init-%d", time.Now().Unix())
})

var _ = AfterSuite(func() {
	_, err := cli.AppDelete("api")
	Expect(err).NotTo(HaveOccurred())

	_, err = cli.EnvDelete("test", "default")
	Expect(err).NotTo(HaveOccurred())

	_, err = cli.ProjectDelete(map[string]string{})
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
