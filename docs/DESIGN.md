# Werf Operator Design

## Overview

The Werf Operator provides a pull-based model for deploying applications using Werf bundles stored in OCI registries. It watches for new bundle versions, manages deployments, and ensures desired state is maintained.

## Non-Functional Requirements

### Security

**Targets**:
- Least privilege RBAC: Operator has minimal permissions, Jobs use pre-configured ServiceAccounts
- Secret references only: No credentials stored in CRD specs
- Secure defaults: Token-based auth for registries, TLS for registry communication

**Assumptions**:
- Registry credentials are managed outside operator (e.g., via external secrets operator)
- Users are responsible for securing target namespace ServiceAccounts
- Operator runs in trusted namespace with network policies managed externally

**Out of scope for Phase 1**:
- Credential rotation automation
- Network policy generation
- Pod security standards enforcement

### Reliability

**Targets**:
- Eventual consistency: System self-heals through periodic reconciliation
- Graceful degradation: Registry failures don't crash operator, marked in status
- Automatic retry: Exponential backoff for transient failures (max 5 retries)
- Idempotent operations: Safe to reconcile multiple times

**Assumptions**:
- Kubernetes API is available and reliable
- Transient failures resolve within retry window (minutes, not hours)
- Failed Jobs are acceptable if status reflects the failure clearly
- Manual intervention is acceptable for persistent failures

**Out of scope for Phase 1**:
- Circuit breakers for persistently failing bundles
- Dead-letter queue or alerting for stuck resources
- Automatic rollback on failed deployments

### Performance

**Targets**:
- Reconciliation latency: Complete within 10 seconds per resource
- Registry poll interval: Default 15 minutes (configurable)
- Drift detection interval: Default 15 minutes (configurable)
- Operator memory: Under 256Mi baseline, scales with WerfBundle count

**Assumptions**:
- Small-to-medium scale: 10-100 WerfBundle resources per cluster
- Registry responses under 5 seconds
- Low churn rate: Bundles update hourly or daily, not constantly
- Sequential job processing per bundle is acceptable

**Out of scope for Phase 1**:
- Large-scale optimization (1000+ bundles)
- Concurrent job execution per bundle
- Aggressive caching or preloading strategies
- Performance benchmarks or load testing

### Observability

**Targets**:
- Structured logging: JSON logs with context (bundle name, namespace, operation)
- Prometheus metrics: Standard controller metrics plus custom metrics
- Status visibility: Phase, last applied version, error messages in CRD status
- Job logs: Retained for 7 days (configurable)

**Assumptions**:
- Prometheus is available for metrics scraping
- Log aggregation handled externally (e.g., fluentd, Loki)
- Users check status via kubectl or Kubernetes dashboard
- Info-level logging is sufficient for normal operations

**Out of scope for Phase 1**:
- Distributed tracing (OpenTelemetry)
- Custom alerting rules or SLOs
- Log sampling or rate limiting
- Grafana dashboards

### Availability

**Targets**:
- Single replica sufficient: Leader election built-in but not required initially
- Restart tolerance: State persisted in CRD status, no local state loss
- Graceful shutdown: In-flight reconciliations complete before termination

**Assumptions**:
- Brief operator downtime (seconds to minutes) is acceptable
- Kubernetes will restart failed operator pods
- No SLA or uptime guarantees for Phase 1
- Eventual consistency means missed events aren't critical

**Out of scope for Phase 1**:
- Multi-replica high availability setup
- Active-active or active-standby patterns
- Failure injection testing or chaos engineering
- SLO definitions or SLA commitments

### Scale

**Targets**:
- WerfBundle resources: 10-100 per cluster
- Namespaces: 10-50 target namespaces
- Concurrent jobs: Limited by Kubernetes scheduler, no operator-imposed limit
- Registry connections: Connection pooling, max 10 concurrent registry operations

**Assumptions**:
- Single cluster deployment
- Standard Kubernetes cluster (not edge or constrained environments)
- Registry can handle polling load (1 req/bundle/15min = ~0.1 req/sec for 100 bundles)
- Job creation rate under 1/second

**Out of scope for Phase 1**:
- Multi-cluster federation or management
- Horizontal operator scaling (sharding by namespace)
- Rate limiting or throttling of operations
- Scale testing beyond 100 bundles

