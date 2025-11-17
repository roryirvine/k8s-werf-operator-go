# WerfBundle Troubleshooting Guide

This guide helps diagnose and resolve common issues with the Werf operator.

## Quick Diagnosis

Start here if you're not sure what's wrong.

### Step 1: Check Bundle Status

```bash
kubectl describe werfbundle my-app -n k8s-werf-operator-go-system
```

Look for these key fields:
- **Phase**: `Syncing`, `Synced`, or `Failed`
- **LastAppliedTag**: Last successfully deployed version (empty if never synced)
- **ConsecutiveFailures**: Number of consecutive registry poll failures (0-6)
- **LastErrorMessage**: Description of the most recent error
- **ActiveJobName**: Name of running job, if any

### Step 2: Check Phase Status

| Phase | Meaning | Next Steps |
|-------|---------|-----------|
| `Syncing` | Operator is trying to sync the bundle | Check `ConsecutiveFailures` and `LastErrorMessage` |
| `Synced` | Last deployment succeeded | Check operator logs if no new jobs created |
| `Failed` | Bundle cannot be synced (6+ failures) | Fix root cause, manually edit to reset counter |

### Step 3: Interpret ConsecutiveFailures

```yaml
status:
  consecutiveFailures: 0  # Healthy - polls working
  consecutiveFailures: 3  # Warning - failing but retrying with backoff
  consecutiveFailures: 6  # Critical - marked Failed, stopped retrying
```

If `consecutiveFailures > 5`, the bundle is marked `Failed` and will not retry further. See "Recovering from Failed Status" below.

## Common Issues and Solutions

### Issue: Bundle stuck in "Syncing" for extended time

**Diagnosis**: Check `kubectl describe werfbundle my-app`:

```bash
kubectl describe werfbundle my-app -n k8s-werf-operator-go-system
```

Look for:
- `status.consecutiveFailures: 3` or higher
- Non-empty `lastErrorMessage`
- `phase: Syncing`

**Root Cause**: Registry polling is failing repeatedly. Operator is retrying with exponential backoff.

**Solutions**:

1. **Check registry URL is correct**:
   ```bash
   # Verify URL from the bundle spec
   kubectl get werfbundle my-app -o jsonpath='{.spec.registry.url}' -n k8s-werf-operator-go-system

   # Test connectivity from operator pod
   kubectl run -it --rm debug --image=curlimages/curl -- sh
   # Inside pod:
   curl -v https://ghcr.io/v2/myorg/my-app-bundle/tags/list
   ```

2. **Check credentials (if using private registry)**:
   ```bash
   # Verify secret exists
   kubectl get secret registry-creds -n k8s-werf-operator-go-system

   # Verify secret is valid (contains .dockerconfigjson)
   kubectl get secret registry-creds -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d | jq .
   ```

3. **Check operator logs for detailed error**:
   ```bash
   kubectl logs -n k8s-werf-operator-go-system -l control-plane=controller-manager --tail=50

   # Look for lines mentioning the bundle name, e.g.:
   # "error polling registry: ... context deadline exceeded"
   # "error polling registry: ... 401 unauthorized"
   # "error polling registry: ... 404 not found"
   ```

4. **Verify network access from operator**:
   ```bash
   # Check if operator pod can reach registry
   kubectl exec -it <operator-pod> -n k8s-werf-operator-go-system -- \
     curl -v https://ghcr.io/v2/myorg/my-app-bundle/tags/list
   ```

5. **Check if registry is temporarily down**:
   - Operator will automatically retry with exponential backoff
   - If registry recovers, bundle will automatically sync
   - Current backoff sequence: 30s → 1m → 2m → 4m → 8m

### Issue: Bundle status shows "Failed" (6+ consecutive failures)

**Diagnosis**: Bundle has stopped retrying after 6 consecutive failures.

```bash
kubectl describe werfbundle my-app -n k8s-werf-operator-go-system
# Shows: status.phase: Failed
#        status.consecutiveFailures: 6
```

**Solution**: Fix the root cause and reset the failure counter by editing the bundle:

```bash
# Option 1: Patch the status to reset counter
kubectl patch werfbundle my-app -n k8s-werf-operator-go-system \
  --type merge -p '{"status":{"consecutiveFailures":0}}'

# Option 2: Edit the bundle (any change triggers reconciliation)
kubectl edit werfbundle my-app -n k8s-werf-operator-go-system
# Then save without changes (forces reconciliation)

# Option 3: Delete and recreate (if other fixes applied)
kubectl delete werfbundle my-app -n k8s-werf-operator-go-system
# Then apply corrected manifest
```

