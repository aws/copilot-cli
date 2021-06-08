// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package pipeline_test

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/copilot-cli/e2e/internal/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("pipeline flow", func() {
	Context("setup CodeCommit repository", func() {
		It("creates the codecommit repository", func() {
			url, err := aws.CreateCodeCommitRepo(repoName)
			Expect(err).NotTo(HaveOccurred())
			repoURL = url
		})

		It("clones the repository", func() {
			endpoint := strings.TrimPrefix(repoURL, "https://")
			url := fmt.Sprintf("https://%s:%s@%s", url.PathEscape(codeCommitCreds.UserName), url.PathEscape(codeCommitCreds.Password), endpoint)

			Eventually(func() error {
				cmd := exec.Command("git", "clone", url)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				return cmd.Run()
			}, "60s", "5s").ShouldNot(HaveOccurred())
		})

		It("copies source code to the git repository", func() {
			cmd := exec.Command("cp", "-r", "frontend", repoName)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			Expect(cmd.Run()).NotTo(HaveOccurred())
		})

		It("should push upstream", func() {
			cmd := exec.Command("git", "add", ".")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Dir = repoName
			Expect(cmd.Run()).NotTo(HaveOccurred())

			cmd = exec.Command("git", "commit", "-m", "first commit")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Dir = repoName
			Expect(cmd.Run()).NotTo(HaveOccurred())

			cmd = exec.Command("git", "push")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Dir = repoName
			Expect(cmd.Run()).NotTo(HaveOccurred())
		})
	})

	Context("create a new app", func() {
		It("app init succeeds", func() {
			_, err := copilot.AppInit(&client.AppInitRequest{
				AppName: appName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
		It("app init creates an copilot directory and workspace file", func() {
			Expect(filepath.Join(repoName, "copilot")).Should(BeADirectory())
			Expect(filepath.Join(repoName, "copilot", ".workspace")).Should(BeAnExistingFile())
		})
		It("app ls includes new app", func() {
			Eventually(copilot.AppList, "30s", "5s").Should(ContainSubstring(appName))
		})
		It("app show includes app name", func() {
			appShowOutput, err := copilot.AppShow(appName)
			Expect(err).NotTo(HaveOccurred())
			Expect(appShowOutput.Name).To(Equal(appName))
			Expect(appShowOutput.URI).To(BeEmpty())
		})
	})

	Context("when creating a new environment", func() {
		It("test env init should succeed", func() {
			_, err := copilot.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: "test",
				Profile: "e2etestenv",
				Prod:    false,
			})
			Expect(err).NotTo(HaveOccurred())
		})
		It("prod env init should succeed", func() {
			_, err := copilot.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: "prod",
				Profile: "e2eprodenv",
				Prod:    false,
			})
			Expect(err).NotTo(HaveOccurred())
		})
		It("env ls should list both envs", func() {
			out, err := copilot.EnvList(appName)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(out.Envs)).To(Equal(2))
			envs := map[string]client.EnvDescription{}
			for _, env := range out.Envs {
				envs[env.Name] = env
				Expect(env.ExecutionRole).NotTo(BeEmpty())
				Expect(env.ManagerRole).NotTo(BeEmpty())
			}

			Expect(envs["test"]).NotTo(BeNil())
			Expect(envs["prod"]).NotTo(BeNil())
		})
	})

	Context("when creating the frontend service", func() {
		It("should initialize the service", func() {
			_, err := copilot.SvcInit(&client.SvcInitRequest{
				Name:       "frontend",
				SvcType:    "Load Balanced Web Service",
				Dockerfile: "./frontend/Dockerfile",
				SvcPort:    "80",
			})
			Expect(err).NotTo(HaveOccurred())
		})
		It("should generate a manifest file", func() {
			Expect(filepath.Join(repoName, "copilot", "frontend", "manifest.yml")).Should(BeAnExistingFile())
		})
		It("should list the service", func() {
			out, err := copilot.SvcList(appName)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(out.Services)).To(Equal(1))
			Expect(out.Services[0].Name).To(Equal("frontend"))
		})
	})

	Context("when creating the pipeline manifest", func() {
		It("should initialize the pipeline", func() {
			_, err := copilot.PipelineInit(appName, repoURL, "master", []string{"test", "prod"})
			Expect(err).NotTo(HaveOccurred())
		})
		It("should generate pipeline artifacts", func() {
			Expect(filepath.Join(repoName, "copilot", "pipeline.yml")).Should(BeAnExistingFile())
			Expect(filepath.Join(repoName, "copilot", "buildspec.yml")).Should(BeAnExistingFile())
		})
	})
})