### Operational Complexity

**Targets**:
- Simple deployment: Single kubectl apply for operator installation
- Standard observability: Works with existing Prometheus/logging setup
- Clear error messages: Status conditions explain what's wrong and how to fix

**Assumptions**:
- Operators have kubectl access and basic Kubernetes knowledge
- Standard Kubernetes cluster (GKE, EKS, AKS, or self-managed with kubeadm)
- Internet connectivity for pulling images and accessing registries
- Standard cluster networking (no service mesh or complex CNI initially)

**Out of scope for Phase 1**:
- Airgap or offline deployments
- Proxy or egress control configuration
- Service mesh integration (Istio, Linkerd)
- Multi-tenancy beyond namespace isolation
- Operator lifecycle management (OLM)

## Core Components

### Custom Resources

**WerfBundle CRD** containing both spec and status:
- Spec defines bundle source, auth, polling config, and deployment preferences
- Status tracks current state, last applied version, and error conditions

### Controller Components

1. **Registry Poller**
   - Polls OCI registries on configured intervals (default 15m)
   - Implements ETag caching and exponential backoff
   - Supports access token auth (username/password planned)
   - Adds jitter to prevent thundering herd

2. **Converge Manager**
   - Creates and monitors k8s Jobs running `werf converge`
   - Implements job deduplication
   - Captures logs for status updates
   - Supports dry-run validation
   - Manages cross-namespace deployments
   - Handles external configuration values

3. **Drift Detector**
   - Periodically runs convergence checks
   - Detects and corrects manual changes
   - Configurable check interval (default 15m)

### Metrics

Standard controller metrics plus custom metrics for:
- Registry polling attempts/duration
- New tag detection
- Converge job execution/results
- Drift detection stats

## RBAC Architecture

### Operator ServiceAccount

The operator pod runs with minimal permissions required for core functionality:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: werf-operator
rules:
- apiGroups: ["werf.io"]
  resources: ["werfbundles"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["werf.io"]
  resources: ["werfbundles/status"]
  verbs: ["get", "update", "patch"]
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["create", "get", "list", "watch", "delete"]
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["get", "list"]
```

### Job ServiceAccount

Each target namespace requires a pre-configured ServiceAccount with permissions for Werf deployments:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: werf-converge
  namespace: target-namespace
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["*"]
```

## Resource Definitions

### WerfBundle CRD Example

```yaml
apiVersion: werf.io/v1alpha1
kind: WerfBundle
metadata:
  name: my-app
  namespace: werf-operator-system
spec:
  # Bundle source configuration
  registry:
    url: ghcr.io/org/bundle
    secretRef:
      name: registry-creds
    pollInterval: 15m
    versionConstraint: ">=1.0.0"

  # Deployment configuration
  converge:
    targetNamespace: my-app-prod
    serviceAccountName: werf-converge-sa
    resourceLimits:
      cpu: "1"
      memory: "1Gi"
    logRetentionDays: 7
    driftDetection:
      enabled: true
      interval: 15m
      maxRetries: 5
    valuesFrom:
      - configMapRef:
          name: app-values
      - secretRef:
          name: app-secrets
          optional: true

status:
  phase: Synced  # Syncing | Synced | Failed
  lastAppliedTag: "1.2.3"
  lastSyncTime: "2025-10-23T10:00:00Z"
  lastErrorMessage: ""
```

## Value Management

External configuration can be provided through ConfigMaps and Secrets:
- Multiple sources supported and merged in order
- Optional sources can be marked
- Values converted to `--set` flags for werf converge
- Sources can be in operator namespace or target namespace

## Deployment Phases

### Phase 1: Core Functionality
- Basic CRD implementation
- Registry polling with access token auth
- Converge job creation and monitoring
- Simple drift detection
- Core metrics
- Basic RBAC implementation

### Phase 2: Enhanced Features
- Username/password auth support
- Private registry TLS configuration
- Namespace-scoped resources
- Advanced drift handling with alerts
- Extended metrics and monitoring
- Values management

### Phase 3: Advanced Features
- Additional auth methods
- Registry-specific configurations
- Enhanced rollback capabilities
- Advanced bundle validation
- Extended RBAC controls