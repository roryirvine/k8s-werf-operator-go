# Security Model

## Deployment Model

The Werf operator is designed for **single-tenant Kubernetes clusters** where the operator is trusted with cluster-wide read access to configuration resources. This document explains the security architecture, trust boundaries, and considerations for multi-tenant environments.

## Operator RBAC Permissions

The operator runs with a **ClusterRole** bound via **ClusterRoleBinding**, granting it cluster-wide permissions for specific resources.

### Cluster-Wide Read Access

The operator has read-only access to the following resources across all namespaces:

| Resource | Verbs | Purpose |
|----------|-------|---------|
| Secrets | `get` | Registry credentials, values resolution from target namespaces |
| ServiceAccounts | `get`, `list`, `watch` | Pre-flight validation before Job creation |
| ConfigMaps | `get`, `list` | Values resolution from target namespaces |

### Cluster-Wide Write Access

The operator has write access to specific resources:

| Resource | Verbs | Purpose |
|----------|-------|---------|
| ConfigMaps | `create`, `update` | Status tracking and caching (operator namespace only) |
| Jobs | `create`, `delete` | Create werf converge Jobs in target namespaces |
| WerfBundles | `update`, `patch` | Update CRD status |

### Why Cluster-Wide Access is Necessary

Cross-namespace deployments require the operator to:

1. **Read registry credentials** from target namespaces where applications deploy
2. **Resolve configuration values** from ConfigMaps and Secrets in target namespaces
3. **Validate ServiceAccounts exist** in target namespaces before creating Jobs
4. **Create Jobs** in target namespaces (not just the operator's namespace)

Without cluster-wide read access, the operator couldn't support cross-namespace deployments.

## Security Boundaries

### Operator vs Job Permissions

The Werf operator follows a **split RBAC model** with clear separation of concerns:

**Operator (cluster-wide read, limited write):**
- Reads configuration from any namespace
- Creates Jobs in target namespaces
- Updates WerfBundle status
- Does NOT have deployment permissions

**Jobs (namespace-scoped deployment permissions):**
- Run with target namespace ServiceAccount
- Execute `werf converge` with namespace-scoped permissions
- Create/update application resources in target namespace only
- Permissions defined by target namespace's Role/RoleBinding

### Trust Boundary

```
┌─────────────────────────────────────────────────────────┐
│ Operator (Cluster-wide read, limited write)            │
│ - Reads Secrets from any namespace                      │
│ - Reads ServiceAccounts from any namespace              │
│ - Creates Jobs in target namespaces                     │
│                                                          │
│ Trust: Operator is trusted infrastructure component     │
└─────────────────────────────────────────────────────────┘
                           │
                           │ creates Job with
                           │ target namespace SA
                           ▼
┌─────────────────────────────────────────────────────────┐
│ Job (Namespace-scoped deployment permissions)           │
│ - Runs with target namespace ServiceAccount             │
│ - Creates/updates application resources                 │
│ - Scoped to target namespace only                       │
│                                                          │
│ Trust: Job uses application-specific permissions        │
└─────────────────────────────────────────────────────────┘
```

**Key insight:** The operator can *read* configuration but Jobs *execute* deployments. This separation limits the blast radius of compromised credentials.

## Least-Privilege Considerations

While the operator has cluster-wide access, it follows least-privilege principles:

1. **Read-only for sensitive resources:** Secrets and ServiceAccounts are `get` only (no create/update/delete)
2. **Specific resources:** Only ConfigMaps, Secrets, ServiceAccounts, and Jobs (not all resources)
3. **Code discipline:** The operator only reads from bundle namespace and target namespace, not arbitrary namespaces
4. **Job permissions scoped:** Jobs run with target namespace ServiceAccount, not operator's privileges

## Single-Tenant Deployment (Current Model)

The operator assumes a **single-tenant cluster** where:

- All users trust the operator to access their Secrets
- Namespace isolation is organizational, not a security boundary
- The operator is part of trusted cluster infrastructure
- Compromise of the operator would expose all Secrets (same as other cluster components)

This is the standard model for most Kubernetes operators (cert-manager, external-secrets, ArgoCD, etc.).

## Multi-Tenant Considerations

If deploying to a **multi-tenant cluster** where namespace isolation is a security boundary:

### Current Limitations

- The operator can technically read Secrets from any namespace (RBAC allows it)
- Code only attempts to read from bundle namespace and target namespace
- No runtime enforcement preventing access to arbitrary namespaces
- This may violate multi-tenant isolation requirements

### Options for Multi-Tenant Deployments

**Option 1: Network Policies**

Implement network policies to restrict which namespaces can be accessed:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: isolate-tenant-namespaces
  namespace: tenant-a
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          tenant: tenant-a
```

Network policies prevent cross-namespace communication even if RBAC allows it.

**Option 2: Namespace-Scoped Roles (Future Enhancement)**

Replace ClusterRole with namespace-scoped Roles:

- Deploy operator instances per tenant namespace
- Grant Role (not ClusterRole) scoped to specific namespaces
- Trade-off: More complex deployment, multiple operator instances
- Future enhancement if multi-tenancy becomes a requirement

**Option 3: Runtime Namespace Validation (Future Enhancement)**

Add code-level validation to reject reading Secrets/ConfigMaps from namespaces other than bundle or target:

- Defense-in-depth even though RBAC allows broader access
- Prevents accidental or malicious misuse
- Trade-off: Additional complexity
- Future enhancement if needed

### Recommendation for Multi-Tenant

If you need multi-tenant isolation:

1. **Short-term:** Deploy network policies to restrict cross-namespace access
2. **Medium-term:** Document trusted namespace relationships in WerfBundle annotations
3. **Long-term:** Implement namespace-scoped Roles if needed (requires operator architecture changes)

## Secrets Management Best Practices

Even in single-tenant clusters, follow these practices:

1. **Use external secrets operators:** Don't store sensitive values directly in Kubernetes Secrets
   - Tools: External Secrets Operator, sealed-secrets, Vault
   - Benefit: Rotate credentials without cluster access

2. **Limit Secret scope:** Create Secrets in target namespaces, not operator namespace
   - Reduces exposure if operator namespace is compromised
   - Follows principle of least privilege

3. **Audit Secret access:** Enable Kubernetes audit logging for Secret reads
   - Monitor which components access which Secrets
   - Detect unexpected access patterns

4. **Registry credentials:** Use ImagePullSecrets where possible instead of Secret references in WerfBundle
   - Kubernetes automatically handles credential injection
   - Reduces operator's need to read registry Secrets

## Related Documentation

- [Job RBAC Setup](job-rbac.md) - How to configure ServiceAccounts for target namespaces
- [Design Document](DESIGN.md) - Overall operator architecture and RBAC decisions
