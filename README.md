# Werf Operator for Kubernetes

A Kubernetes operator for deploying and managing applications packaged with [Werf](https://werf.io/) using declarative bundle definitions.

## Current Status

**Slice 1 - Basic Bundle Deployment** (Alpha)

This is early-stage software (v1alpha1 API). The operator supports basic bundle deployment with the following functionality:

- Watch for `WerfBundle` custom resources in the cluster
- Poll OCI registries for available bundle tags
- Create Kubernetes Jobs to run `werf converge` deployments
- Track deployment status in the WerfBundle resource
- Proper RBAC separation (operator minimal, job permissions namespace-scoped)

**What does NOT work yet:**
- Semantic versioning (tags are sorted lexicographically, not by semver)
- Advanced registry authentication (access tokens only, no username/password)
- Cross-namespace deployments
- Drift detection
- Helm integration
- Custom value overrides

See [Slices 2-5 in PLAN.md](.notes/PLAN.md) for the full roadmap.

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
   EOF
   ```

3. **Check status:**

   ```bash
   kubectl get werfbundle -A
   kubectl describe werfbundle my-app -n k8s-werf-operator-go-system
   kubectl logs -n k8s-werf-operator-go-system -l control-plane=controller-manager
   ```

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
4. Run E2E test scenarios:
   - Missing ServiceAccount handling
   - Invalid registry error handling
   - Job lifecycle and cleanup
5. Clean up the cluster

## Documentation

- **[DESIGN.md](.notes/DESIGN.md)** - Architecture and design decisions
- **[PLAN.md](.notes/PLAN.md)** - Implementation roadmap for all slices
- **[RBAC Setup](docs/job-rbac.md)** - Detailed RBAC configuration guide
- **[JOURNAL.md](.notes/JOURNAL.md)** - Development notes and lessons learned

## Troubleshooting

### Bundle stuck in "Syncing" state

- Check ServiceAccount exists in target namespace:
  ```bash
  kubectl get sa werf-converge -n <target-namespace>
  ```
- Check operator logs for errors:
  ```bash
  kubectl logs -n k8s-werf-operator-go-system -l control-plane=controller-manager
  ```

### Registry connection fails

- Verify registry URL is correct and accessible
- Check network connectivity from operator pod
- For private registries, credentials must be configured (Slice 2 feature)

### Job not running

- Check if werf image is available: `werf:latest` from `ghcr.io/werf/werf`
- Verify ServiceAccount has permissions to create necessary resources
- Check Job logs: `kubectl logs job/<job-name> -n <namespace>`

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

See [DESIGN.md](.notes/DESIGN.md) and [PLAN.md](.notes/PLAN.md) for architecture details.

## Contributing

Contributions are welcome! Before starting work:

1. Check [PLAN.md](.notes/PLAN.md) for current work and priorities
2. Each slice adds a single unit of end-to-end functionality
3. All changes must include tests
4. Run `make test` before submitting changes

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.

## Next Steps

What's coming in Slice 2 and beyond:

- **Slice 2:** Enhanced registry integration with ETag caching, exponential backoff, and improved error handling
- **Slice 3:** Values management and multi-namespace support
- **Slice 4:** Drift detection and automatic correction
- **Slice 5:** Advanced features including semantic versioning and username/password auth

Current plans and implementation details are in [PLAN.md](.notes/PLAN.md).
