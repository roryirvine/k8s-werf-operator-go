//go:build e2e
// +build e2e

/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/werf/k8s-werf-operator-go/test/utils"
)

// namespace where the project is deployed in
const namespace = "k8s-werf-operator-go-system"

// serviceAccountName created for the project
const serviceAccountName = "k8s-werf-operator-go-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "k8s-werf-operator-go-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "k8s-werf-operator-go-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should fail gracefully when ServiceAccount is missing", func() {
			By("creating a test namespace for the bundle")
			bundleNS := "werfbundle-test-1"
			cmd := exec.Command("kubectl", "create", "ns", bundleNS)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create test namespace")

			By("creating a WerfBundle with invalid ServiceAccount reference")
			werfBundleYAML := fmt.Sprintf(`
apiVersion: werf.io/v1alpha1
kind: WerfBundle
metadata:
  name: test-bundle-missing-sa
  namespace: %s
spec:
  registry:
    url: ghcr.io/werf/test-bundle
  converge:
    serviceAccountName: nonexistent-sa
`, bundleNS)

			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(werfBundleYAML)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create WerfBundle")

			By("verifying WerfBundle status is Failed due to missing ServiceAccount")
			verifyBundleFailed := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "werfbundle", "test-bundle-missing-sa", "-n", bundleNS,
					"-o", "jsonpath={.status.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Failed"), "Expected bundle status to be Failed")
			}
			Eventually(verifyBundleFailed, 30*time.Second).Should(Succeed())

			By("cleaning up test namespace")
			cmd = exec.Command("kubectl", "delete", "ns", bundleNS, "--wait=true")
			_, _ = utils.Run(cmd)
		})

		It("should handle invalid registry gracefully", func() {
			By("creating a test namespace for the bundle")
			bundleNS := "werfbundle-test-2"
			cmd := exec.Command("kubectl", "create", "ns", bundleNS)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create test namespace")

			By("creating ServiceAccount for werf converge jobs")
			saYAML := fmt.Sprintf(`
apiVersion: v1
kind: ServiceAccount
metadata:
  name: werf-converge
  namespace: %s
`, bundleNS)

			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(saYAML)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ServiceAccount")

			By("creating a WerfBundle pointing to nonexistent registry")
			werfBundleYAML := fmt.Sprintf(`
apiVersion: werf.io/v1alpha1
kind: WerfBundle
metadata:
  name: test-bundle-invalid-registry
  namespace: %s
spec:
  registry:
    url: ghcr.io/nonexistent/bundle-that-does-not-exist
  converge:
    serviceAccountName: werf-converge
`, bundleNS)

			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(werfBundleYAML)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create WerfBundle")

			By("verifying that a Job was created")
			verifyJobCreated := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "jobs", "-n", bundleNS,
					"-l", "app.kubernetes.io/instance=test-bundle-invalid-registry",
					"-o", "jsonpath={.items[*].metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(), "Expected at least one Job to be created")
			}
			Eventually(verifyJobCreated, 30*time.Second).Should(Succeed())

			By("cleaning up test namespace")
			cmd = exec.Command("kubectl", "delete", "ns", bundleNS, "--wait=true")
			_, _ = utils.Run(cmd)
		})

		It("should garbage collect Job when WerfBundle is deleted", func() {
			By("creating a test namespace for the bundle")
			bundleNS := "werfbundle-test-3"
			cmd := exec.Command("kubectl", "create", "ns", bundleNS)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create test namespace")

			By("creating ServiceAccount for werf converge jobs")
			saYAML := fmt.Sprintf(`
apiVersion: v1
kind: ServiceAccount
metadata:
  name: werf-converge
  namespace: %s
`, bundleNS)

			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(saYAML)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ServiceAccount")

			By("creating a WerfBundle")
			werfBundleYAML := fmt.Sprintf(`
apiVersion: werf.io/v1alpha1
kind: WerfBundle
metadata:
  name: test-bundle-cleanup
  namespace: %s
spec:
  registry:
    url: ghcr.io/werf/test-bundle
  converge:
    serviceAccountName: werf-converge
`, bundleNS)

			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(werfBundleYAML)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create WerfBundle")

			By("verifying that a Job was created")
			var jobName string
			verifyJobCreated := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "jobs", "-n", bundleNS,
					"-l", "app.kubernetes.io/instance=test-bundle-cleanup",
					"-o", "jsonpath={.items[0].metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(), "Expected Job to be created")
				jobName = output
			}
			Eventually(verifyJobCreated, 30*time.Second).Should(Succeed())

			By("deleting the WerfBundle")
			cmd = exec.Command("kubectl", "delete", "werfbundle", "test-bundle-cleanup", "-n", bundleNS)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete WerfBundle")

			By("verifying that the Job was garbage collected")
			verifyJobDeleted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "job", jobName, "-n", bundleNS)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred(), "Expected Job to be deleted")
			}
			Eventually(verifyJobDeleted, 30*time.Second).Should(Succeed())

			By("cleaning up test namespace")
			cmd = exec.Command("kubectl", "delete", "ns", bundleNS, "--wait=true")
			_, _ = utils.Run(cmd)
		})
	})
})

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() (string, error) {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	return utils.Run(cmd)
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
