package import_certs

import (
	"net/http"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/copilot-cli/e2e/internal/client"
)

var _ = Describe("Import Certificate", func() {

	Context("when creating a new app", func() {
		var appInitErr error

		BeforeAll(func() {
			_, appInitErr = cli.AppInit(&client.AppInitRequest{
				AppName: appName,
			})
		})

		It("app init succeeds", func() {
			Expect(appInitErr).NotTo(HaveOccurred())
		})

		It("app init creates a copilot directory", func() {
			Expect("./copilot").Should(BeADirectory())
		})

		It("app ls includes new application", func() {
			Eventually(cli.AppList, "30s", "5s").Should(ContainSubstring(appName))
		})
	})

	Context("when adding new environments", func() {
		fatalErrors := make(chan error)
		wgDone := make(chan bool)
		It("env init should succeed for adding test environment", func() {
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					content, err := cli.EnvInit(&client.EnvInitRequest{
						AppName:           appName,
						EnvName:           "test",
						Profile:           testEnvironmentProfile,
						Prod:              false,
						CertificateImport: "arn:aws:acm:us-west-2:323664494501:certificate/a6a4fffb-b498-4190-b5b2-7c2dff4e8d39",
					})
					if err == nil {
						break
					}
					if !isStackSetOperationInProgress(content) {
						fatalErrors <- err
					}
					time.Sleep(waitingInterval)
				}
			}()
			go func() {
				wg.Wait()
				close(wgDone)
				close(fatalErrors)
			}()

			select {
			case <-wgDone:
			case err := <-fatalErrors:
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	Context("when deploying the environments", func() {
		fatalErrors := make(chan error)
		wgDone := make(chan bool)
		It("env deploy should succeed for deploying test environment", func() {
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := cli.EnvDeploy(&client.EnvDeployRequest{
					AppName: appName,
					Name:    "test",
				})
				if err != nil {
					fatalErrors <- err
				}
			}()
			go func() {
				wg.Wait()
				close(wgDone)
				close(fatalErrors)
			}()

			select {
			case <-wgDone:
			case err := <-fatalErrors:
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	Context("when initializing Load Balanced Web Services", func() {
		var svcInitErr error

		It("svc init should succeed for creating the frontend service", func() {
			_, svcInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       "frontend",
				SvcType:    "Load Balanced Web Service",
				Dockerfile: "./src/Dockerfile",
				SvcPort:    "80",
			})
			Expect(svcInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when deploying a Load Balanced Web Service", func() {
		It("deployments should succeed", func() {
			fatalErrors := make(chan error)
			wgDone := make(chan bool)
			var wg sync.WaitGroup
			wg.Add(1)
			// deploy frontend to test.
			go func() {
				defer wg.Done()
				for {
					content, err := cli.SvcDeploy(&client.SvcDeployInput{
						Name:     "frontend",
						EnvName:  "test",
						ImageTag: "frontend",
					})
					if err == nil {
						break
					}
					if !isStackSetOperationInProgress(content) && !isImagePushingToECRInProgress(content) {
						fatalErrors <- err
					}
					time.Sleep(waitingInterval)
				}
			}()
			go func() {
				wg.Wait()
				close(wgDone)
				close(fatalErrors)
			}()

			select {
			case <-wgDone:
			case err := <-fatalErrors:
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("svc show should contain the expected domain and the request should succeed", func() {
			// Check frontend service
			svc, err := cli.SvcShow(&client.SvcShowRequest{
				Name:    "frontend",
				AppName: appName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(1))

			wantedURLs := map[string]string{
				"test": "https://test.copilot-e2e-tests.ecs.aws.dev",
			}
			for _, route := range svc.Routes {
				// Validate route has the expected HTTPS endpoint.
				Expect(route.URL).To(Equal(wantedURLs[route.Environment]))

				// Make sure the response is OK.
				var resp *http.Response
				var fetchErr error
				Eventually(func() (int, error) {
					resp, fetchErr = http.Get(route.URL)
					return resp.StatusCode, fetchErr
				}, "60s", "1s").Should(Equal(200))
				// HTTP should route to HTTPS.
				Eventually(func() (int, error) {
					resp, fetchErr = http.Get(strings.Replace(route.URL, "https", "http", 1))
					return resp.StatusCode, fetchErr
				}, "60s", "1s").Should(Equal(200))
			}
		})
	})
})