After resetting, check logs to verify the bundle begins retrying:

```bash
kubectl logs -f -n k8s-werf-operator-go-system -l control-plane=controller-manager
```

### Issue: Jobs failing with "OOMKilled" status

**Diagnosis**: Job pod is terminated with insufficient memory.

```bash
# Check job status
kubectl get job my-app-<hash>-<uuid> -n k8s-werf-operator-go-system -o wide
# STATUS: Failed, REASON: BackoffLimitExceeded or Pod OOMKilled

# Check pod events
kubectl describe pod <pod-name> -n k8s-werf-operator-go-system
# Look for: "Reason: OOMKilled"

# Check WerfBundle status
kubectl describe werfbundle my-app -n k8s-werf-operator-go-system
# lastJobStatus: Failed
# lastJobLogs may show memory-related errors
```

**Root Cause**: Werf converge process requires more memory than allocated.

**Solution**: Increase memory limits:

```bash
kubectl patch werfbundle my-app -n k8s-werf-operator-go-system --type merge -p \
  '{"spec":{"converge":{"resourceLimits":{"memory":"2Gi"}}}}'
```

**Recommended progression**:
- Default: `1Gi` (small/simple apps)
- If OOMKilled: `2Gi` (medium apps or complex manifests)
- If still OOMKilled: `4Gi` (large apps or heavy builds)

See [Configuration Reference](configuration.md) for detailed recommendations.

### Issue: Too many registry requests / bandwidth concerns

**Diagnosis**: Operator is polling too frequently.

```bash
# Check poll interval
kubectl get werfbundle my-app -o jsonpath='{.spec.registry.pollInterval}' -n k8s-werf-operator-go-system

# Monitor registry requests in operator logs
kubectl logs -n k8s-werf-operator-go-system -l control-plane=controller-manager | grep "polling registry"
```

**Root Cause**: Poll interval is too short, or ETag caching isn't working.

**Solution 1: Increase poll interval**

```bash
kubectl patch werfbundle my-app -n k8s-werf-operator-go-system --type merge -p \
  '{"spec":{"registry":{"pollInterval":"30m"}}}'
```

**Solution 2: Verify ETag support in registry**

ETag caching (HTTP 304 responses) requires registry support:

```bash
# Check if registry supports ETags
curl -I -H "If-None-Match: dummy" \
  https://ghcr.io/v2/myorg/my-app-bundle/tags/list

# Look for these headers in response:
# ETag: "..."
# If missing, registry doesn't support ETags; increase poll interval instead
```

**Impact of ETag caching**:
- With ETag: 304 Not Modified (zero bytes, just headers)
- Without ETag: Full tag list every poll (50-500KB per registry)

For 100 bundles polling every 15m = 576 polls/day. With a 100KB registry response:
- Without ETag: 100KB × 576 = 57.6MB/day
- With ETag: ~1KB × 576 = 0.576MB/day
- Savings: 57MB/day or ~1.7GB/month

### Issue: Job logs not captured in WerfBundle status

**Diagnosis**: `status.lastJobLogs` is empty, but you need to see job output.

```bash
kubectl describe werfbundle my-app -n k8s-werf-operator-go-system
# lastJobLogs: "" (empty)
# lastJobStatus: Failed or Succeeded
```

**Root Cause**: Job logs exceed ~5KB (status field size limit), or pod was terminated before logs were captured.

**Solution**: Check pod logs directly:

```bash
# Find the job name from status
JOB_NAME=$(kubectl get werfbundle my-app -o jsonpath='{.status.activeJobName}' -n k8s-werf-operator-go-system)

# If activeJobName is empty, find recent job:
kubectl get job -n k8s-werf-operator-go-system -l app.kubernetes.io/instance=my-app --sort-by=.metadata.creationTimestamp

# Get pod name from job
POD_NAME=$(kubectl get pods -n k8s-werf-operator-go-system -l job-name=$JOB_NAME -o jsonpath='{.items[0].metadata.name}')

# View full logs
kubectl logs $POD_NAME -n k8s-werf-operator-go-system --tail=200

# Or stream in real-time (if pod is still running)
kubectl logs -f $POD_NAME -n k8s-werf-operator-go-system
```

