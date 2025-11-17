# WerfBundle Configuration Reference

This document describes all configurable fields in the WerfBundle resource and explains the behavior of reliability features.

## Registry Configuration

The `spec.registry` section defines how the operator accesses your OCI registry.

### url (Required)

The OCI registry URL where bundle images are stored.

```yaml
spec:
  registry:
    url: ghcr.io/org/my-app-bundle
```

**Format**: Full registry URL (must be accessible from the operator pod)

**Examples**:
- Docker Hub: `docker.io/myorg/mybundle`
- GitHub Container Registry: `ghcr.io/myorg/mybundle`
- GitLab Registry: `registry.gitlab.com/mygroup/mybundle`
- Private registry: `registry.internal.company.com/mybundle`

### secretRef (Optional)

Reference to a Kubernetes Secret containing registry credentials.

```yaml
spec:
  registry:
    secretRef:
      name: registry-creds
```

**How it works**:
- Secret must exist in the operator namespace (currently `k8s-werf-operator-go-system`)
- Secret must contain a `.dockerconfigjson` key with valid Docker config
- Used for private registries requiring authentication (currently access token auth only)
- If not specified, the operator attempts anonymous access

**Creating a registry secret**:

```bash
kubectl create secret docker-registry registry-creds \
  --docker-server=ghcr.io \
  --docker-username=<your-username> \
  --docker-password=<your-token> \
  -n k8s-werf-operator-go-system
```

### pollInterval (Optional)

How frequently the operator checks the registry for new bundle tags.

```yaml
spec:
  registry:
    pollInterval: 15m
```

**Default**: `15m` (15 minutes)

**Valid formats**: Kubernetes duration string
- Seconds: `30s`, `60s`
- Minutes: `5m`, `15m`, `30m`
- Hours: `1h`, `2h`, `6h`, `24h`

**Constraints**:
- Minimum: `1m` (prevents excessive registry load)
- Maximum: `24h` (reasonable upper bound)

