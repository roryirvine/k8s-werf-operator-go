# Werf Operator for Kubernetes

A Kubernetes operator for deploying and managing applications packaged with [Werf](https://werf.io/) using declarative bundle definitions.

## Current Status

**Slice 2 - Enhanced Registry Integration** (Alpha)

This is early-stage software (v1alpha1 API). The operator supports bundle deployment with robust registry polling and configurable resource management.

### What works now

- Watch for `WerfBundle` custom resources in the cluster
- Poll OCI registries for available bundle tags
- Robust registry polling with ETag caching and exponential backoff for reliability
- Create Kubernetes Jobs to run `werf converge` deployments with configurable resource limits
- Track deployment status in the WerfBundle resource
- Proper RBAC separation (operator minimal, job permissions namespace-scoped)
- Capture and retain deployment job logs for troubleshooting

**What does NOT work yet:**
- Semantic versioning (tags are sorted lexicographically, not by semver)
- Advanced registry authentication (access tokens only, no username/password)
- Cross-namespace deployments
- Drift detection
- Helm integration
- Custom value overrides

See [PLAN.md](docs/PLAN.md) for the full roadmap including Slice 3+.

## Quick Start

### Prerequisites

- Kubernetes 1.24+ cluster
- `kubectl` configured to access your cluster
- Access to an OCI registry with Werf bundles

### Install the Operator

1. **Install CRDs:**
   ```bash
   make install
   ```

2. **Deploy the operator:**
   ```bash
   make deploy IMG=ghcr.io/werf/k8s-werf-operator-go:v0.0.1
   ```

   Note: Replace the image with your own build or use the pre-built image if available.

3. **Verify deployment:**
   ```bash
   kubectl get pods -n k8s-werf-operator-go-system
   kubectl get crds | grep werfbundle
   ```

### Create a WerfBundle

1. **Set up target namespace with ServiceAccount:**

   In the namespace where you want werf to deploy applications, create a ServiceAccount with appropriate permissions:

   ```bash
   kubectl create namespace production

   kubectl apply -f - <<EOF
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: werf-converge
     namespace: production
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: Role
   metadata:
     name: werf-converge
     namespace: production
   rules:
   - apiGroups: ["*"]
     resources: ["*"]
     verbs: ["*"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: RoleBinding
   metadata:
     name: werf-converge
     namespace: production
   roleRef:
     apiGroup: rbac.authorization.k8s.io
     kind: Role
     name: werf-converge
   subjects:
   - kind: ServiceAccount
     name: werf-converge
     namespace: production
   EOF
   ```

2. **Create WerfBundle resource:**

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: werf.io/v1alpha1
   kind: WerfBundle
   metadata:
     name: my-app
     namespace: k8s-werf-operator-go-system
   spec:
     registry:
       url: ghcr.io/org/my-app-bundle
       pollInterval: 15m
     converge:
       serviceAccountName: werf-converge
       resourceLimits:
         cpu: "2"
         memory: "2Gi"
   EOF
   ```

3. **Check status:**

   ```bash
   kubectl get werfbundle -A
   kubectl describe werfbundle my-app -n k8s-werf-operator-go-system
   kubectl logs -n k8s-werf-operator-go-system -l control-plane=controller-manager
   ```

## Reliability Features

The operator includes several built-in features to ensure robust registry polling and reliable deployment:

### ETag Caching

The operator uses HTTP ETag caching to minimize registry requests and save bandwidth. When polling for new bundle versions:
- First poll requests the full tag list and stores the ETag response header
- Subsequent polls include the ETag in the `If-None-Match` header
- If content hasn't changed, the registry responds with HTTP 304 (Not Modified), saving bandwidth
- This is particularly valuable for large registries or frequent polling intervals

### Exponential Backoff

When registry polling fails (network errors, timeouts, server errors), the operator automatically retries with exponential backoff:
- Retry attempts: Up to 5 consecutive failures before marking the bundle as failed
- Backoff sequence: 30s → 1m → 2m → 4m → 8m
- Each failure increments a counter in the WerfBundle status (`status.consecutiveFailures`)
- Manual intervention or registry recovery will reset the counter

### Jitter

To prevent the "thundering herd" problem when multiple bundles are scheduled to poll simultaneously, the operator adds randomness to polling intervals:
- Jitter: ±10% randomness applied to configured poll intervals
- Example: `pollInterval: 15m` will actually poll between 13.5-16.5 minutes
- Spreads load across time rather than having all bundles poll at the same moment

### Configurable Resource Limits

Jobs that run `werf converge` can consume significant resources. Configure limits to prevent cluster disruption:
- Default limits: 1 CPU, 1Gi memory
- Example: `resourceLimits: {cpu: "2", memory: "2Gi"}`
- Recommended for production: Match expected Werf workload requirements
- Jobs that exceed memory limits will be killed; check pod logs for `OOMKilled` status

### Job Logs Retention

Deployment logs are automatically captured and retained in the WerfBundle status for troubleshooting:
- Default retention: 7 days (configurable via `logRetentionDays`)
- Logs are captured from the completed Job pod
- Useful for debugging failed deployments without accessing the cluster directly

## Running Tests

### Unit and Integration Tests

Tests internal components without requiring a cluster:

```bash
make test
```

Shows coverage for:
- API types validation
- Registry client tag handling
- Job specification builder
- Controller reconciliation logic

### End-to-End Tests

Tests the operator against a real Kubernetes cluster (Kind). Requires `kind` and `docker`.

```bash
make local-test
```

This will:
1. Create a local Kind cluster
2. Build and load the operator image
3. Deploy CRDs and operator
4. Run E2E test scenarios (verifying operator validation and error handling):
   - Missing ServiceAccount → fail-fast validation (no Job created, status = Failed)
   - Invalid registry → fail-fast validation (no Job created, status = Failed)

   **Note:** These tests verify the operator's validation logic before Job creation. Garbage collection testing is deferred to Slice 2+ (requires working registry access).
5. Clean up the cluster

## Documentation

- **[DESIGN.md](docs/DESIGN.md)** - Architecture and design decisions
- **[PLAN.md](docs/PLAN.md)** - Implementation roadmap for all slices
- **[Configuration Reference](docs/configuration.md)** - Detailed configuration field documentation, defaults, and reliability features
- **[Troubleshooting Guide](docs/troubleshooting.md)** - Diagnostic procedures and solutions for common issues
- **[RBAC Setup](docs/job-rbac.md)** - Detailed RBAC configuration guide

## Troubleshooting

For common issues and diagnostic procedures, see the [Troubleshooting Guide](docs/troubleshooting.md).

Quick reference for the most common issues:

**Bundle stuck in Syncing**
- Check `status.consecutiveFailures` and `status.lastErrorMessage`
- Verify registry URL is accessible: `kubectl describe werfbundle my-app -n k8s-werf-operator-go-system`
- Check operator logs: `kubectl logs -n k8s-werf-operator-go-system -l control-plane=controller-manager`

**Job fails with OOMKilled**
- Increase memory limits: `kubectl patch werfbundle my-app --type merge -p '{"spec":{"converge":{"resourceLimits":{"memory":"2Gi"}}}}'`

**Jobs not being created**
- Verify ServiceAccount exists in target namespace: `kubectl get sa werf-converge -n <namespace>`
- Check permissions: ServiceAccount must have access to create resources

For detailed troubleshooting procedures, error message explanations, and advanced debugging, see the [Troubleshooting Guide](docs/troubleshooting.md).

## Development

For developers working on the operator:

1. **Local development:**
   ```bash
   # Run tests
   make test

   # Build binary
   make build

   # Generate manifests
   make manifests

   # Lint code
   make lint
   ```

2. **Creating a development cluster:**
   ```bash
   make local-test
   ```

3. **Cleaning up:**
   ```bash
   make cleanup-test-local
   ```

See [DESIGN.md](docs/DESIGN.md) and [PLAN.md](docs/PLAN.md) for architecture details.

## Contributing

Contributions are welcome! Before starting work:

1. Check [PLAN.md](docs/PLAN.md) for current work and priorities
2. Each slice adds a single unit of end-to-end functionality
3. All changes must include tests
4. Run `make test` before submitting changes

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.

## Next Steps

Current roadmap:

- **Slice 2:** ✓ Enhanced registry integration with ETag caching, exponential backoff, and configurable resource management (complete)
- **Slice 3:** Values management and cross-namespace deployments
- **Slice 4:** Drift detection and automatic correction
- **Slice 5:** Advanced features including semantic versioning and username/password auth

For detailed implementation plans, see [PLAN.md](docs/PLAN.md).