**Understanding log capture limitations**:
- Status field limited to ~5KB for performance
- Large Werf builds or deployments can produce 100KB+ of logs
- Always check pod logs for complete output

### Issue: New bundle versions not creating jobs

**Diagnosis**: Registry polling works (no errors) but no new jobs created.

```bash
# Check bundle status
kubectl describe werfbundle my-app -n k8s-werf-operator-go-system
# phase: Synced
# lastAppliedTag: v1.2.3
# lastETag: "..." (indicates successful poll)

# Check if new tags exist in registry
curl -s https://ghcr.io/v2/myorg/my-app-bundle/tags/list | jq '.tags'
# Should see new tags like v1.3.0, v2.0.0, etc.

# Check jobs
kubectl get job -n k8s-werf-operator-go-system -l app.kubernetes.io/instance=my-app
# Should show jobs with different tags
```

**Root Cause**: Bundle is only deploying latest tag (by lexicographic sort), but you expect all new tags.

**Current Behavior**: Operator creates a job only for the most recent tag (alphabetically), not for every tag change.

**Expected Behavior in Slice 5**: Semantic versioning support will allow filtering tags by version constraint (e.g., `>=1.0.0,<2.0.0`).

**Current Workaround**:
- Use consistent tag naming that sorts correctly (e.g., `v1.0.0`, `v1.1.0`, etc.)
- Avoid tags like `latest`, `stable`, or `release` unless they're actually the newest version

### Issue: ServiceAccount not found error

**Diagnosis**: Job fails because target namespace ServiceAccount doesn't exist.

```bash
kubectl describe werfbundle my-app -n k8s-werf-operator-go-system
# lastErrorMessage: "ServiceAccount werf-converge does not exist in..."
# phase: Failed

# Or check job logs
kubectl logs job/<job-name> -n k8s-werf-operator-go-system
# Error: could not auth to cluster
```

**Root Cause**: `spec.converge.serviceAccountName` references a ServiceAccount that doesn't exist in the target namespace.

