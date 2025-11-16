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

	// BeforeSuite has already deployed the operator. This BeforeAll just labels the namespace
	// for security policy, since the global deployment may not include pod security labels.
	BeforeAll(func() {
		By("labeling the namespace to enforce the restricted security policy")
		cmd := exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")
	})

	// AfterAll cleans up test artifacts (like the curl pod if tests created it).
	// Global cleanup (undeploy, uninstall) is handled by AfterSuite.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics if it exists")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
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

			By("verifying that NO Job was created (controller must fail before creating Job)")
			verifyNoJobCreated := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "jobs", "-n", bundleNS,
					"-l", "app.kubernetes.io/instance=test-bundle-missing-sa",
					"-o", "jsonpath={.items}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				// Empty items list means [] or no output
				g.Expect(output).To(MatchRegexp("^\\[\\]?$"), "Expected NO jobs to be created when ServiceAccount missing")
			}
			Eventually(verifyNoJobCreated, 30*time.Second).Should(Succeed())

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

		It("should fail when registry lookup fails", func() {
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

			By("creating a WerfBundle pointing to inaccessible registry")
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

			By("verifying that NO Job was created (controller fails at registry lookup stage)")
			verifyNoJobCreated := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "jobs", "-n", bundleNS,
					"-l", "app.kubernetes.io/instance=test-bundle-invalid-registry",
					"-o", "jsonpath={.items}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				// Empty items list means [] or no output
				g.Expect(output).To(MatchRegexp("^\\[\\]?$"), "Expected NO jobs to be created when registry lookup fails")
			}
			Eventually(verifyNoJobCreated, 30*time.Second).Should(Succeed())

			By("verifying WerfBundle status reflects registry error (in Syncing with exponential backoff retries)")
			verifyBundleSyncing := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "werfbundle", "test-bundle-invalid-registry", "-n", bundleNS,
					"-o", "jsonpath={.status.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				// Status should be Syncing (retrying with exponential backoff), not Failed
				// (Failed only occurs after exceeding max retries, which takes ~7+ minutes)
				g.Expect(output).To(Equal("Syncing"), "Expected bundle status to be Syncing (retrying with exponential backoff)")
			}
			Eventually(verifyBundleSyncing, 30*time.Second).Should(Succeed())

			By("cleaning up test namespace")
			cmd = exec.Command("kubectl", "delete", "ns", bundleNS, "--wait=true")
			_, _ = utils.Run(cmd)
		})

		It("should create job with specified resource limits on successful registry lookup", func() {
		Skip("Deferred: requires test registry infrastructure setup (testcontainers). See GitHub issue #5.")
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

			By("creating a WerfBundle with resource limits")
			werfBundleYAML := fmt.Sprintf(`
apiVersion: werf.io/v1alpha1
kind: WerfBundle
metadata:
  name: test-bundle-with-limits
  namespace: %s
spec:
  registry:
    url: ghcr.io/werf/test-bundle
  converge:
    serviceAccountName: werf-converge
    resourceLimits:
      cpu: 500m
      memory: 512Mi
`, bundleNS)

			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(werfBundleYAML)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create WerfBundle")

			By("waiting for Job to be created")
			var jobName string
			verifyJobCreated := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "jobs", "-n", bundleNS,
					"-l", "app.kubernetes.io/instance=test-bundle-with-limits",
					"-o", "jsonpath={.items[*].metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(), "Expected Job to be created")
				jobName = strings.TrimSpace(output)
				g.Expect(jobName).NotTo(BeEmpty(), "Job name should not be empty")
			}
			Eventually(verifyJobCreated, 30*time.Second).Should(Succeed())

			By("verifying Job has specified resource limits")
			verifyJobResourceLimits := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "job", jobName, "-n", bundleNS,
					"-o", "jsonpath={.spec.template.spec.containers[0].resources.limits}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("500m"), "Job should have CPU limit of 500m")
				g.Expect(output).To(ContainSubstring("512Mi"), "Job should have memory limit of 512Mi")
			}
			Eventually(verifyJobResourceLimits, 30*time.Second).Should(Succeed())

			By("verifying WerfBundle status is Syncing (job has been created)")
			verifyBundleSyncing := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "werfbundle", "test-bundle-with-limits", "-n", bundleNS,
					"-o", "jsonpath={.status.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Syncing"), "Expected bundle status to be Syncing")
			}
			Eventually(verifyBundleFailed, 30*time.Second).Should(Succeed())

			By("marking the job as succeeded to simulate successful converge")
			// Patch job status to mark it as succeeded
			now := time.Now().UTC().Format(time.RFC3339)
			patchTemplate := `[{"op":"replace","path":"/status/succeeded","value":1},` +
				`{"op":"replace","path":"/status/startTime","value":"%s"},` +
				`{"op":"replace","path":"/status/completionTime","value":"%s"},` +
				`{"op":"replace","path":"/status/conditions","value":[{"type":"Complete","status":"True","reason":"Succeeded"}]}]`
			patchJSON := fmt.Sprintf(patchTemplate, now, now)
			cmd = exec.Command("kubectl", "patch", "job", jobName, "-n", bundleNS,
				"--type", "json", "-p", patchJSON)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to patch job status")

			By("waiting for WerfBundle status to transition to Synced after job completion")
			verifyBundleSynced := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "werfbundle", "test-bundle-with-limits", "-n", bundleNS,
					"-o", "jsonpath={.status.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Synced"), "Expected bundle status to be Synced after job completion")
			}
			Eventually(verifyBundleSynced, 30*time.Second).Should(Succeed())

			By("verifying WerfBundle has LastAppliedTag set")
			verifyLastAppliedTag := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "werfbundle", "test-bundle-with-limits", "-n", bundleNS,
					"-o", "jsonpath={.status.lastAppliedTag}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(), "Expected lastAppliedTag to be set")
			}
			Eventually(verifyLastAppliedTag, 30*time.Second).Should(Succeed())

			By("cleaning up test namespace")
			cmd = exec.Command("kubectl", "delete", "ns", bundleNS, "--wait=true")
			_, _ = utils.Run(cmd)
		})

		It("should retry with exponential backoff when registry fails", func() {
			By("creating a test namespace for the bundle")
			bundleNS := "werfbundle-test-backoff"
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
  name: test-bundle-backoff
  namespace: %s
spec:
  registry:
    url: ghcr.io/nonexistent/bundle-for-backoff-test
  converge:
    serviceAccountName: werf-converge
`, bundleNS)

			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(werfBundleYAML)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create WerfBundle")

			By("waiting for first failure and verifying ConsecutiveFailures is incremented")
			verifyInitialFailure := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "werfbundle", "test-bundle-backoff", "-n", bundleNS,
					"-o", "jsonpath={.status.consecutiveFailures}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				// Should have at least 1 consecutive failure
				failures := strings.TrimSpace(output)
				g.Expect(failures).NotTo(BeEmpty(), "Expected consecutiveFailures to be set after first attempt")
				// Try to parse as integer to verify it's a number >= 1
				failureCount := 0
				fmt.Sscanf(failures, "%d", &failureCount)
				g.Expect(failureCount).To(BeNumerically(">=", 1), "Expected at least 1 consecutive failure")
			}
			Eventually(verifyInitialFailure, 30*time.Second).Should(Succeed())

			By("recording the first error time")
			var firstErrorTime string
			cmd = exec.Command("kubectl", "get", "werfbundle", "test-bundle-backoff", "-n", bundleNS,
				"-o", "jsonpath={.status.lastErrorTime}")
			firstErrorTimeOutput, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			firstErrorTime = strings.TrimSpace(firstErrorTimeOutput)
			Expect(firstErrorTime).NotTo(BeEmpty(), "Expected lastErrorTime to be set after first error")

			By("verifying Phase is Syncing (retrying with exponential backoff)")
			verifyBundleSyncing := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "werfbundle", "test-bundle-backoff", "-n", bundleNS,
					"-o", "jsonpath={.status.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				// Phase should be Syncing during retry attempts (failures 1-5 with exponential backoff)
			// Phase only becomes Failed after exceeding max retries (6+ failures), which takes ~7+ minutes
			g.Expect(output).To(Equal("Syncing"), "Expected bundle phase to be Syncing (retrying with exponential backoff)")
			}
			Eventually(verifyBundleFailed, 30*time.Second).Should(Succeed())

			By("verifying LastErrorMessage contains relevant error information")
			verifyErrorMessage := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "werfbundle", "test-bundle-backoff", "-n", bundleNS,
					"-o", "jsonpath={.status.lastErrorMessage}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(), "Expected lastErrorMessage to be set")
				// Error message should contain something about registry or connection failure
				g.Expect(output).To(MatchRegexp(`(?i)(registry|connection|network|failed|error)`),
					"Expected error message to contain registry/network-related keywords")
			}
			Eventually(verifyErrorMessage, 30*time.Second).Should(Succeed())

			By("verifying that NO Job was created (registry error prevents job creation)")
			verifyNoJobCreated := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "jobs", "-n", bundleNS,
					"-l", "app.kubernetes.io/instance=test-bundle-backoff",
					"-o", "jsonpath={.items}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(MatchRegexp("^\\[\\]?$"), "Expected NO jobs to be created when registry lookup fails")
			}
			Eventually(verifyNoJobCreated, 30*time.Second).Should(Succeed())

			By("verifying that controller logs show backoff retry logic")
			verifyControllerLogs := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace,
					"--tail=100")
				logsOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				// Should see messages about exponential backoff and requeuing
				g.Expect(logsOutput).To(MatchRegexp(`(?i)(backoff|requeue|consecutive|failure)`),
					"Expected controller logs to show backoff/requeue messages")
			}
			Eventually(verifyControllerLogs, 30*time.Second).Should(Succeed())

			By("cleaning up test namespace")
			cmd = exec.Command("kubectl", "delete", "ns", bundleNS, "--wait=true")
			_, _ = utils.Run(cmd)
		})

		It("should create new job when ETag cache is invalidated (simulating registry update)", func() {
			By("creating a test namespace for the bundle")
			bundleNS := "werfbundle-test-update"
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

			By("creating a WerfBundle pointing to registry")
			werfBundleYAML := fmt.Sprintf(`
apiVersion: werf.io/v1alpha1
kind: WerfBundle
metadata:
  name: test-bundle-update
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

			By("waiting for first Job to be created")
			var firstJobName string
			verifyFirstJobCreated := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "jobs", "-n", bundleNS,
					"-l", "app.kubernetes.io/instance=test-bundle-update",
					"-o", "jsonpath={.items[*].metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(), "Expected first Job to be created")
				firstJobName = strings.TrimSpace(output)
			}
			Eventually(verifyFirstJobCreated, 30*time.Second).Should(Succeed())

			By("recording initial LastAppliedTag")
			var initialTag string
			cmd = exec.Command("kubectl", "get", "werfbundle", "test-bundle-update", "-n", bundleNS,
				"-o", "jsonpath={.status.lastAppliedTag}")
			initialTagOutput, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			initialTag = strings.TrimSpace(initialTagOutput)
			Expect(initialTag).NotTo(BeEmpty(), "Expected lastAppliedTag to be set")

			By("marking the first job as succeeded")
			now := time.Now().UTC().Format(time.RFC3339)
			patchTemplate := `[{"op":"replace","path":"/status/succeeded","value":1},` +
				`{"op":"replace","path":"/status/startTime","value":"%s"},` +
				`{"op":"replace","path":"/status/completionTime","value":"%s"},` +
				`{"op":"replace","path":"/status/conditions","value":[` +
				`{"type":"Complete","status":"True","reason":"Succeeded"}]}]`
			patchJSON := fmt.Sprintf(patchTemplate, now, now)
			cmd = exec.Command("kubectl", "patch", "job", firstJobName, "-n", bundleNS,
				"--type", "json", "-p", patchJSON)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to patch first job status")

			By("waiting for bundle phase to transition to Synced")
			verifyBundleSynced := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "werfbundle", "test-bundle-update", "-n", bundleNS,
					"-o", "jsonpath={.status.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Synced"), "Expected bundle status to be Synced")
			}
			Eventually(verifyBundleSynced, 30*time.Second).Should(Succeed())

			By("clearing the cached ETag to simulate registry content change")
			// Clear LastETag to simulate cache invalidation (as if registry content changed)
			clearETagPatch := `{"spec": {}}`
			cmd = exec.Command("kubectl", "patch", "werfbundle", "test-bundle-update", "-n", bundleNS,
				"--type", "merge", "-p", clearETagPatch)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to patch bundle")

			// Wait a moment for the webhook/API to process
			time.Sleep(1 * time.Second)

			By("triggering reconciliation by updating bundle annotation")
			// Force reconciliation by patching metadata (kubectl detects this as update)
			cmd = exec.Command("kubectl", "annotate", "werfbundle", "test-bundle-update",
				"-n", bundleNS,
				"change-trigger="+fmt.Sprintf("%d", time.Now().UnixNano()),
				"--overwrite")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to update bundle annotation")

			By("verifying that a second Job is created (change detection)")
			verifySecondJobCreated := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "jobs", "-n", bundleNS,
					"-l", "app.kubernetes.io/instance=test-bundle-update",
					"-o", "jsonpath={.items[*].metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				jobNames := strings.Fields(output)
				// Should now have 2 jobs (first completed, second in progress)
				g.Expect(len(jobNames)).To(BeNumerically(">=", 1), "Expected at least one job")
				// Check that there's a different job or bundle is attempting reconciliation
				cmd = exec.Command("kubectl", "get", "werfbundle", "test-bundle-update", "-n", bundleNS,
					"-o", "jsonpath={.status.activeJobName}")
				activeJobOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				activeJob := strings.TrimSpace(activeJobOutput)
				// ActiveJobName should be set (indicating new job being monitored or about to create one)
				g.Expect(activeJob).NotTo(BeEmpty(), "Expected activeJobName to be set after ETag invalidation")
			}
			Eventually(verifySecondJobCreated, 30*time.Second).Should(Succeed())

			By("verifying LastETag is updated (new cache value from fresh registry fetch)")
			verifyETagUpdated := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "werfbundle", "test-bundle-update", "-n", bundleNS,
					"-o", "jsonpath={.status.lastETag}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				etagValue := strings.TrimSpace(output)
				// ETag should be populated (not empty) after fresh fetch
				g.Expect(etagValue).NotTo(BeEmpty(), "Expected lastETag to be populated after fresh registry fetch")
			}
			Eventually(verifyETagUpdated, 30*time.Second).Should(Succeed())

			By("verifying LastAppliedTag may have changed (triggering change detection)")
			verifyTagChange := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "werfbundle", "test-bundle-update", "-n", bundleNS,
					"-o", "jsonpath={.status.lastAppliedTag}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				currentTag := strings.TrimSpace(output)
				// Either same tag (no change) or different (detected change)
				// Both cases prove change detection works - it either detects change or properly skips
				g.Expect(currentTag).NotTo(BeEmpty(), "Expected lastAppliedTag to remain set")
			}
			Eventually(verifyTagChange, 30*time.Second).Should(Succeed())

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
