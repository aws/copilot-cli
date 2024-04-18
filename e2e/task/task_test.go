// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"fmt"
	"time"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Task", func() {
	Context("when creating a new app", Ordered, func() {
		var err error
		BeforeAll(func() {
			_, err = cli.AppInit(&client.AppInitRequest{
				AppName: appName,
			})
		})

		It("app init succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when adding a new environment", Ordered, func() {
		var (
			err error
		)
		BeforeAll(func() {
			_, err = cli.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: envName,
				Profile: envName,
			})
		})

		It("env init should succeed", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when deploying the environment", Ordered, func() {
		var envDeployErr error
		BeforeAll(func() {
			_, envDeployErr = cli.EnvDeploy(&client.EnvDeployRequest{
				AppName: appName,
				Name:    envName,
			})
		})

		It("should succeed", func() {
			Expect(envDeployErr).NotTo(HaveOccurred())
		})
	})

	Context("when running in an environment", Ordered, func() {
		var err error
		BeforeAll(func() {
			task := client.TaskRunInput{
				GroupName: fmt.Sprintf("e2e-task-%d", time.Now().Unix()),

				Dockerfile: "./backend/Dockerfile",

				AppName: appName,
				Env:     envName,

				EnvFile: "./sesame.env",
			}
			_, err = cli.TaskRun(&task)
			tasks = append(tasks, task)
		})

		It("should succeed", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when running in default cluster and subnets", Ordered, func() {
		var err error
		var taskLogs string
		BeforeAll(func() {
			task := client.TaskRunInput{
				GroupName: fmt.Sprintf("e2e-task-%d", time.Now().Unix()),

				Dockerfile: "./backend/Dockerfile",

				Default: true,
				Follow:  true,
			}
			taskLogs, err = cli.TaskRun(&task)
			tasks = append(tasks, task)
		})

		It("should succeed", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("task running", func() {
			Expect(taskLogs).To(ContainSubstring("e2e success: task running."))
		})
	})

	Context("when running in specific subnets and security groups", func() {
		It("should succeed", func() {
			Skip("Not implemented yet")
		})

		It("task running", func() {
			Skip("Test is not implemented yet")
		})
	})

	Context("when running with command and environment variables", Ordered, func() {
		var err error
		var taskLogs string
		BeforeAll(func() {
			task := client.TaskRunInput{
				GroupName: fmt.Sprintf("e2e-task-%d", time.Now().Unix()),

				Dockerfile: "./backend/Dockerfile",

				Command: "/bin/sh check_override.sh",
				EnvVars: "STATUS=OVERRIDDEN",

				Default: true,
				Follow:  true,
			}
			taskLogs, err = cli.TaskRun(&task)
			tasks = append(tasks, task)
		})

		It("should succeed", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("environment variables overridden", func() {
			Expect(taskLogs).To(ContainSubstring("e2e environment variables: OVERRIDDEN"))
		})
	})

	Context("when running with an image", func() {
		It("should succeed", func() {
			Skip("Not implemented yet")
		})

		It("task running", func() {
			Skip("Test is not implemented yet")
		})
	})
})
