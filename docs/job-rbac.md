# Job RBAC Configuration

## Overview

The Werf Operator uses a split RBAC model:

- **Operator ServiceAccount** (in operator namespace): Minimal permissions to watch WerfBundles and create Jobs
- **Job ServiceAccount** (in target namespace): Broad permissions to run `werf converge` and manage application resources

This design follows the principle of least privilege. The operator itself cannot directly modify application resources; it can only orchestrate Job creation.

## Why Split RBAC?

If the operator had broad permissions:
- A vulnerability in the operator could compromise all deployed applications
- The operator namespace could become a single point of attack

With split RBAC:
- The operator namespace is a narrow attack surface (can only create jobs)
- Each target namespace is isolated (compromise of one doesn't affect others)
- Different teams can manage operator vs application permissions separately

## Setting Up Job ServiceAccount

Each **target namespace** where WerfBundles are deployed must have:
1. A ServiceAccount named `werf-converge` (or custom name specified in WerfBundle spec)
2. A Role with permissions werf needs
3. A RoleBinding connecting the ServiceAccount to the Role

### Example Setup

Create these resources in your **target namespace** (e.g., `my-app-prod`):

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: werf-converge
  namespace: my-app-prod

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: werf-converge
  namespace: my-app-prod
rules:
# Werf needs to manage core application resources
- apiGroups: [""]
  resources: ["pods", "services", "configmaps", "secrets", "persistentvolumeclaims"]
  verbs: ["create", "update", "patch", "delete", "get", "list", "watch"]

# Werf needs to manage deployment resources
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
  verbs: ["create", "update", "patch", "delete", "get", "list", "watch"]

# Werf needs to manage ingress for routing
- apiGroups: ["networking.k8s.io"]
  resources: ["ingresses"]
  verbs: ["create", "update", "patch", "delete", "get", "list", "watch"]

# Optional: For helm/kustomize deployments
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: werf-converge
  namespace: my-app-prod
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: werf-converge
subjects:
- kind: ServiceAccount
  name: werf-converge
  namespace: my-app-prod
```

### Customizing Permissions

The permissions above are broad but appropriate for Werf converge operations. Depending on your use case, you can:

- **Remove unused resources**: If you only deploy Deployments and Services, remove StatefulSets, DaemonSets, etc.
- **Restrict to specific namespace resources**: The example above is namespace-scoped (can't access cluster-wide resources like Nodes)
- **Add cluster-scoped resources if needed**: If Werf manages custom resources or cluster-scoped resources, add rules for those

**Example: Minimal permissions for simple deployments**

```yaml
apiGroups: [""]
resources: ["services", "configmaps", "secrets"]
verbs: ["create", "update", "patch", "delete", "get", "list", "watch"]
---
apiGroups: ["apps"]
resources: ["deployments"]
verbs: ["create", "update", "patch", "delete", "get", "list", "watch"]
```

## Understanding Job Labels

When the operator creates a Job to run `werf converge`, it adds standard Kubernetes labels for identification and management:

| Label | Value | Purpose |
|-------|-------|---------|
| `app.kubernetes.io/name` | `werf-operator` | Identifies this Job is managed by the Werf Operator |
| `app.kubernetes.io/instance` | Bundle name (e.g., `my-app`) | Identifies which WerfBundle created this Job |
| `app.kubernetes.io/managed-by` | `werf-operator` | Confirms the operator manages this resource |

**Finding Jobs for a specific WerfBundle:**

Use the `instance` label to find Jobs created for a specific bundle:

```bash
# Find all jobs for the "my-app" WerfBundle
kubectl get jobs -n my-app-prod -l app.kubernetes.io/instance=my-app

# Watch logs of the latest job for "my-app"
kubectl logs -n my-app-prod \
  -l app.kubernetes.io/instance=my-app,app.kubernetes.io/name=werf-operator \
  -f --max-log-requests=1
```

## Using a Custom ServiceAccount Name

By default, WerfBundle uses ServiceAccount `werf-converge`. To use a different name:

```yaml
apiVersion: werf.io/v1alpha1
kind: WerfBundle
metadata:
  name: my-app
spec:
  registry:
    url: ghcr.io/org/my-app-bundle
  converge:
    serviceAccountName: custom-werf-sa  # Use this ServiceAccount instead
```

Make sure to create the ServiceAccount with the matching name in your target namespace.

## Verification

To verify your RBAC setup is correct:

1. **Check ServiceAccount exists**:
```bash
kubectl get serviceaccount werf-converge -n my-app-prod
```

2. **Check Role is bound to ServiceAccount**:
```bash
kubectl get rolebinding werf-converge -n my-app-prod
kubectl describe rolebinding werf-converge -n my-app-prod
```

3. **Verify Role has expected permissions**:
```bash
kubectl get role werf-converge -n my-app-prod -o yaml
```

4. **Watch operator logs for ServiceAccount errors**:
If the ServiceAccount is missing, the operator will fail reconciliation with:
```
ServiceAccount werf-converge not found in namespace my-app-prod
```

5. **Check Job's status** after WerfBundle is created:
```bash
kubectl get jobs -n my-app-prod -l app.kubernetes.io/instance=my-app
kubectl describe job <job-name> -n my-app-prod  # Check Events for auth errors
```

## Troubleshooting

### ServiceAccount not found error

The operator reconciles WerfBundle but can't create Jobs because the ServiceAccount doesn't exist in the target namespace.

**Fix**: Create the ServiceAccount in the target namespace (see example above).

### Job created but fails with permission denied

The Job was created but fails when running `werf converge`. The ServiceAccount exists but doesn't have permissions for resources werf needs.

**Fix**: Review the Role and add missing resources that werf is trying to create (check Job logs for hints).

```bash
kubectl logs -n my-app-prod <job-pod-name>
```

### Different namespaces for operator and bundles

If the operator is in namespace `werf-system` but WerfBundles are in namespace `my-app`, make sure:

1. The operator's ClusterRole (generated by `make manifests`) can watch WerfBundles in all namespaces
2. Each target namespace has its own ServiceAccount and Role setup

The operator is cluster-scoped (can watch WerfBundles everywhere), but Jobs always run in their corresponding namespaces using that namespace's ServiceAccount.

## Security Notes

- ServiceAccount tokens are automatically mounted into Pods created with that ServiceAccount
- The Job inherits all permissions the ServiceAccount has in that namespace
- Jobs in different namespaces cannot affect each other (namespace isolation)
- To rotate permissions: update the Role, Jobs created after the change get new permissions automatically

## Next Steps

Once you have RBAC configured:

1. Create a WerfBundle resource pointing to your bundle
2. Operator creates a Job in the target namespace
3. Job runs `werf converge` with the ServiceAccount's permissions
4. Application resources are deployed/updated as specified in the bundle