**Solution**: Create the ServiceAccount (see [Configuration Reference](configuration.md#serviceAccountName) for setup instructions).

```bash
# Create namespace
kubectl create namespace production

# Create ServiceAccount
kubectl create serviceaccount werf-converge -n production

# Create Role with necessary permissions
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

After creating ServiceAccount, edit the bundle to trigger reconciliation:

```bash
kubectl patch werfbundle my-app -n k8s-werf-operator-go-system --type merge -p \
  '{"spec":{"converge":{"serviceAccountName":"werf-converge"}}}'
```

## Understanding Status Fields

These fields in `kubectl describe werfbundle` help diagnose issues:

| Field | Type | Meaning |
|-------|------|---------|
| `phase` | String | Current state: `Syncing`, `Synced`, or `Failed` |
| `lastAppliedTag` | String | Last successfully deployed tag (empty if never synced) |
| `lastSyncTime` | Timestamp | When last successful deployment occurred |
| `lastErrorMessage` | String | Description of most recent error (if any) |
| `lastETag` | String | HTTP ETag from last registry response (for caching) |
| `consecutiveFailures` | Integer 0-6 | Count of consecutive failures; 6+ triggers backoff pause |
| `lastErrorTime` | Timestamp | When last error occurred (used for backoff calculation) |
| `activeJobName` | String | Name of currently running Job (for deduplication) |
| `lastJobStatus` | String | Status of most recent Job: `Running`, `Succeeded`, `Failed` |
| `lastJobLogs` | String | Last ~5KB of job output (check pod logs for full output) |

## Advanced Debugging

### Monitoring backoff progression

When registry polling fails, exponential backoff delays retries. Monitor this progression:

```bash
# Watch status fields change
watch kubectl describe werfbundle my-app -n k8s-werf-operator-go-system

# Or check status programmatically
kubectl get werfbundle my-app -o jsonpath='{.status.consecutiveFailures},{.status.lastErrorTime}' \
  -n k8s-werf-operator-go-system
```

Expected progression (each failure increases counter):
- 1st failure: consecutiveFailures=1, requeue after 30s
- 2nd failure: consecutiveFailures=2, requeue after 1m
- 3rd failure: consecutiveFailures=3, requeue after 2m
- 4th failure: consecutiveFailures=4, requeue after 4m
- 5th failure: consecutiveFailures=5, requeue after 8m
- 6th failure: consecutiveFailures=6, phase=Failed (stopped retrying)

### Checking ETag caching effectiveness

Verify that ETag caching is working (preventing unnecessary registry requests):

```bash
# Look for "304 Not Modified" responses in operator logs
kubectl logs -n k8s-werf-operator-go-system -l control-plane=controller-manager | \
  grep -i "etag\|304\|not modified"

# Or check the lastETag field
kubectl get werfbundle my-app -o jsonpath='{.status.lastETag}' -n k8s-werf-operator-go-system
# Non-empty value indicates ETag is being used

# Verify ETag changes only when tags change
kubectl get werfbundle my-app -o jsonpath='{.status.lastETag}' -n k8s-werf-operator-go-system > /tmp/etag1
# Wait a bit, then check again
kubectl get werfbundle my-app -o jsonpath='{.status.lastETag}' -n k8s-werf-operator-go-system > /tmp/etag2
diff /tmp/etag1 /tmp/etag2
# If no diff: ETag hasn't changed (registry content unchanged, caching working)
# If diff: Registry content changed (or new ETag generated for other reasons)
```

### Checking operator resource usage

Monitor if operator pod itself is having issues:

```bash
# Check operator pod status
kubectl get pods -n k8s-werf-operator-go-system

# Check operator logs for errors
kubectl logs -n k8s-werf-operator-go-system -l control-plane=controller-manager --tail=200

# Check operator pod resource usage
kubectl top pod -n k8s-werf-operator-go-system -l control-plane=controller-manager

# If pod keeps restarting, check events
kubectl describe pod <operator-pod> -n k8s-werf-operator-go-system
```

### Debugging failed job pods

When a Werf converge job fails:

```bash
# Find the failed pod
kubectl get pods -n k8s-werf-operator-go-system -l app.kubernetes.io/instance=my-app

# Check pod description for events
kubectl describe pod <pod-name> -n k8s-werf-operator-go-system
# Look for: Reason, Message, events (Last State, Reason, Message)

# Check container logs
kubectl logs <pod-name> -n k8s-werf-operator-go-system -c werf

# If pod was OOMKilled, check previous logs
kubectl logs <pod-name> -n k8s-werf-operator-go-system -c werf --previous

# Check resource usage at time of failure
kubectl describe pod <pod-name> -n k8s-werf-operator-go-system | grep -A 20 "Containers"
```

## Getting Help

If you're stuck, gather this information for debugging:

1. **Bundle status**:
   ```bash
   kubectl describe werfbundle my-app -n k8s-werf-operator-go-system
   ```

2. **Recent operator logs**:
   ```bash
   kubectl logs -n k8s-werf-operator-go-system -l control-plane=controller-manager --tail=100
   ```

3. **Recent job status**:
   ```bash
   kubectl get job -n k8s-werf-operator-go-system -l app.kubernetes.io/instance=my-app -o wide
   ```

4. **Failed job logs** (if applicable):
   ```bash
   kubectl logs job/<job-name> -n k8s-werf-operator-go-system
   ```

5. **Configuration**:
   ```bash
   kubectl get werfbundle my-app -o yaml -n k8s-werf-operator-go-system
   ```

This information helps diagnose nearly all issues.

## Common Error Messages

| Error | Meaning | Fix |
|-------|---------|-----|
| "error polling registry: context deadline exceeded" | Registry not responding within timeout | Check registry health, network connectivity |
| "error polling registry: 401 unauthorized" | Registry credentials invalid or missing | Check registry secret, verify token is valid |
| "error polling registry: 404 not found" | Registry URL incorrect or repository doesn't exist | Verify registry URL and repository name |
| "ServiceAccount ... does not exist" | Target namespace ServiceAccount not found | Create ServiceAccount with proper RBAC |
| "pod failed with OOMKilled" | Job ran out of memory | Increase `resourceLimits.memory` |
| "pod failed with exit code X" | Werf converge process failed | Check pod logs for Werf error details |
| "ETag support not detected" | Registry doesn't return ETag headers | Increase poll interval or try different registry |

## Still Need Help?

- Check [Configuration Reference](configuration.md) for detailed field explanations
- Review [DESIGN.md](DESIGN.md) for architecture and design decisions
- Check [PLAN.md](PLAN.md) for roadmap and known limitations
