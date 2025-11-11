// Package converge provides log capture utilities for werf converge jobs.
package converge

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CaptureJobLogs retrieves logs from pods associated with a job.
// Returns the last lines of logs from the job's containers (up to ~5KB).
// Uses clientset to access pod logs via the API server.
func CaptureJobLogs(ctx context.Context, c client.Client, clientset kubernetes.Interface, jobName string, namespace string) (string, error) {
	// List pods with the job label
	pods := &corev1.PodList{}
	selector := client.MatchingLabels{"batch.kubernetes.io/job-name": jobName}
	if err := c.List(ctx, pods, client.InNamespace(namespace), selector); err != nil {
		return "", fmt.Errorf("failed to list pods for job: %w", err)
	}

	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for job %s", jobName)
	}

	// Collect logs from all pods and containers
	var allLogs []string

	for _, pod := range pods.Items {
		// Try to get logs from main container (usually "werf")
		if len(pod.Spec.Containers) > 0 {
			logs, err := getPodLogs(ctx, clientset, pod, pod.Spec.Containers[0].Name, namespace)
			if err == nil && logs != "" {
				allLogs = append(allLogs, logs)
			}
		}
	}

	if len(allLogs) == 0 {
		return "", fmt.Errorf("no logs found for job %s", jobName)
	}

	// Combine logs from all pods
	combined := strings.Join(allLogs, "\n---\n")

	// Trim to reasonable size (5KB for Status field storage)
	if len(combined) > 5120 {
		// Keep the last 5KB (most recent logs)
		return combined[len(combined)-5120:], nil
	}

	return combined, nil
}

// getPodLogs retrieves logs from a pod's container using the Kubernetes clientset.
// Returns up to 500 lines of logs.
func getPodLogs(ctx context.Context, clientset kubernetes.Interface, pod corev1.Pod, container string, namespace string) (string, error) {
	// Determine tail lines: up to 500 lines
	tailLines := int64(500)

	// Get logs from the pod
	req := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: container,
		TailLines: &tailLines,
	})

	// Read the response
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get logs from pod %s/%s: %w", namespace, pod.Name, err)
	}
	defer stream.Close()

	// Read logs into a buffer
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, stream); err != nil {
		return "", fmt.Errorf("failed to read logs from pod %s/%s: %w", namespace, pod.Name, err)
	}

	return buf.String(), nil
}
