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

## Operator Permissions (ClusterRole)

The operator itself runs with a **ClusterRole** that grants cluster-wide permissions for specific resources:

### Cluster-Wide Read Access

| Resource | Verbs | Why Needed |
|----------|-------|------------|
| Secrets | `get` | Read registry credentials and values from target namespaces |
| ServiceAccounts | `get`, `list`, `watch` | Validate ServiceAccount exists before creating Jobs |
| ConfigMaps | `get`, `list` | Read configuration values from target namespaces |

### Cluster-Wide Write Access

| Resource | Verbs | Why Needed |
|----------|-------|------------|
| ConfigMaps | `create`, `update` | Status tracking and caching (operator namespace only) |
| Jobs | `create`, `delete`, `get`, `list`, `watch` | Create werf converge Jobs in target namespaces |
| WerfBundles | `update`, `patch` | Update WerfBundle status |
| WerfBundles/status | `update`, `patch` | Update WerfBundle status subresource |

### Why Cluster-Wide Access?

Cross-namespace deployments require the operator to:
- Read registry credentials from target namespaces (where apps deploy)
- Validate ServiceAccounts exist in target namespaces before creating Jobs
- Resolve configuration values from ConfigMaps/Secrets in target namespaces
- Create Jobs in target namespaces (not just the operator's namespace)

**Important distinction:**
- **Operator reads configuration** (Secrets, ConfigMaps, ServiceAccounts) cluster-wide
- **Jobs execute deployments** (create Pods, Services, etc.) with namespace-scoped permissions
- The operator cannot directly create application resources; it only orchestrates Job creation

### Single-Tenant vs Multi-Tenant

The operator's ClusterRole assumes a **single-tenant cluster** where:
- The operator is trusted infrastructure
- Cluster-wide read access to Secrets is acceptable
- Namespace isolation is organizational, not a security boundary

For **multi-tenant clusters** with strict namespace isolation:
- Consider network policies to restrict cross-namespace communication
- See [Security Model](security-model.md) for multi-tenant considerations

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

## Pre-Deployment Verification Checklist

Before creating a WerfBundle, verify RBAC is correctly configured. Follow this checklist in order:

### 1. Target Namespace Exists

```bash
kubectl get namespace my-app-prod
# Expected: Namespace details (not "NotFound" error)
```

If namespace doesn't exist:
```bash
kubectl create namespace my-app-prod
```

### 2. Job ServiceAccount Created

```bash
kubectl get serviceaccount werf-converge -n my-app-prod
# Expected: ServiceAccount details
```

If not found, create the ServiceAccount, Role, and RoleBinding (see "Setting Up Job ServiceAccount" section above).

### 3. Role and RoleBinding Configured

```bash
# Verify Role exists
kubectl get role werf-converge -n my-app-prod

# Verify RoleBinding connects SA to Role
kubectl get rolebinding werf-converge -n my-app-prod -o yaml
# Check: subjects[0].name == "werf-converge" && roleRef.name == "werf-converge"
```

### 4. Job Permissions Valid

Run the verification commands from "Verify Job ServiceAccount Permissions" section below.

All `kubectl auth can-i` commands for Job resources should return "yes".

### 5. Operator Can Access Target Namespace

```bash
# Operator should be able to validate SA exists
kubectl auth can-i get serviceaccounts -n my-app-prod \
  --as=system:serviceaccount:werf-system:werf-operator-controller-manager
# Expected: "yes"

# Operator should be able to create Jobs
kubectl auth can-i create jobs -n my-app-prod \
  --as=system:serviceaccount:werf-system:werf-operator-controller-manager
# Expected: "yes"
```

### 6. Ready to Deploy

If all checks pass, you can create a WerfBundle targeting this namespace:

```yaml
apiVersion: werf.io/v1alpha1
kind: WerfBundle
metadata:
  name: my-app
  namespace: werf-system
spec:
  registry:
    url: ghcr.io/org/my-app-bundle
  targetNamespace: my-app-prod
  converge:
    serviceAccountName: werf-converge
```

Monitor the deployment:
```bash
kubectl describe werfbundle my-app -n werf-system
kubectl get jobs -n my-app-prod -l app.kubernetes.io/instance=my-app
```

## Cross-Namespace Verification

The operator's primary use case is cross-namespace deployment (operator in `werf-system`, apps in other namespaces). Verify the operator can access target namespaces.

### Understanding Cross-Namespace Access

The operator needs:
- **ServiceAccount validation**: Read ServiceAccounts in target namespace (pre-flight check)
- **Secrets resolution**: Read registry credentials and values from target namespace
- **Job creation**: Create Jobs in target namespace (not just operator namespace)

These permissions come from a **ClusterRole** bound via **ClusterRoleBinding** (see `config/rbac/role.yaml` and `config/rbac/role_binding.yaml`).

### Verify Operator Cross-Namespace Permissions

Test if operator can perform required operations in a different namespace:

```bash
# Replace 'werf-system' with operator namespace
# Replace 'my-app-prod' with target namespace

# 1. Operator must validate ServiceAccount exists
kubectl auth can-i get serviceaccounts -n my-app-prod \
  --as=system:serviceaccount:werf-system:werf-operator-controller-manager
# Expected: "yes"

# 2. Operator must read Secrets for registry credentials
kubectl auth can-i get secrets -n my-app-prod \
  --as=system:serviceaccount:werf-system:werf-operator-controller-manager
# Expected: "yes"

# 3. Operator must create Jobs in target namespace
kubectl auth can-i create jobs -n my-app-prod \
  --as=system:serviceaccount:werf-system:werf-operator-controller-manager
# Expected: "yes"
```

If any command returns "no", verify:

1. **ClusterRole exists and has required rules**:
```bash
kubectl get clusterrole werf-operator-manager-role -o yaml
# Check for rules with resources: [serviceaccounts, secrets, jobs]
```

2. **ClusterRoleBinding exists and binds to operator ServiceAccount**:
```bash
kubectl get clusterrolebinding werf-operator-manager-rolebinding -o yaml
# Check subjects[0].name == "werf-operator-controller-manager"
# Check subjects[0].namespace == "werf-system" (your operator namespace)
# Check roleRef.name == "werf-operator-manager-role"
```

3. **NOT using RoleBinding** (common mistake):
```bash
# This should return no results (RoleBinding is namespace-scoped, won't work)
kubectl get rolebinding -n werf-system | grep werf-operator
```

### Testing Multiple Target Namespaces

If deploying to multiple namespaces, test operator permissions for each:

```bash
for ns in my-app-dev my-app-staging my-app-prod; do
  echo "Testing namespace: $ns"
  kubectl auth can-i get serviceaccounts -n $ns \
    --as=system:serviceaccount:werf-system:werf-operator-controller-manager
done
# Expected: "yes" for all namespaces
```

ClusterRole permissions are cluster-wide; if they work for one namespace, they work for all. This test verifies the ClusterRoleBinding is correctly configured.

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

### Verify Operator Permissions

The operator runs with a ClusterRole (see `config/rbac/role.yaml`). Verify it has cluster-wide permissions:

```bash
# Check if operator can read Secrets cluster-wide
kubectl auth can-i get secrets --all-namespaces \
  --as=system:serviceaccount:werf-system:werf-operator-controller-manager

# Check if operator can list ServiceAccounts cluster-wide
kubectl auth can-i list serviceaccounts --all-namespaces \
  --as=system:serviceaccount:werf-system:werf-operator-controller-manager

# Check if operator can create Jobs in target namespace
kubectl auth can-i create jobs -n my-app-prod \
  --as=system:serviceaccount:werf-system:werf-operator-controller-manager

# Verify operator CANNOT directly create Pods in target namespace (Jobs should)
kubectl auth can-i create pods -n my-app-prod \
  --as=system:serviceaccount:werf-system:werf-operator-controller-manager
# Expected: "no" - operator creates Jobs, not Pods directly
```

**Note**: Replace `werf-system` with your operator namespace and `my-app-prod` with your target namespace.

### Verify Job ServiceAccount Permissions

Jobs run with namespace-scoped ServiceAccounts. Verify the Job ServiceAccount has required permissions in the target namespace:

```bash
# Check if Job SA can create Deployments
kubectl auth can-i create deployments -n my-app-prod \
  --as=system:serviceaccount:my-app-prod:werf-converge

# Check if Job SA can update Services
kubectl auth can-i update services -n my-app-prod \
  --as=system:serviceaccount:my-app-prod:werf-converge

# Check if Job SA can manage Ingresses
kubectl auth can-i create ingresses -n my-app-prod \
  --as=system:serviceaccount:my-app-prod:werf-converge

# Verify Job SA CANNOT access other namespaces (security boundary)
kubectl auth can-i get pods -n other-namespace \
  --as=system:serviceaccount:my-app-prod:werf-converge
# Expected: "no" - Jobs are namespace-scoped
```

If any expected permission returns "no", review your Role and RoleBinding configuration.

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