**How polling works**:
- Interval is a target, not a guarantee - reconciliation happens on all events plus periodic resync
- Proper interval enforcement will be added in a future release
- See [ETag Caching](#etag-caching) below for how polling is optimized

**Note on jitter**: A ±10% random variation is automatically added to the poll interval to spread load when multiple bundles have the same interval. For example, a 15-minute interval will actually poll between 13.5 and 16.5 minutes.

## Converge Configuration

The `spec.converge` section defines how `werf converge` deployments are executed.

### serviceAccountName (Required)

Name of the Kubernetes ServiceAccount to use when running werf converge Jobs.

```yaml
spec:
  converge:
    serviceAccountName: werf-converge
```

**Requirements**:
- ServiceAccount must exist in the target namespace (specified in separate configuration)
- ServiceAccount must have permissions to create/update the resources defined in your Werf manifest
- Operator validates this before creating a Job; if missing, the bundle is marked Failed

**Setup example**:

```bash
kubectl create serviceaccount werf-converge -n production
kubectl apply -f - <<EOF
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

### resourceLimits (Optional)

CPU and memory limits for werf converge Jobs.

```yaml
spec:
  converge:
    resourceLimits:
      cpu: "2"
      memory: "2Gi"
```

**Defaults**:
- CPU: `1` (1 CPU core)
- Memory: `1Gi`

**Why limits matter**:
- Without limits, a runaway Werf process can consume cluster resources and starve other workloads
- Memory limit violations result in `OOMKilled` status - check pod logs if jobs fail suddenly
- CPU limits are soft (processes can burst above limit), memory limits are hard (pod is killed if exceeded)

**Recommended values**:

| Workload Size | CPU | Memory | Notes |
|---|---|---|---|
| Tiny (< 5 services, < 100MB) | 500m | 512Mi | Small Werf build/deployment |
| Small (5-20 services, 100MB-1GB) | 1 | 1Gi | Default; handles most apps |
| Medium (20-50 services, 1-2GB) | 2 | 2Gi | Larger Werf builds or complex manifests |
| Large (50+ services, 2GB+) | 4 | 4Gi | Significant build/deploy workloads |

**Format**:
- CPU: `"500m"` (milliCPU), `"1"` (core), `"1.5"` (fractional core)
- Memory: `"512Mi"`, `"1Gi"`, `"1G"` (Kubernetes format)

### logRetentionDays (Optional)

How long completed Jobs should be retained in the cluster for log inspection.

```yaml
spec:
  converge:
    logRetentionDays: 7
```

**Default**: `7` (7 days)

**How it works**:
- Completed Jobs are automatically deleted after this period by Kubernetes
- Job logs are captured in the WerfBundle status (up to ~5KB) immediately after completion
- Large logs are stored separately; check pod logs directly if needed: `kubectl logs job/<job-name>`

**Constraints**:
- Minimum: `1` day
- Practical maximum: `30` days (older logs should be archived to external logging system)

**Considerations**:
- Shorter retention (1-3 days) for high-volume deployments to reduce cluster storage
- Longer retention (14-30 days) for production deployments to facilitate debugging
- Logs beyond what fits in status (~5KB) require checking pod logs directly

## Reliability Behavior

### ETag Caching

The operator uses HTTP ETag headers to minimize registry requests and save bandwidth.

**How it works**:
1. First registry poll: Operator requests full tag list, receives response with ETag header
2. ETag stored in `status.lastETag`
3. Subsequent polls: Operator includes `If-None-Match: <etag>` header in request
4. Registry response:
   - If content unchanged: HTTP 304 Not Modified (ETag matches)
   - If content changed: HTTP 200 OK with new tag list

**Bandwidth savings**:
- Unchanged registries: Zero bytes transferred (just HTTP headers)
- Valuable for large registries or frequent polling
- Example: 1000-tag registry (50KB response) × 96 polls/day × 30 days = 144MB saved with ETag caching

**Status field** (`status.lastETag`):
- Updated whenever registry content changes
- Reset when bundle is manually edited
- Used internally for duplicate detection

### Exponential Backoff

When registry polling fails (network errors, timeouts, server errors), the operator automatically retries with exponential backoff.

**Retry behavior**:
- **Max retries**: 5 consecutive failures (marked Failed on 6th)
- **Backoff sequence**: 30s → 1m → 2m → 4m → 8m → 8m (capped)
- **Reset**: Counter resets to 0 on successful poll or successful Job completion
- **Manual recovery**: Editing the WerfBundle or waiting for success resets the counter

**Status fields** related to backoff:
- `status.consecutiveFailures`: Current failure count (0-6)
- `status.lastErrorTime`: Timestamp of most recent failure
- `status.lastErrorMessage`: Description of the error

**Example timeline**:
```
12:00:00 - Poll fails → consecutiveFailures=1, requeue after 30s
12:00:30 - Poll fails → consecutiveFailures=2, requeue after 1m
12:01:30 - Poll fails → consecutiveFailures=3, requeue after 2m
12:03:30 - Poll fails → consecutiveFailures=4, requeue after 4m
12:07:30 - Poll fails → consecutiveFailures=5, requeue after 8m
12:15:30 - Poll fails → consecutiveFailures=6, bundle marked Failed
12:15:35 - Poll succeeds → consecutiveFailures=0, bundle marked Synced
```

### Jitter

Random variation is added to poll intervals to prevent the "thundering herd" problem.

**How it works**:
- ±10% random variation applied to configured `pollInterval`
- Example: `pollInterval: 15m` → polls between 13.5m and 16.5m
- Different for each bundle, so multiple bundles don't poll simultaneously

**Why it matters**:
- Without jitter: All 100 bundles poll at exactly 15:00, overwhelming registry
- With jitter: Polls spread over 13:30-16:30 range, distributing registry load
- Prevents cascading failures and unfair quota consumption

**Example**:
```yaml
spec:
  registry:
    pollInterval: 15m
```

Actual polls will occur at random intervals between 13.5m and 16.5m, distributed across all bundles.

## Complete Example

```yaml
apiVersion: werf.io/v1alpha1
kind: WerfBundle
metadata:
  name: my-app
  namespace: k8s-werf-operator-go-system
spec:
  # Registry configuration with credentials
  registry:
    url: ghcr.io/myorg/my-app-bundle
    secretRef:
      name: ghcr-credentials
    pollInterval: 10m

  # Converge configuration for deployment
  converge:
    serviceAccountName: werf-converge
    resourceLimits:
      cpu: "2"
      memory: "2Gi"
    logRetentionDays: 14
```

This configuration:
- Polls `ghcr.io/myorg/my-app-bundle` every ~10 minutes (with ±10% jitter)
- Uses `ghcr-credentials` Secret for authentication
- Runs converge Jobs with 2 CPU and 2Gi memory limits
- Retains completed Job logs for 14 days
- Uses `werf-converge` ServiceAccount in the target namespace

## Troubleshooting Configuration Issues

### "Bundle stuck in Syncing with many consecutive failures"

Check `spec.registry` configuration:
- **Invalid registry URL**: Verify `spec.registry.url` is correct and accessible
- **Authentication failed**: Check `spec.registry.secretRef` exists and contains valid credentials
- **Registry offline**: Check registry health; operator will retry with exponential backoff

```bash
kubectl describe werfbundle my-app -n k8s-werf-operator-go-system
# Look at status.lastErrorMessage and status.consecutiveFailures
```

### "Jobs failing with OOMKilled"

Increase memory in `spec.converge.resourceLimits.memory`:

```bash
# Check pod logs to confirm OOMKilled
kubectl logs job/my-app-<hash>-<uuid> -n k8s-werf-operator-go-system

# Update the WerfBundle with larger memory
kubectl patch werfbundle my-app -n k8s-werf-operator-go-system --type merge -p '
{"spec":{"converge":{"resourceLimits":{"memory":"2Gi"}}}}'
```

### "Too many registry requests"

Increase `spec.registry.pollInterval` and verify ETag support:

```bash
# Check if registry supports ETags
curl -I -H "If-None-Match: dummy" \
  https://ghcr.io/v2/myorg/my-app-bundle/tags/list

# If you see ETag headers in response, ETag caching is working
# If not, try a different poll interval
kubectl patch werfbundle my-app -n k8s-werf-operator-go-system --type merge -p '
{"spec":{"registry":{"pollInterval":"30m"}}}'
```

### "Pod fails before getting logs"

Job logs are limited to ~5KB in status; larger logs require direct pod access:

```bash
# Check full pod logs
kubectl logs job/my-app-<hash>-<uuid> -n k8s-werf-operator-go-system --tail=100

# Check pod events for failures
kubectl describe pod <pod-name> -n k8s-werf-operator-go-system
```
