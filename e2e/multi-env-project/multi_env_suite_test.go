package multi_env_app_test

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
var testEnvironmentProfile string
var prodEnvironmentProfile string

func TestMultiEnvProject(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Multiple Environments Suite")
}

var _ = BeforeSuite(func() {
	testEnvironmentProfile = "e2etestenv"
	prodEnvironmentProfile = "e2eprodenv"
	ecsCli, err := client.NewCLI()
	cli = ecsCli
	Expect(err).NotTo(HaveOccurred())
	projectName = fmt.Sprintf("e2e-multienv-%d", time.Now().Unix())
})

var _ = AfterSuite(func() {
	_, err := cli.ProjectDelete(map[string]string{"test": testEnvironmentProfile, "prod": prodEnvironmentProfile})
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
