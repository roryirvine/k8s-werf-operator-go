# Implementation Plan

## Slice 1: Basic Bundle Deployment
Implement core CRD and simple bundle deployment without advanced features.

---

## Task 1: Project Setup

**The problem we're solving**: We need a well-structured Kubernetes operator project with proper scaffolding, testing infrastructure, and CI/CD pipelines. Starting with the right foundation prevents technical debt and ensures consistency.

**Why Kubebuilder**:
- Kubebuilder is the standard scaffolding tool for Go operators - it generates boilerplate for CRDs, controllers, webhooks, and RBAC
- It follows Kubernetes API conventions automatically (versioning, status subresources, printer columns)
- Trade-off: More generated code means more to understand initially, but this is far better than hand-rolling everything and missing crucial patterns
- Operational benefit: New team members familiar with Kubebuilder can navigate the codebase immediately

**Architecture decisions**:
  1. **Use kubebuilder v3+ with plugins**: Scaffolds project with modern controller-runtime patterns
    - Why: Gives us manager lifecycle, event filtering, and predicate support out of the box
    - Alternative: Writing operator from scratch would take weeks and miss edge cases like leader election and graceful shutdown

  2. **Single manager, single namespace deployment initially**: Operator watches one namespace for WerfBundle resources
    - Why: Simpler RBAC setup, easier to reason about permissions, matches how most operators start
    - Alternative: Cluster-scoped watching requires ClusterRole and more complex security model - we can add this later in Phase 2

  3. **GitHub Actions for CI**: Run tests, linting, and builds on every PR
    - Why: Free for public repos, integrates natively with GitHub, good Action marketplace for Go tools
    - Alternative: Could use GitLab CI or Jenkins, but we're already on GitHub and Actions has excellent Go support

**Testing strategy**:
Test these scenarios:
  1. `kubebuilder init` creates valid module → Proves project structure is correct and dependencies resolve
  2. `make manifests` generates CRD YAML → Proves kubebuilder tooling works and CRDs will be valid
  3. `make test` runs without errors → Proves test infrastructure is configured correctly
  4. GitHub Actions workflow completes successfully → Proves CI pipeline can build and test the operator
  5. Devcontainer build succeeds and tools are available → Proves development environment is consistent

**Implementation approach**:
You need 4 components:

  1. **Kubebuilder initialization**: Initialize the Go module and operator project structure
    - Run `kubebuilder init --domain werf.io --repo github.com/<org>/k8s-werf-operator-go`
    - This creates the main.go, Makefile, Dockerfile, and manager scaffolding
    - The domain becomes the API group for your CRDs (werf.io/v1alpha1)

  2. **Makefile extensions**: Add custom targets for linting and local testing
    - Add `make lint` target that runs `golangci-lint` with strict settings
    - Add `make local-test` that runs integration tests against kind cluster
    - These extend the generated Makefile without modifying kubebuilder's targets

  3. **GitHub Actions workflow**: Create .github/workflows/ci.yml
    - Run on pull_request and push to main
    - Steps: checkout, setup Go, install dependencies, run linting, run tests, build binary
    - Cache Go modules to speed up builds

  4. **Devcontainer validation**: Verify the existing .devcontainer setup
    - Check that kubebuilder, kubectl, and Go tools are in PATH
    - Document any missing tools that need to be added to devcontainer.json
    - Test that the container can build and run the operator locally

**Key Go patterns**:
- **Go modules**: Use `go mod tidy` regularly to keep dependencies clean - kubebuilder adds many deps
- **Makefile conventions**: Kubebuilder's Makefile uses `controller-gen` for code generation - understand the `manifests` and `generate` targets
- **Project layout**: Follow standard Go project structure with `api/`, `controllers/`, `internal/` directories - this makes the codebase navigable

**What success looks like**:
- Running `make test` passes with no failures
- `make manifests` generates valid CRD YAML in config/crd/bases/
- GitHub Actions workflow is green on a test commit
- You can explain why kubebuilder generates both a types.go and a groupversion_info.go file
- You understand the difference between `make install` (installs CRD) and `make deploy` (deploys operator)

---

## Task 2: CRD Implementation

**The problem we're solving**: We need to define the WerfBundle resource that users will create in their clusters. This CRD must capture all necessary configuration for pulling bundles from OCI registries and deploying them with Werf. A well-designed CRD is the contract between the operator and its users.

**Why embed auth reference instead of inline credentials**:
- Secrets should never be in the spec directly - they'd be visible in kubectl output and audit logs
- SecretRef pattern (referencing Secret by name) is standard Kubernetes practice used by Pod, ServiceAccount, etc.
- Trade-off: Requires users to create Secret separately, but this is correct separation of concerns
- Operational benefit: Secrets can be rotated without modifying WerfBundle resource

**Architecture decisions**:
  1. **Use v1alpha1 for initial version**: Signal that API is not stable yet
    - Why: Gives us freedom to change fields without breaking compatibility promises
    - Alternative: Starting with v1 would lock us into the initial design - we can promote to v1beta1 later

  2. **Separate Spec and Status**: Spec is desired state (user input), Status is observed state (operator output)
    - Why: This is fundamental Kubernetes pattern - Status subresource enables optimistic concurrency
    - Alternative: Putting everything in Spec would break controller patterns and cause conflicts

  3. **Validation via markers, not webhook initially**: Use kubebuilder validation markers (// +kubebuilder:validation:...)
    - Why: Markers generate OpenAPI validation in CRD, validated by API server automatically - no webhook needed
    - Alternative: Validation webhook adds complexity and another point of failure - save for cross-field validation later

  4. **Flat spec structure for MVP**: Keep registry config and converge config as direct fields
    - Why: Simpler to implement and understand, fewer nil checks, clearer what's required
    - Alternative: Deep nesting (spec.config.registry.url) is harder to work with and requires more boilerplate

**Testing strategy**:
Test these scenarios:
  1. Valid WerfBundle YAML applies successfully → Proves CRD validation accepts correct input
  2. Missing required field (e.g., registry.url) is rejected → Proves required field validation works
  3. Invalid pollInterval format rejected → Proves type validation works for duration fields
  4. Status subresource can be updated independently → Proves status is properly configured as subresource
  5. `kubectl get werfbundles` shows useful columns → Proves printer columns are defined
  6. Generated Go types have proper JSON tags → Proves kubebuilder markers are correct

**Implementation approach**:
You need 3 components:

  1. **API types definition**: Create api/v1alpha1/werfbundle_types.go
    - Define WerfBundleSpec struct with registry and converge fields
    - Define WerfBundleStatus struct with phase, lastAppliedTag, error fields
    - Add kubebuilder markers for validation (Pattern, Minimum, Required, Optional)
    - Add printer columns marker for kubectl output (show phase and lastAppliedTag)

  2. **Group version info**: Ensure api/v1alpha1/groupversion_info.go is correct
    - This is generated by kubebuilder but verify GroupVersion constant matches
    - The SchemeBuilder is what registers types with the Kubernetes scheme

  3. **Manifest generation**: Run `make manifests` to generate CRD YAML
    - Output goes to config/crd/bases/werf.io_werfbundles.yaml
    - Inspect generated YAML to verify validation rules appear in OpenAPI schema
    - Check that status is marked as a subresource in the CRD

**Key Go patterns**:
- **Struct tags**: Use `json:"fieldName,omitempty"` tags - this is how Go structs map to Kubernetes API fields
- **Pointer vs value fields**: Use pointers for optional fields (*string, *int32) - nil means unset, different from zero value
- **Kubebuilder markers**: Comments starting with `// +kubebuilder:` are parsed by controller-gen to generate CRD YAML
- **metav1.TypeMeta and ObjectMeta**: Every Kubernetes resource embeds these - they provide apiVersion, kind, name, namespace
- **Status subresource**: The `// +kubebuilder:subresource:status` marker makes status a separate subresource - critical for proper controller behavior

**What success looks like**:
- `make manifests` generates CRD without errors
- The generated CRD has proper OpenAPI validation schema
- You can create a WerfBundle resource and see it with `kubectl get werfbundles`
- You can explain why PollInterval is a string field (metav1.Duration) instead of time.Duration
- You understand why we embed metav1.TypeMeta and metav1.ObjectMeta in WerfBundle struct
- Invalid resources are rejected by the API server with clear error messages

---

## Task 3: Basic Controller

**The problem we're solving**: We need a reconciliation loop that watches WerfBundle resources, polls OCI registries for new tags, and creates Kubernetes Jobs to run `werf converge` when updates are found. This is the core operator logic that makes the declarative API actually do something.

**Why reconciliation loop over event handlers**:
- Reconciliation is level-triggered (look at current state) not edge-triggered (react to events) - handles missed events gracefully
- Controller-runtime's Reconcile() is called for any change or resync, giving one place to implement logic
- Trade-off: Reconcile is called frequently so must be idempotent and fast - but this forces good design
- Operational benefit: Operator self-heals - if external state changes, next reconciliation corrects it

**Architecture decisions**:
  1. **Store last-seen tag in Status, not in-memory cache**: Use WerfBundle.Status.LastAppliedTag
    - Why: Status is persistent across restarts, survives leader election changes, visible to users
    - Alternative: In-memory cache would lose state on pod restart and require complex synchronization

  2. **Use Job for converge, not exec in existing pod**: Create a new Kubernetes Job to run `werf converge`
    - Why: Jobs handle retries, logs, resource limits, and cleanup automatically
    - Alternative: Running werf in operator pod would require werf binary in operator image and break security boundaries

  3. **Poll on reconcile + periodic resync**: Check registry whenever Reconcile() is called
    - Why: Controller-runtime automatically requeues resources periodically - no need for separate goroutines
    - Alternative: Background goroutines per resource would require complex lifecycle management

  4. **Single registry client instance**: Create OCI registry client once in controller SetupWithManager
    - Why: Client can maintain connection pools and rate limiters across reconciliations
    - Alternative: Creating client per reconciliation would be wasteful and break rate limiting

  5. **Defer polling interval logic to later**: For MVP, reconcile on every change and rely on controller resync
    - Why: Simpler to implement, still functional, PollInterval enforcement can be added in Slice 2
    - Alternative: Implementing proper interval tracking in MVP would delay first working version

**Testing strategy**:
Test these scenarios:
  1. WerfBundle created triggers reconciliation → Proves controller is watching resources
  2. New tag in registry triggers Job creation → Proves polling and job creation logic works
  3. Same tag polled twice doesn't create duplicate Job → Proves idempotency and status tracking works
  4. Job completion updates WerfBundle status to Synced → Proves job monitoring works
  5. Job failure updates status to Failed with error message → Proves error handling and status updates work
  6. Invalid registry URL sets status to Failed → Proves error conditions are captured
  7. Reconcile completes in under 10 seconds → Proves reconciliation is fast enough

**Implementation approach**:
You need 5 components:

  1. **WerfBundleReconciler struct**: Define in controllers/werfbundle_controller.go
    - Embed client.Client for Kubernetes API access
    - Hold reference to OCI registry client
    - Hold reference to scheme for object creation
    - Optionally hold reference to recorder for events

  2. **Reconcile() method**: Implements the main reconciliation loop
    - Fetch the WerfBundle resource by namespaced name
    - Handle deletion (return early if not found)
    - Call registry client to get latest tag matching constraints
    - Compare with Status.LastAppliedTag
    - If different, create Job and update status to Syncing
    - If same, ensure Job completed successfully and status is Synced
    - Return ctrl.Result with requeue time

  3. **Registry client**: Create internal/registry/client.go
    - Method: ListTags(ctx, repoURL, authSecret) returns []string
    - Method: GetLatestTag(ctx, repoURL, constraint, authSecret) returns string
    - Use github.com/google/go-containerregistry for OCI operations
    - Handle authentication by reading Secret referenced in spec.registry.secretRef

  4. **Job creator**: Create internal/converge/job.go
    - Method: CreateConvergeJob(ctx, bundle, tag) returns *batchv1.Job
    - Build Job spec with werf container, mount bundle config, set resource limits
    - Set owner reference so Job is deleted when WerfBundle is deleted
    - Job name should be deterministic based on bundle name and tag

  5. **Status updater**: Add methods to WerfBundleReconciler
    - Method: updateStatusSyncing(ctx, bundle, tag) updates phase and timestamp
    - Method: updateStatusSynced(ctx, bundle, tag) updates phase and lastAppliedTag
    - Method: updateStatusFailed(ctx, bundle, err) updates phase and error message
    - Use r.Status().Update(ctx, bundle) to persist changes

**Key Go patterns**:
- **Context passing**: Every Kubernetes API call takes context.Context - this enables timeouts and cancellation
- **Error wrapping**: Use `fmt.Errorf("failed to X: %w", err)` to add context while preserving original error
- **ctrl.Result**: Return value from Reconcile() - RequeueAfter schedules next reconciliation
- **Requeue vs RequeueAfter**: Requeue retries immediately (use for transient errors), RequeueAfter schedules specific time
- **Owner references**: SetControllerReference() makes child resources (Jobs) get deleted when parent (WerfBundle) is deleted
- **Predicate filtering**: Use predicates in SetupWithManager to filter which events trigger reconciliation

**What success looks like**:
- WerfBundle creation triggers reconciliation and you can see log lines
- Registry is polled and latest tag is detected
- Job is created successfully and appears in `kubectl get jobs`
- Job completion updates WerfBundle status correctly
- You can explain why we use `client.Client` instead of direct clientset
- You understand the difference between r.Client.Get() and r.Client.List()
- You can describe what happens if two reconciliations run concurrently for the same resource

---

## Task 4: RBAC Setup

**The problem we're solving**: Kubernetes operators need carefully scoped permissions to do their job without excessive privileges. We need two separate RBAC configurations: one for the operator pod itself (minimal permissions) and one for the Jobs that run werf converge (broad permissions in target namespace).

**Why split operator and Job permissions**:
- Operator only needs to watch WerfBundles and create Jobs - giving it full namespace access violates least privilege
- Werf converge needs broad permissions (create/update/delete apps) but only in target namespace, not operator namespace
- Trade-off: Requires pre-configuring ServiceAccounts in target namespaces, but this is correct security model
- Operational benefit: If operator is compromised, attacker can't directly modify application resources

**Architecture decisions**:
  1. **Operator uses ClusterRole for CRD access**: Operator needs cluster-wide view of WerfBundle CRD
    - Why: CRDs are cluster-scoped by definition, and WerfBundles might be in multiple namespaces
    - Alternative: Namespace-scoped Role wouldn't allow operator to watch CRDs

  2. **Job uses namespace-scoped Role**: Each target namespace has its own werf-converge Role
    - Why: Limits blast radius - compromised Job in namespace A can't affect namespace B
    - Alternative: ClusterRole for Jobs would give every converge job cluster admin, which is excessive

  3. **Operator validates ServiceAccount exists**: Before creating Job, check that spec.converge.serviceAccountName exists
    - Why: Fail fast with clear error instead of letting Job fail with obscure auth error
    - Alternative: Letting Job fail would work but harder to debug and slower feedback

  4. **Document but don't create Job ServiceAccounts**: Operator doesn't auto-create werf-converge ServiceAccounts
    - Why: Users should explicitly grant permissions - auto-creating would be magic that hides security implications
    - Alternative: Auto-creating would be convenient but makes it unclear who authorized what permissions

**Testing strategy**:
Test these scenarios:
  1. Operator pod starts successfully with minimal ClusterRole → Proves RBAC is configured correctly
  2. Operator can read WerfBundle resources → Proves CRD permissions work
  3. Operator can create Jobs → Proves Job creation permissions work
  4. Operator cannot create Deployments directly → Proves we haven't over-granted permissions
  5. Job with valid ServiceAccount can run werf converge → Proves Job RBAC works
  6. Job without ServiceAccount fails with clear error → Proves validation works
  7. Reconcile fails gracefully when ServiceAccount doesn't exist → Proves validation prevents Job creation

**Implementation approach**:
You need 4 components:

  1. **Operator ClusterRole definition**: Create config/rbac/role.yaml (kubebuilder generates base)
    - Add markers in controller: `// +kubebuilder:rbac:groups=werf.io,resources=werfbundles,verbs=get;list;watch`
    - Add markers for status: `// +kubebuilder:rbac:groups=werf.io,resources=werfbundles/status,verbs=get;update;patch`
    - Add markers for Jobs: `// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=create;get;list;watch;delete`
    - Add markers for Secrets: `// +kubebuilder:rbac:groups="",resources=secrets,verbs=get`
    - Run `make manifests` to generate YAML

  2. **Job ServiceAccount documentation**: Create docs/job-rbac.md
    - Provide example Role YAML for werf-converge ServiceAccount
    - Explain what permissions werf needs (create/update/delete apps, configmaps, secrets, services, etc.)
    - Include example RoleBinding to bind Role to ServiceAccount
    - Document that this must be created in each target namespace

  3. **ServiceAccount validation**: Add to WerfBundleReconciler.Reconcile()
    - Before creating Job, call r.Client.Get() to fetch ServiceAccount in target namespace
    - If not found, update status to Failed with message "ServiceAccount X not found in namespace Y"
    - Return without creating Job
    - This prevents cryptic authentication errors later

  4. **RBAC testing utilities**: Add to internal/testing/rbac.go
    - Helper function: CreateTestServiceAccount(client, namespace, name) for tests
    - Helper function: CreateTestRole(client, namespace, rules) for tests
    - Use these in integration tests to set up proper RBAC

**Key Go patterns**:
- **Kubebuilder RBAC markers**: `// +kubebuilder:rbac:` comments generate RBAC YAML - keep them above controller struct
- **ServiceAccount defaulting**: Kubernetes automatically mounts ServiceAccount token if pod doesn't specify one
- **RBAC checking**: Use r.Client.Get() with ServiceAccount type to check existence - don't use SubjectAccessReview (too complex)
- **Error messages**: Make RBAC errors explicit - say "ServiceAccount not found" not just "authentication failed"

**What success looks like**:
- `make manifests` generates valid ClusterRole YAML
- Operator deploys successfully with generated RBAC
- You can create a test ServiceAccount and successfully create Jobs
- You can explain why operator uses ClusterRole but Jobs use Role
- You understand what ownerReferences have to do with RBAC (nothing - they're separate concepts)
- Error messages clearly indicate RBAC problems vs other failures

---

## Task 5: Testing

**The problem we're solving**: We need comprehensive tests that prove each component works in isolation (unit tests), works with real Kubernetes API (integration tests), and works end-to-end (E2E tests). Good tests document expected behavior and catch regressions early.

**Why three test levels**:
- Unit tests are fast and test logic without Kubernetes - run in milliseconds, no cluster needed
- Integration tests use envtest (real API server) to test controller behavior - catch client bugs, still run in CI
- E2E tests use real cluster to test operator orchestration on real Kubernetes - catch RBAC, deployment, GC issues, run slower
- Trade-off: E2E tests are slower and more brittle, but they catch issues that unit/integration tests miss
- Operational benefit: Fast feedback loop (unit) plus confidence in real-world behavior (E2E)

**Architecture decisions**:
  1. **Use envtest for integration tests**: Controller-runtime's envtest runs real API server in-process
    - Why: Tests interact with real Kubernetes API, not mocks - catches serialization bugs, validation issues
    - Alternative: Mocking client would be faster but wouldn't catch API compatibility issues

  2. **Use table-driven tests**: Define test cases as structs with inputs and expected outputs
    - Why: Adding new test cases is trivial, easy to see coverage gaps, reduces boilerplate
    - Alternative: Individual test functions for each case leads to lots of copy-paste

  3. **Test Job creation, not Job execution**: Mock or skip actual werf converge execution in tests
    - Why: We're testing operator logic, not werf itself - werf tests belong in werf project
    - Alternative: Running real werf in tests would be slow and require complex setup

  4. **Use fake registry for Slice 1 integration tests**: Create simple in-memory fake that implements registry interface
    - Why: Slice 1 tests controller logic, not OCI protocol - fake is sufficient and avoids Docker-in-Docker setup
    - Alternative: Real registry would catch OCI bugs but adds complexity unnecessary for proving controller works
    - Note: Real registry testing is deferred to Slice 2 (Enhanced Registry Integration)

  5. **E2E tests verify orchestration, not successful deployments**: Test that Jobs are created and status updates work, not that werf converge succeeds
    - Why: In Slice 1 we don't have real registries or werf bundles - Jobs will fail but that's expected
    - What we prove: Operator deploys to real cluster, RBAC enforcement works, Job controller interaction works, garbage collection works
    - What we don't prove: Successful werf converge, real registry auth, actual application deployment (deferred to Slice 2)
    - Alternative: Setting up successful deployments requires real registry setup and werf bundles - out of scope for Slice 1

  6. **Separate test fixtures directory**: Create test/fixtures/ with sample WerfBundle YAML
    - Why: Keeps test data separate from test code, easy to add new fixtures, more maintainable
    - Alternative: Embedding YAML in test code makes tests harder to read

**Testing strategy**:
Test these scenarios:

**Unit Tests** (internal/registry, internal/converge):
  1. ListTags returns tags sorted by semver → Proves tag parsing works
  2. GetLatestTag with version constraint filters correctly → Proves constraint logic works
  3. CreateConvergeJob generates valid Job spec → Proves Job builder works
  4. CreateConvergeJob sets owner reference → Proves Job lifecycle is tied to WerfBundle
  5. Status update methods modify correct fields → Proves status logic works

**Integration Tests** (controllers):
  1. Reconcile creates Job when new tag found → Proves full reconciliation loop works
  2. Reconcile doesn't create duplicate Jobs → Proves idempotency works
  3. Job completion updates status to Synced → Proves job monitoring works
  4. Missing ServiceAccount prevents Job creation → Proves validation works
  5. Invalid registry URL updates status to Failed → Proves error handling works
  6. WerfBundle deletion deletes owned Jobs → Proves owner reference cleanup works

**E2E Tests** (test/e2e):
Note: These tests verify operator orchestration on real cluster, not successful werf converge (no real registry/bundles in Slice 1)

  1. Missing ServiceAccount → Job creation fails, WerfBundle status = Failed → Proves RBAC enforcement works on real cluster
  2. Valid ServiceAccount, invalid registry → Job created, Pod starts, Job fails, WerfBundle status = Failed → Proves Job orchestration and status watching works
  3. Delete WerfBundle → Job is garbage collected → Proves owner references work on real cluster

**Implementation approach**:
You need 5 components:

  1. **Unit test files**: Create *_test.go files next to implementation files
    - Use standard Go testing package with table-driven tests
    - Define test cases as []struct{name, input, want, wantErr}
    - Use t.Run(tc.name, func(t *testing.T) { ... }) for subtests
    - Mock external dependencies (Kubernetes client, registry client) using interfaces

  2. **Integration test suite**: Create controllers/suite_test.go
    - Use envtest to start real API server: testEnv = &envtest.Environment{...}
    - In TestMain, start testEnv.Start() and defer testEnv.Stop()
    - Create test client: k8sClient, _ = client.New(cfg, ...)
    - Register WerfBundle scheme: scheme.AddToScheme(scheme.Scheme)

  3. **Integration test cases**: Create controllers/werfbundle_controller_test.go
    - Use Ginkgo/Gomega (kubebuilder default) or standard testing
    - Create test WerfBundle using k8sClient.Create()
    - Trigger reconciliation by calling reconciler.Reconcile()
    - Assert on WerfBundle status and created Jobs using k8sClient.Get()
    - Use Eventually() for asynchronous operations

  4. **E2E test harness**: Create test/e2e/e2e_test.go
    - Requires real cluster (kind, minikube, or existing cluster)
    - Use kubectl or client-go to create resources
    - Use wait loops to check for expected state
    - Clean up resources in defer or AfterEach

  5. **Test fixtures and helpers**: Create test/fixtures/ and internal/testing/
    - fixtures/werfbundle.yaml: Example WerfBundle resource
    - testing/helpers.go: Functions like CreateTestWerfBundle(), WaitForJob()
    - testing/fakeregistry.go: In-memory fake registry that returns predefined tags and can simulate errors

**Key Go patterns**:
- **Table-driven tests**: `tests := []struct{name string; input X; want Y}` - iterate and t.Run each
- **Test helpers**: Functions that return (*T, cleanup func()) - caller defers cleanup
- **Eventually loops**: Use `gomega.Eventually()` or manual loop with timeout for async checks
- **Test fixtures**: Use `testdata/` or `test/fixtures/` directory - ignored by go build
- **Interfaces for mocking**: Define interfaces like RegistryClient even if only one impl - enables mocking in tests

**What success looks like**:
- `make test` runs all unit and integration tests successfully
- Test coverage is above 80% for controller and internal packages
- E2E test verifies operator orchestration on real cluster (Job creation, RBAC enforcement, garbage collection)
- You can explain why E2E tests in Slice 1 test failure paths (no real registry) vs success paths (deferred to Slice 2)
- You can explain the difference between using envtest vs a real cluster
- You understand why we don't mock the Kubernetes client in integration tests
- Tests fail with clear error messages that indicate what went wrong
- You can add a new test case without copying lots of boilerplate

---

## Task 6: User Documentation

**The problem we're solving**: Engineers need to understand what the operator does, how to test it works, and what's coming next - without reading design docs or source code. The README is the first (and often only) documentation people read, so it must be clear, practical, and focused on outcomes.

**Why focus on testable outcomes**:
- Users care about "what can I do with this?" not "how is it architected?"
- Runnable examples prove the software works and serve as implicit tests
- Focusing on current capabilities sets clear expectations (this is alpha/beta/stable)
- Next steps create transparency about roadmap without promising timelines

**Architecture decisions**:
  1. **README is for users, not developers**: Developers read DESIGN.md and PLAN.md in .notes/
    - Why: Different audiences need different information - mixing them makes both harder to use
    - Alternative: Single comprehensive README would be overwhelming and hard to maintain

  2. **Show working examples, don't just describe**: Include actual YAML that can be applied
    - Why: Users can copy-paste to get started, examples serve as documentation and validation
    - Alternative: Prose descriptions are less clear and can drift from reality

  3. **Link to detailed docs, don't duplicate**: README links to docs/ for RBAC setup, etc.
    - Why: Single source of truth, README stays concise, detailed docs can be comprehensive
    - Alternative: Everything in README makes it too long, duplicating makes maintenance harder

**Testing strategy**:
Test these scenarios:
  1. Fresh clone, follow README, E2E test passes → Proves instructions are complete and correct
  2. README examples apply without errors → Proves YAML examples are valid
  3. Links in README resolve to existing files → Proves documentation structure is consistent
  4. "What works now" section matches passing tests → Proves we're honest about capabilities

**Implementation approach**:
You need 3 components:

  1. **README.md structure**: Create/update root README.md with sections
    - **Brief overview**: 2-3 sentences explaining what Werf Operator does
    - **Current status**: What phase/slice is complete, what actually works
    - **Quick start**: Minimal steps to deploy operator and create test WerfBundle
    - **Running tests**: How to run E2E tests, what to expect
    - **Documentation**: Links to docs/ directory files (RBAC setup, etc.)
    - **Next steps**: What features are planned next (link to Slice 2+ overview)
    - **Contributing**: Point to development docs if they exist

  2. **Example YAML snippets**: Include in README or examples/ directory
    - Minimal WerfBundle example that actually works
    - ServiceAccount/Role/RoleBinding for target namespace
    - Keep examples realistic but simple (real registry, not fake URLs)

  3. **E2E test instructions**: Step-by-step for running full test
    - Prerequisites (kind/minikube, kubectl, make)
    - Commands to run (make deploy, make test-e2e, etc.)
    - What success looks like (operator deployed, Job created, status updated to Failed - note: Job failure is expected in Slice 1 since we don't have real bundles)
    - How to clean up

**Key patterns**:
- **Imperative voice**: "Run `make deploy`" not "You can run `make deploy`"
- **Verify steps**: After each major step, show how to verify it worked
- **Real examples**: Use github.com/werf/website-bundle or similar real bundle, not example.com
- **Version clarity**: State "This is alpha software" or "v1alpha1 API" so expectations are clear

**What success looks like**:
- Someone unfamiliar with the project can clone and run E2E tests following only README
- README accurately reflects what works (no "TODO" features listed as working)
- README clearly states that Slice 1 E2E tests verify orchestration, not successful deployments (Jobs will fail - this is expected)
- Links to docs/ files are all valid
- Examples can be copy-pasted and applied successfully
- Next steps section gives clear picture of roadmap without over-promising
- You can explain why README doesn't mention controller-runtime or kubebuilder (implementation details)

## Slice 2: Enhanced Registry Integration
Add robust registry handling and job management.

---

## Task 1: Registry Client Enhancements

**The problem we're solving**: The basic registry polling from Slice 1 is functional but naive - it polls on every reconciliation regardless of whether content changed, doesn't handle transient failures gracefully, and can create thundering herd problems if many bundles poll simultaneously. We need production-grade polling with caching, backoff, and jitter.

**Why ETag caching matters**:
- ETags let registries tell us "nothing changed" without transferring data - saves bandwidth and registry load
- Conditional requests (If-None-Match) return 304 Not Modified instantly if content unchanged
- Trade-off: Must store ETag per bundle, but this is just a string in Status
- Operational benefit: Registry bills often include bandwidth charges - caching reduces costs

**Architecture decisions**:
  1. **Store ETag in WerfBundle Status**: Add `lastETag` field to status
    - Why: Persists across restarts, visible for debugging, no separate cache storage needed
    - Alternative: In-memory cache would be lost on restart and require synchronization

  2. **Exponential backoff with max retries**: Start at 30s, double each retry, max 5 retries
    - Why: Gives transient issues time to resolve without hammering failing registries
    - Alternative: Fixed interval retry would overwhelm failing systems or waste time on permanent failures
    - Max retries prevents infinite retry loops - after 5 failures, mark bundle as Failed

  3. **Jitter for poll intervals**: Add random 0-20% jitter to configured pollInterval
    - Why: Prevents all bundles polling simultaneously (thundering herd), spreads load
    - Alternative: Fixed intervals would cause spikes every N minutes as all bundles poll together
    - Example: 15min interval becomes 12-18min randomly per bundle

  4. **Separate retry state from poll interval**: Track retry attempts in Status separate from normal polls
    - Why: Retry after failure is different concern than scheduled polling - don't conflate them
    - Alternative: Using same timer would make logic complex and harder to reason about

**Testing strategy**:
Test these scenarios:
  1. Registry returns 304 Not Modified with valid ETag → Proves caching works, no unnecessary downloads
  2. Registry returns 200 with new content → Proves we detect changes and update ETag
  3. Registry returns 500 error → Proves exponential backoff kicks in, retries with increasing intervals
  4. Fifth consecutive failure → Proves max retries honored, bundle marked Failed
  5. Success after 2 failures → Proves backoff resets after success
  6. 100 bundles with 15min interval → Proves jitter spreads polls over time (not all at same second)
  7. ETag persists across operator restart → Proves state survives pod restart

**Implementation approach**:
You need 4 components:

  1. **Enhanced RegistryClient**: Update internal/registry/client.go
    - Add method: `ListTagsWithETag(ctx, url, auth, lastETag) (tags []string, newETag string, err error)`
    - Set `If-None-Match` header if lastETag provided
    - Return special error type for 304 Not Modified (not a real error)
    - Extract ETag from response headers
    - Handle common HTTP errors with descriptive error types (NetworkError, AuthError, NotFoundError)

  2. **WerfBundleStatus additions**: Update api/v1alpha1/werfbundle_types.go
    - Add `LastETag string` field to status
    - Add `ConsecutiveFailures int32` field to track retry attempts
    - Add `LastErrorTime metav1.Time` field to track when error occurred
    - These enable backoff calculation and debugging

  3. **Backoff calculator**: Create internal/registry/backoff.go
    - Function: `CalculateNextPoll(consecutiveFailures int32, baseInterval time.Duration) time.Duration`
    - Formula: `baseInterval * 2^consecutiveFailures` with cap at 30 minutes
    - For failures: 30s, 1m, 2m, 4m, 8m (then give up)
    - Separate function: `AddJitter(interval time.Duration) time.Duration` adds 0-20% random

  4. **Reconciler updates**: Update controllers/werfbundle_controller.go
    - Call ListTagsWithETag with status.LastETag
    - If 304 response, return early (no changes, requeue after pollInterval + jitter)
    - If 200 response, update status.LastETag and continue with job creation
    - On error, increment status.ConsecutiveFailures, calculate backoff, requeue after backoff interval
    - On success, reset status.ConsecutiveFailures to 0

**Key Go patterns**:
- **Custom error types**: Define `type NotModifiedError struct{}` to distinguish 304 from real errors
- **HTTP headers**: Use `resp.Header.Get("ETag")` and `req.Header.Set("If-None-Match", etag)`
- **Exponential backoff**: `time.Duration(30) * time.Second * (1 << failures)` - bit shift for power of 2
- **Random jitter**: `rand.Intn(int(interval / 5))` for 0-20% jitter, use `math/rand` with seed
- **Error wrapping**: Wrap HTTP errors with context: `fmt.Errorf("registry request failed: %w", err)`

**What success looks like**:
- Second poll of unchanged bundle returns instantly without downloading tags
- Registry failure triggers increasing retry intervals (30s, 1m, 2m, 4m, 8m)
- After 5 failures, bundle marked Failed with clear error message
- 100 bundles don't all poll at exactly :00 minutes - spread over several minutes
- You can explain why we store ETag in Status vs memory
- You understand the difference between retry backoff and poll interval jitter

---

## Task 2: Job Management Improvements

**The problem we're solving**: The basic job creation from Slice 1 can create duplicate jobs if reconciliation happens multiple times, doesn't capture logs for debugging, and has no resource limits (could starve cluster resources). We need production-grade job management with deduplication, observability, and resource controls.

**Why job deduplication matters**:
- Multiple reconciliations can occur while job is running (status updates, resync, manual edits)
- Without deduplication, we'd create multiple jobs for same bundle version - waste resources and confusing
- Trade-off: Must track job state in Status, adds complexity but prevents serious operational issues
- Operational benefit: Cluster resources are expensive - don't run same deployment twice

**Architecture decisions**:
  1. **Track active job in Status**: Add `activeJobName` field to status
    - Why: Provides link between bundle and job, enables deduplication check
    - Alternative: Listing all jobs and filtering by labels is slower and more complex
    - Job name includes bundle name and tag for easy correlation

  2. **Job naming includes content hash**: Name format `<bundle>-<tag-hash>-<short-uuid>`
    - Why: Deterministic part (tag hash) enables duplicate detection, UUID prevents collisions
    - Alternative: Random-only names would require checking every reconciliation if job exists
    - Example: `my-app-a3f8c2-x9k4s` for tag "v1.2.3"

  3. **Capture job logs in ConfigMap**: Create ConfigMap with last N lines of job output
    - Why: Jobs get garbage collected but we need logs for debugging failures
    - Alternative: External log aggregator is better long-term but adds dependency
    - ConfigMap limited to last 500 lines to avoid size issues (1MB limit)

  4. **Set resource limits on jobs**: CPU/memory limits from spec.converge.resourceLimits
    - Why: Prevents runaway werf processes from impacting cluster
    - Alternative: No limits risks OOM kills or cluster instability
    - Default: 1 CPU, 1Gi memory if not specified

  5. **Job TTL and cleanup**: Set `ttlSecondsAfterFinished` based on logRetentionDays
    - Why: Old jobs consume etcd space, automatic cleanup keeps cluster tidy
    - Alternative: Manual cleanup requires operator intervention

**Testing strategy**:
Test these scenarios:
  1. Reconcile during active job → Proves job deduplication works, no duplicate created
  2. Job completes successfully → Proves logs captured to ConfigMap, status updated to Synced
  3. Job fails → Proves logs captured with error details, status shows failure message
  4. Job without resource limits → Proves defaults applied (1 CPU, 1Gi)
  5. Job with custom limits → Proves spec.resourceLimits honored
  6. ConfigMap size exceeds 1MB → Proves truncation logic prevents API rejection
  7. Old job cleaned up after TTL → Proves automatic cleanup works

**Implementation approach**:
You need 5 components:

  1. **WerfBundleStatus additions**: Update api/v1alpha1/werfbundle_types.go
    - Add `ActiveJobName string` field tracking current job
    - Add `LastJobStatus string` field (Succeeded, Failed, Running)
    - Add `LastJobLogs string` field storing tail of logs (or ConfigMap reference)

  2. **Job deduplication check**: Update Reconcile() method
    - Before creating job, check if status.ActiveJobName is set
    - If set, fetch Job by name and check status
    - If job still running, return without creating new job (requeue to check later)
    - If job completed/failed, clear status.ActiveJobName and proceed

  3. **Enhanced job builder**: Update internal/converge/job.go
    - Function: `BuildJobSpec(bundle, tag, opts) *batchv1.Job` with options struct
    - Apply resource limits from bundle.Spec.Converge.ResourceLimits or defaults
    - Set `ttlSecondsAfterFinished` based on logRetentionDays (default 7 days = 604800 seconds)
    - Add labels for easy filtering: `werf.io/bundle: <name>`, `werf.io/tag: <tag>`
    - Set `backoffLimit: 0` - we handle retries at controller level, not job level

  4. **Log capture**: Create internal/converge/logs.go
    - Function: `CaptureJobLogs(ctx, client, jobName, namespace) (string, error)`
    - List pods with job label `batch.kubernetes.io/job-name=<jobName>`
    - Get logs from main container (limit to last 500 lines)
    - Trim to ~100KB if longer (ConfigMap limit is 1MB but leave margin)
    - Return as string for storage in Status or ConfigMap

  5. **Reconciler job monitoring**: Update Reconcile() method
    - After creating job, set status.ActiveJobName and status.LastJobStatus = "Running"
    - On subsequent reconciliations, check job status
    - If job succeeded: capture logs, update status to Synced, clear ActiveJobName
    - If job failed: capture logs, update status to Failed with error, clear ActiveJobName
    - Store logs in status.LastJobLogs (if < 10KB) or separate ConfigMap (if larger)

**Key Go patterns**:
- **Hash computation**: Use `hash/fnv` for deterministic short hashes: `h := fnv.New32a(); h.Write([]byte(tag))`
- **Resource quantities**: Use `resource.MustParse("1")` for CPU, `resource.MustParse("1Gi")` for memory
- **Pod log streaming**: Use `clientset.CoreV1().Pods(ns).GetLogs(name, &corev1.PodLogOptions{TailLines: ptr.To(int64(500))})`
- **Job status checking**: Check `job.Status.Succeeded > 0` or `job.Status.Failed > 0` for completion
- **ConfigMap creation**: `corev1.ConfigMap{Data: map[string]string{"logs": logContent}}`

**What success looks like**:
- Rapid reconciliations don't create duplicate jobs
- Failed job logs are visible in `kubectl describe werfbundle`
- Jobs have proper resource limits preventing cluster resource exhaustion
- Old completed jobs are automatically cleaned up after retention period
- You can explain why job names include both hash and UUID
- You understand the tradeoff between storing logs in Status vs ConfigMap

---

## Task 3: Testing

**The problem we're solving**: Slice 2 adds complex retry logic, caching, and state management that's difficult to test manually. We need comprehensive automated tests that verify backoff calculations, ETag handling, job deduplication, and error scenarios work correctly.

**Why test retry logic extensively**:
- Backoff and retry logic has many edge cases (success after N failures, max retries, timer calculations)
- Getting it wrong means either hammering failing registries or giving up too quickly
- Trade-off: More test cases means more maintenance, but retry bugs cause production incidents
- Operational benefit: Confidence that failure handling is correct before deploying

**Architecture decisions**:
  1. **Mock registry for deterministic unit/integration testing**: Create fake registry that returns controlled responses
    - Why: Can't rely on real registry failures/ETags in tests - need deterministic behavior
    - Alternative: Recording real responses is brittle and hard to maintain
    - Mock returns 304/200/500 on demand for testing all paths
    - Note: This slice also adds real OCI registry testing (deferred from Slice 1) to verify protocol handling

  2. **Table-driven tests for backoff logic**: Test all failure counts (0-5) and intervals
    - Why: Backoff formula is mathematical - exhaustive testing catches off-by-one errors
    - Alternative: Testing only happy path would miss edge cases
    - Each row: (failures, baseInterval, expectedBackoff)

  3. **Integration tests with envtest for job deduplication**: Use real K8s API for job lifecycle
    - Why: Deduplication involves watching jobs, updating status - need real API semantics
    - Alternative: Mocking K8s client is complex and doesn't catch API interaction bugs
    - envtest provides real API server for testing

  4. **Separate test for log capture size limits**: Test with logs > 1MB to verify truncation
    - Why: ConfigMap size limit is hard limit - must handle gracefully
    - Alternative: Not testing would lead to production errors when large logs occur

**Testing strategy**:
Test these scenarios:

**Unit Tests** (internal/registry/backoff_test.go):
  1. Zero failures returns base interval → Proves no backoff on success
  2. One failure returns 2x base interval → Proves exponential backoff starts correctly
  3. Five failures returns capped interval → Proves max backoff enforced
  4. Jitter adds 0-20% variance → Proves jitter calculation correct (test multiple iterations)
  5. Negative or nil inputs handled → Proves defensive programming

**Unit Tests** (internal/registry/client_test.go):
  1. Request with ETag receives 304 → Proves If-None-Match header sent, NotModified error returned
  2. Request without ETag receives 200 → Proves normal flow works
  3. Response includes new ETag → Proves ETag extracted from headers
  4. Network error wrapped with context → Proves error handling provides useful messages
  5. 401/403 returns AuthError type → Proves auth errors distinguishable
  6. 404 returns NotFoundError type → Proves missing repos handled separately

**Integration Tests** (controllers/werfbundle_controller_test.go):
  1. Create bundle, reconcile twice rapidly → Proves only one job created (deduplication)
  2. Job completes, reconcile again → Proves ActiveJobName cleared, status updated
  3. Job fails, logs captured → Proves error logs stored in status
  4. Bundle with resourceLimits → Proves job created with specified limits
  5. Bundle without resourceLimits → Proves defaults applied
  6. Reconcile with ETag match → Proves 304 response doesn't create job
  7. Reconcile with ETag mismatch → Proves 200 response creates job with new tag

**E2E Tests** (test/e2e/):
  1. Deploy operator, create bundle, verify job runs with limits → Proves full flow works
  2. Simulate registry failure, verify backoff → Proves retry logic works end-to-end
  3. Update registry, verify new job created → Proves change detection works

**Real Registry Tests** (internal/registry/integration_test.go):
  1. Push tag to real Docker registry, list tags → Proves real OCI protocol handling works
  2. Request with ETag, verify 304 response → Proves real registry ETag behavior matches expectations
  3. Authentication with real registry → Proves auth headers properly formatted
  4. Invalid repository returns 404 → Proves error handling works with real HTTP responses

**Implementation approach**:
You need 6 components:

  1. **Mock registry**: Create internal/registry/mock.go
    - Struct: `type MockRegistry struct { Responses map[string]*http.Response }`
    - Method: `SetResponse(url, status, etag, body)` configures next response
    - Implements same interface as real RegistryClient
    - Tracks call count for verification

  2. **Backoff test cases**: Create internal/registry/backoff_test.go
    - Table-driven test with cases for each failure count
    - Test jitter with multiple runs (10 iterations) to verify randomness range
    - Test that max backoff caps at configured maximum
    - Verify negative inputs return safe defaults

  3. **Registry client tests**: Create internal/registry/client_test.go
    - Use httptest.Server for controlled HTTP responses
    - Set ETag headers: `w.Header().Set("ETag", "\"abc123\"")`
    - Check If-None-Match header in request
    - Verify NotModifiedError returned for 304
    - Verify other error types for 401/403/404/500

  4. **Integration test helpers**: Update controllers/suite_test.go
    - Helper: `CreateTestBundle(name, tag, etag) *werfv1alpha1.WerfBundle`
    - Helper: `WaitForJobCreation(bundle) *batchv1.Job` with timeout
    - Helper: `SimulateJobCompletion(job, success bool)` updates job status
    - Helper: `GetJobLogs(job) string` extracts logs from status/ConfigMap

  5. **Integration test cases**: Update controllers/werfbundle_controller_test.go
    - Use Ginkgo BeforeEach to set up test bundle
    - Use Eventually() for async operations (job creation, status updates)
    - Assert on job count: `Expect(jobList.Items).To(HaveLen(1))`
    - Assert on resource limits: `Expect(job.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu())`

  6. **Real registry test infrastructure**: Create internal/registry/integration_test.go and test helpers
    - Use testcontainers-go to spin up Docker registry container for tests
    - Helper: `StartTestRegistry() (url, cleanup func())` starts registry and returns cleanup
    - Push test images using go-containerregistry in test setup
    - Test real HTTP interactions: ETags, auth, 304 responses, error codes
    - Skip if Docker not available: `if testing.Short() { t.Skip() }`
    - These tests verify our OCI protocol handling is correct (deferred from Slice 1)

**Key Go patterns**:
- **Test tables**: `tests := []struct{name string; input X; want Y; wantErr bool}{...}`
- **httptest.Server**: `srv := httptest.NewServer(http.HandlerFunc(...))` for HTTP mocking
- **gomega Eventually**: `Eventually(func() int { return len(jobs.Items) }).Should(Equal(1))`
- **Resource comparison**: `Expect(cpu.Cmp(resource.MustParse("1"))).To(Equal(0))`
- **Time-based tests**: Use fixed seed for random in tests: `rand.Seed(42)` for reproducibility

**What success looks like**:
- All backoff calculations verified with exhaustive test cases
- Mock registry enables testing all HTTP response scenarios
- Real registry tests verify OCI protocol handling (ETags, auth, HTTP codes)
- Integration tests prove job deduplication prevents duplicates
- Log capture tested with edge cases (empty logs, huge logs, non-UTF8)
- Test coverage > 85% for registry and converge packages
- You can explain why we use both mocks (deterministic) and real registry (protocol verification)
- You understand when to use unit tests vs integration tests vs E2E tests

---

## Task 4: Documentation

**The problem we're solving**: Users need to understand the new reliability features (ETag caching, backoff, resource limits) and how to configure them. Documentation must explain what changed, why it matters, and how to use the new capabilities.

**Why document reliability features prominently**:
- Users coming from Slice 1 need to know what improved
- Understanding backoff behavior helps debug why bundle isn't updating immediately
- Resource limits prevent cluster issues - users must configure appropriately
- Trade-off: More docs means more maintenance, but undocumented features go unused

**Implementation approach**:
You need 3 components:

  1. **README updates**: Update root README.md
    - Add "Reliability Features" section explaining ETag caching, exponential backoff, jitter
    - Update examples to show `pollInterval` configuration: `pollInterval: 15m`
    - Add example showing `resourceLimits`: `cpu: "2"`, `memory: "2Gi"`
    - Update "What works now" section to include "Robust registry polling with caching and retry"
    - Update "Next steps" to point to Slice 3 (values and multi-namespace)

  2. **Configuration reference**: Create docs/configuration.md
    - Document `pollInterval` field: format, default (15m), minimum (1m), maximum (24h)
    - Document `resourceLimits` field: defaults (1 CPU, 1Gi), recommended values for different workload sizes
    - Document `logRetentionDays` field: default (7), affects job TTL
    - Explain retry behavior: 5 max retries, exponential backoff (30s to 8m)
    - Explain jitter: adds 0-20% randomness to poll intervals

  3. **Troubleshooting guide**: Create docs/troubleshooting.md
    - "Bundle stuck in Syncing" → Check ConsecutiveFailures in status, may be in backoff
    - "Too many registry requests" → Increase pollInterval, verify ETag support in registry
    - "Job failed with OOMKilled" → Increase memory in resourceLimits
    - "Logs not captured" → Check job pod logs directly, may exceed ConfigMap size
    - Include kubectl commands for debugging: `kubectl describe werfbundle`, `kubectl logs job/<name>`

**What success looks like**:
- Users can find documentation for all new configuration fields
- Troubleshooting guide covers common issues users will encounter
- Examples are copy-pasteable and actually work
- "What works now" accurately reflects Slice 2 capabilities

## Slice 3: Values and Multi-namespace Support

---

## Task 1: Values Integration

**The problem we're solving**: Werf bundles need external configuration (database URLs, API keys, feature flags) that varies per environment. Hardcoding these in the bundle is inflexible and insecure. We need to pass values from ConfigMaps and Secrets to werf converge as `--set` arguments, with proper precedence and optional sources.

**Why use ConfigMaps and Secrets**:
- ConfigMaps for non-sensitive config (URLs, feature flags) - visible in kubectl
- Secrets for sensitive data (passwords, tokens) - base64 encoded, access controlled
- Trade-off: Values are external to bundle, requires coordination, but enables reuse across environments
- Operational benefit: Same bundle deployed to dev/staging/prod with different values

**Architecture decisions**:
  1. **ValuesFrom array with precedence**: Later entries override earlier ones
    - Why: Enables layering (base values + environment-specific overrides)
    - Alternative: Single values source would require duplicating common values
    - Example: `[{configMapRef: common}, {configMapRef: prod}]` - prod values override common

  2. **Optional sources**: Mark ConfigMap/Secret as optional if it might not exist
    - Why: Allows for environment-specific values that don't exist everywhere
    - Alternative: Requiring all sources to exist makes deployment fragile
    - Example: `{secretRef: {name: db-creds, optional: true}}` won't fail if Secret missing

  3. **Namespace-aware lookups**: Look in bundle namespace first, then target namespace
    - Why: Values might be in operator namespace (centralized) or app namespace (distributed)
    - Alternative: Single namespace search would require moving all values to one place
    - Operator namespace is checked first for security (admin-controlled values take precedence)

  4. **Convert to werf --set flags**: Transform key-value pairs to CLI arguments
    - Why: Werf converge uses --set for value injection, not env vars or files
    - Alternative: Creating values file would require temp storage and cleanup
    - Example: `{foo: bar, nested.key: val}` becomes `--set foo=bar --set nested.key=val`

  5. **No value interpolation or templates**: Pass values as-is to werf
    - Why: Keep operator simple, werf handles templating, we're just a passthrough
    - Alternative: Doing variable substitution would duplicate werf's logic and cause bugs

**Testing strategy**:
Test these scenarios:
  1. Single ConfigMap values → Proves basic lookup and conversion works
  2. Multiple ValuesFrom with overlapping keys → Proves precedence (later wins)
  3. Optional Secret missing → Proves optional flag honored, no error
  4. Required ConfigMap missing → Proves required sources fail with clear error
  5. Values in operator namespace → Proves namespace-aware lookup works
  6. Values in target namespace → Proves cross-namespace lookup works
  7. Nested keys (dot notation) → Proves complex key paths converted correctly
  8. Special characters in values → Proves proper escaping in --set arguments

**Implementation approach**:
You need 4 components:

  1. **WerfBundleSpec additions**: Update api/v1alpha1/werfbundle_types.go
    - Add `ValuesFrom []ValuesSource` field to Converge spec
    - Define `type ValuesSource struct { ConfigMapRef, SecretRef *LocalObjectReference; Optional bool }`
    - Add validation: at least one of ConfigMapRef or SecretRef must be set
    - Add kubebuilder marker for optional: `// +kubebuilder:validation:Optional`

  2. **Values resolver**: Create internal/values/resolver.go
    - Function: `ResolveValues(ctx, client, bundle, valuesFrom) (map[string]string, error)`
    - For each ValuesSource, determine namespace (bundle namespace or target namespace)
    - Fetch ConfigMap or Secret using client.Get()
    - If not found and Optional=false, return error
    - If not found and Optional=true, skip silently
    - Merge all values with later sources overriding earlier ones
    - Return flat map of key-value pairs

  3. **Values to CLI converter**: Create internal/values/cli.go
    - Function: `ToSetFlags(values map[string]string) []string`
    - For each key-value pair, create `--set key=value` argument
    - Escape special characters in values (quotes, spaces, equals)
    - Handle nested keys (already dot-notation in map keys)
    - Return slice of strings to append to converge command args

  4. **Job builder integration**: Update internal/converge/job.go
    - Before building job, call ResolveValues() to get merged values
    - Call ToSetFlags() to convert to CLI arguments
    - Append --set flags to werf converge command in job container args
    - Handle errors from ResolveValues (missing required ConfigMap/Secret)

**Key Go patterns**:
- **Map merging**: `for k, v := range source { merged[k] = v }` - later maps override earlier
- **Optional fields**: Use `*LocalObjectReference` pointer for optional ConfigMapRef/SecretRef
- **Namespace resolution**: Check `bundle.Spec.Converge.TargetNamespace` or default to `bundle.Namespace`
- **String escaping**: Use `strings.ReplaceAll()` or `strconv.Quote()` for special characters
- **Error aggregation**: Collect all missing required sources before returning error

**What success looks like**:
- ConfigMap values properly passed to werf converge as --set flags
- Multiple ValuesFrom sources merge correctly with right precedence
- Optional missing sources don't cause failures
- Required missing sources fail with clear error naming the missing ConfigMap/Secret
- You can explain why we look in operator namespace before target namespace
- You understand why we don't do variable interpolation in the operator

---

## Task 2: Multi-namespace Support

**The problem we're solving**: Currently operator and target app deploy to same namespace. In production, operators typically run in their own namespace (e.g., `werf-operator-system`) while deploying apps to separate namespaces (e.g., `my-app-prod`). We need cross-namespace deployment with proper RBAC validation.

**Why separate operator and app namespaces**:
- Security isolation: Operator permissions don't grant app permissions and vice versa
- Multi-tenancy: One operator can deploy to many app namespaces
- Trade-off: More complex RBAC setup, requires pre-configuration of target namespaces
- Operational benefit: Ops team controls operator, dev teams can't modify operator resources

**Architecture decisions**:
  1. **TargetNamespace field in spec**: Explicitly specify where to deploy
    - Why: Clear and auditable - no magic namespace guessing
    - Alternative: Deriving from bundle name would be confusing and error-prone
    - Defaults to bundle namespace for backward compatibility

  2. **Validate ServiceAccount exists before job creation**: Pre-flight check
    - Why: Fail fast with clear error instead of obscure job pod auth failures
    - Alternative: Letting job fail would work but harder to debug
    - Check both ServiceAccount exists and has proper RoleBinding

  3. **Operator needs cross-namespace Secret read**: For registry credentials in target namespace
    - Why: Bundle may reference registry Secret in target namespace, not operator namespace
    - Alternative: Requiring all Secrets in operator namespace is inflexible
    - Add RBAC rule: get Secrets in all namespaces (limited scope)

  4. **Job runs with target namespace ServiceAccount**: Not operator ServiceAccount
    - Why: Job needs app deployment permissions, not operator management permissions
    - Alternative: Using operator SA would require giving operator excessive permissions
    - ServiceAccount must be pre-created by cluster admin in target namespace

  5. **Document namespace setup requirements**: Don't auto-create namespaces or RBAC
    - Why: Creating namespaces/RBAC is policy decision, should be explicit
    - Alternative: Auto-creating would hide security implications from admins
    - Provide clear documentation and example YAML for setup

**Testing strategy**:
Test these scenarios:
  1. Deploy to same namespace (default) → Proves backward compatibility
  2. Deploy to different namespace with valid SA → Proves cross-namespace works
  3. Deploy to namespace without ServiceAccount → Proves validation fails with clear error
  4. Deploy with ServiceAccount but no RBAC → Proves job fails with auth error (expected)
  5. Registry Secret in target namespace → Proves cross-namespace Secret reading works
  6. Registry Secret in operator namespace → Proves original flow still works
  7. Multiple bundles to same target namespace → Proves no conflicts

**Implementation approach**:
You need 4 components:

  1. **WerfBundleSpec additions**: Update api/v1alpha1/werfbundle_types.go
    - Add `TargetNamespace string` field to Converge spec
    - Add `ServiceAccountName string` field to Converge spec (required for cross-namespace)
    - Add validation: if TargetNamespace != bundle namespace, ServiceAccountName required
    - Add kubebuilder validation markers

  2. **ServiceAccount validator**: Create internal/rbac/validator.go
    - Function: `ValidateServiceAccount(ctx, client, name, namespace) error`
    - Check ServiceAccount exists using client.Get()
    - Optionally check RoleBinding exists (list RoleBindings with SA subject)
    - Return descriptive error: "ServiceAccount 'werf-converge' not found in namespace 'my-app'"
    - This is pre-flight check, doesn't guarantee job will succeed (RBAC might still be wrong)

  3. **Namespace resolution**: Update Reconcile() method
    - Function: `getTargetNamespace(bundle) string` returns target or defaults to bundle namespace
    - Use this consistently for: Secret lookups, Job creation, ServiceAccount validation
    - Update status to include target namespace for visibility

  4. **RBAC updates**: Update controllers/werfbundle_controller.go markers
    - Add marker: `// +kubebuilder:rbac:groups="",resources=secrets,verbs=get,namespace=*`
    - Add marker: `// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get,namespace=*`
    - This grants operator read access to Secrets/SAs in all namespaces
    - Run `make manifests` to regenerate RBAC YAML

**Key Go patterns**:
- **Namespace defaulting**: `ns := bundle.Spec.Converge.TargetNamespace; if ns == "" { ns = bundle.Namespace }`
- **Cross-namespace Get**: `client.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &obj)`
- **Optional validation**: Only validate SA if cross-namespace deployment (TargetNamespace != bundle namespace)
- **Status visibility**: Add `TargetNamespace` to status for debugging

**What success looks like**:
- Operator in `werf-operator-system` deploys app to `my-app-prod` successfully
- Missing ServiceAccount fails immediately with helpful error message
- Registry Secrets work from either operator or target namespace
- Backward compatibility maintained (same-namespace deployments still work)
- You can explain why ServiceAccount must be in target namespace, not operator namespace
- You understand the security boundary between operator and app namespaces

---

## Task 3: Testing

**The problem we're solving**: Values merging and cross-namespace deployment have complex logic with many edge cases (precedence, optional sources, namespace resolution, RBAC). Manual testing would be time-consuming and error-prone. We need automated tests covering all combinations.

**Why test namespace permissions carefully**:
- Cross-namespace RBAC is error-prone - easy to forget a permission or use wrong namespace
- Production outages often caused by incorrect RBAC setup
- Trade-off: RBAC tests require complex setup (namespaces, SAs, Roles) but catch critical bugs
- Operational benefit: Confidence that permission model works before production deployment

**Architecture decisions**:
  1. **Use envtest for namespace creation**: Create real namespaces in test API server
    - Why: Namespace is fundamental K8s concept - mocking doesn't test real behavior
    - Alternative: Mocking namespaces would miss API validation and defaulting logic
    - envtest supports full namespace lifecycle

  2. **Test both valid and invalid RBAC**: Create scenarios with missing/wrong permissions
    - Why: Need to verify both success path and failure path give right results
    - Alternative: Only testing success wouldn't catch permission leaks or bad error messages
    - Invalid RBAC should fail job pod creation, not operator reconciliation

  3. **Table-driven values merging tests**: Matrix of value sources and precedence
    - Why: Merging logic has many combinations (2 sources, 3 sources, overlapping keys, etc.)
    - Alternative: Individual test per case would be verbose and hard to maintain
    - Each row: (sources, expected merged result)

  4. **Integration test for full cross-namespace flow**: End-to-end namespace deployment
    - Why: Unit tests verify components, but integration test proves they work together
    - Alternative: Only unit tests would miss integration issues between resolver, validator, job builder
    - Create operator namespace, target namespace, bundle, verify job in target namespace

**Testing strategy**:
Test these scenarios:

**Unit Tests** (internal/values/resolver_test.go):
  1. Single ConfigMap all keys → Proves basic resolution works
  2. Two ConfigMaps overlapping keys → Proves later overrides earlier
  3. ConfigMap + Secret overlapping → Proves Secret takes precedence (listed later)
  4. Optional ConfigMap missing → Proves skipped without error
  5. Required ConfigMap missing → Proves error with ConfigMap name
  6. Values from operator namespace → Proves namespace resolution works
  7. Empty values (zero-length value) → Proves empty strings handled

**Unit Tests** (internal/values/cli_test.go):
  1. Simple key-value → Proves basic --set generation
  2. Nested key (dot notation) → Proves dots preserved in --set
  3. Value with spaces → Proves quoting/escaping works
  4. Value with quotes → Proves quote escaping works
  5. Value with equals sign → Proves equals escaping works
  6. Unicode values → Proves non-ASCII characters handled

**Unit Tests** (internal/rbac/validator_test.go):
  1. ServiceAccount exists → Proves validation succeeds
  2. ServiceAccount missing → Proves validation fails with correct error message
  3. ServiceAccount in different namespace → Proves cross-namespace check works

**Integration Tests** (controllers/werfbundle_controller_test.go):
  1. Bundle with valuesFrom ConfigMap → Proves job created with --set args
  2. Bundle with valuesFrom Secret → Proves Secret values included
  3. Bundle with targetNamespace → Proves job created in target namespace
  4. Bundle with targetNamespace but no SA → Proves reconciliation fails with validation error
  5. Bundle with values in target namespace → Proves cross-namespace value resolution
  6. Two bundles, same target namespace → Proves no conflicts, both jobs created

**E2E Tests** (test/e2e/):
  1. Deploy operator, create target namespace with SA/Role, create bundle → Proves full flow
  2. Create bundle with values, verify values in job logs → Proves values passed correctly
  3. Update ConfigMap, trigger reconciliation → Proves value changes detected

**Implementation approach**:
You need 4 components:

  1. **Values test fixtures**: Create test/fixtures/values-*.yaml
    - ConfigMap with sample values: `app.name: myapp`, `app.replicas: 3`
    - Secret with sample values: `db.password: secret123`
    - Use these in integration tests for realistic scenarios

  2. **RBAC test helpers**: Create internal/testing/rbac.go (extend from Slice 1)
    - Helper: `CreateTestNamespace(client, name) *corev1.Namespace`
    - Helper: `CreateTestServiceAccountWithRole(client, name, namespace, rules) (*corev1.ServiceAccount, *rbacv1.Role, *rbacv1.RoleBinding)`
    - These simplify test setup for cross-namespace scenarios

  3. **Values test helpers**: Create internal/values/testing.go
    - Helper: `CreateConfigMapWithValues(client, name, namespace, data map[string]string)`
    - Helper: `CreateSecretWithValues(client, name, namespace, data map[string]string)`
    - Helper: `ExtractSetFlags(jobArgs []string) map[string]string` parses --set from job args

  4. **Integration test suite**: Update controllers/werfbundle_controller_test.go
    - Create "Values Integration" test context
    - Create "Multi-namespace Deployment" test context
    - Use BeforeEach to set up namespaces and fixtures
    - Use AfterEach to clean up namespaces
    - Assert on job args containing expected --set flags
    - Assert on job namespace matching target namespace

**Key Go patterns**:
- **Namespace creation in tests**: `ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-ns"}}; client.Create(ctx, ns)`
- **Cleanup in tests**: `defer client.Delete(ctx, ns)` or use Ginkgo AfterEach
- **Arg parsing in assertions**: `strings.Contains(strings.Join(args, " "), "--set foo=bar")`
- **RBAC rule building**: `rules := []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"create"}}}`

**What success looks like**:
- Values merging logic verified with all precedence combinations
- Cross-namespace deployment tested with proper namespace isolation
- RBAC validation catches missing ServiceAccounts before job creation
- CLI conversion handles all special characters correctly
- Test coverage > 80% for values and rbac packages
- You can explain why we test both valid and invalid RBAC
- You understand the difference between operator permissions and job permissions

---

## Task 4: Documentation

**The problem we're solving**: Users need to understand how to configure values management and set up multi-namespace deployments. RBAC setup is particularly complex and error-prone - documentation must be clear, complete, and include working examples.

**Why RBAC documentation is critical**:
- RBAC mistakes are most common deployment failure for multi-namespace operators
- Users need to understand both operator permissions and job permissions
- Trade-off: Detailed RBAC docs are long, but incomplete docs cause production issues
- Operational benefit: Self-service setup reduces support burden

**Implementation approach**:
You need 4 components:

  1. **README updates**: Update root README.md
    - Add "Configuration Values" section with ConfigMap and Secret examples
    - Add "Multi-namespace Deployment" section with targetNamespace example
    - Update "What works now" section to include "External values from ConfigMaps/Secrets" and "Cross-namespace deployments"
    - Update "Next steps" to point to Slice 4 (drift detection)

  2. **Values examples**: Update examples/ directory or inline in README
    - Example: WerfBundle with valuesFrom referencing ConfigMap
    - Example: WerfBundle with multiple ValuesFrom (precedence demo)
    - Example: Optional Secret for environment-specific config
    - Show complete YAML that users can apply directly

  3. **RBAC documentation**: Update docs/job-rbac.md
    - Add section on cross-namespace deployments
    - Explain operator needs: read Secrets/ServiceAccounts in target namespace
    - Explain job needs: full app deployment permissions in target namespace
    - Provide example Role YAML for common scenarios (deploy app with Deployments/Services/Ingress)
    - Provide example RoleBinding connecting Role to ServiceAccount
    - Include commands to verify RBAC: `kubectl auth can-i create pods --as=system:serviceaccount:my-app:werf-converge -n my-app`

  4. **Troubleshooting additions**: Update docs/troubleshooting.md
    - "ServiceAccount not found" error → Verify ServiceAccount created in target namespace
    - "Unauthorized" errors in job pod → Check RoleBinding exists and is correct
    - "ConfigMap not found" error → Check ConfigMap exists and optional flag if conditional
    - "Values not applied" → Check job logs for --set flags, verify werf bundle schema
    - Provide debugging workflow: check Status, describe bundle, check job logs, verify RBAC

**What success looks like**:
- Users can follow docs to set up cross-namespace deployment without support
- RBAC examples are correct and complete (tested)
- Values examples demonstrate common patterns (base + override)
- Troubleshooting covers the most common configuration mistakes
- "What works now" accurately lists valuesFrom and targetNamespace features

## Slice 4: Drift Detection
Implement drift detection and correction.

---

## Task 1: Drift Detector

**The problem we're solving**: After werf converge deploys resources, users might manually modify them (kubectl edit, scripts, other tools). These manual changes cause drift from desired state defined in bundle. Without detection, operator doesn't know about drift and can't correct it. We need periodic checks that detect drift and trigger re-convergence.

**Why drift detection matters**:
- Manual changes happen in production (emergency fixes, debugging, misconfigurations)
- Without detection, cluster state diverges from bundle definition indefinitely
- Trade-off: Periodic checks consume resources, but drift causes subtle production issues
- Operational benefit: Self-healing - cluster automatically returns to desired state

**Architecture decisions**:
  1. **Separate drift check interval from poll interval**: Independent timing for registry checks vs drift checks
    - Why: Drift detection and registry polling are different concerns with different frequencies
    - Alternative: Using same interval would tie unrelated operations together
    - Example: Poll registry every 15min, check drift every 5min

  2. **Drift check triggers werf converge in check mode**: Run converge with --dry-run to detect differences
    - Why: Werf itself knows what resources should exist and can compare to actual state
    - Alternative: Implementing our own diff logic would duplicate werf's complex comparison
    - Werf exit code indicates drift: 0 = no drift, non-zero = drift detected

  3. **Auto-correction optional**: driftDetection.enabled can be true (auto-correct) or false (detect only)
    - Why: Some users want to know about drift but not auto-fix (for audit/approval workflows)
    - Alternative: Always auto-correcting might override intentional changes
    - When enabled, drift triggers normal converge job (same as registry update)

  4. **Track drift in Status conditions**: Add Condition type "Drifted" to status
    - Why: Conditions are standard Kubernetes pattern for state tracking
    - Alternative: Custom status fields would be non-standard and harder to query
    - Condition shows: when drift detected, what triggered correction, correction status

  5. **Max retry for drift correction**: After 3 failed corrections, stop retrying and alert
    - Why: If drift keeps happening, might be external system fighting operator
    - Alternative: Infinite retries would waste resources and hide underlying issue
    - Status shows "DriftCorrectionFailed" condition for monitoring/alerting

**Testing strategy**:
Test these scenarios:
  1. Resources match bundle → Proves no false positives, no unnecessary corrections
  2. Resource manually modified → Proves drift detected within check interval
  3. Drift detected, auto-correct enabled → Proves converge job created and drift fixed
  4. Drift detected, auto-correct disabled → Proves detection without correction, status updated
  5. Drift correction fails 3 times → Proves retry limit enforced, DriftCorrectionFailed condition set
  6. Drift corrected successfully → Proves Drifted condition cleared, status returns to Synced
  7. Registry update during drift handling → Proves registry updates take precedence

**Implementation approach**:
You need 5 components:

  1. **WerfBundleSpec additions**: Update api/v1alpha1/werfbundle_types.go
    - Add `DriftDetection` struct to Converge spec
    - Fields: `Enabled bool`, `Interval metav1.Duration` (default 15m), `MaxRetries int32` (default 3)
    - Add kubebuilder validation: Interval minimum 1m, MaxRetries minimum 1

  2. **WerfBundleStatus additions**: Update api/v1alpha1/werfbundle_types.go
    - Add `Conditions []metav1.Condition` field (standard Kubernetes condition pattern)
    - Define condition types: `"Drifted"`, `"DriftCorrectionFailed"`
    - Add `DriftCheckCount int32` tracking how many checks performed
    - Add `DriftCorrectionAttempts int32` tracking correction retries

  3. **Drift checker**: Create internal/drift/detector.go
    - Function: `CheckDrift(ctx, client, bundle) (bool, error)` returns whether drift detected
    - Create temporary Job running `werf converge --dry-run`
    - Wait for job completion (with timeout)
    - Check job exit code: 0 = no drift, non-zero = drift
    - Parse job logs to extract what changed (for status message)
    - Return drift boolean and description of changes

  4. **Reconciler drift logic**: Update controllers/werfbundle_controller.go
    - After successful converge, schedule next drift check using `RequeueAfter: driftInterval`
    - On drift check reconciliation, call CheckDrift()
    - If no drift: update status.DriftCheckCount, requeue after driftInterval
    - If drift detected: set "Drifted" condition with reason and message
    - If auto-correct enabled: create converge job (same as registry update path)
    - If auto-correct disabled: just update condition, requeue after driftInterval
    - If correction fails: increment DriftCorrectionAttempts, check against MaxRetries

  5. **Condition helpers**: Create internal/conditions/helpers.go
    - Function: `SetCondition(bundle, conditionType, status, reason, message)`
    - Function: `RemoveCondition(bundle, conditionType)`
    - Function: `IsConditionTrue(bundle, conditionType) bool`
    - These standardize condition management across reconciler

**Key Go patterns**:
- **Conditions**: Use `meta.SetStatusCondition(&bundle.Status.Conditions, metav1.Condition{...})`
- **Condition query**: Use `meta.FindStatusCondition(bundle.Status.Conditions, "Drifted")`
- **Time-based requeue**: Return `ctrl.Result{RequeueAfter: driftInterval}` for scheduled checks
- **Job with timeout**: Use context.WithTimeout for drift check job waiting
- **Exit code checking**: `job.Status.Succeeded > 0` for success, `job.Status.Failed > 0` for failure

**What success looks like**:
- Manual resource changes detected within configured drift interval
- Auto-correction restores desired state automatically when enabled
- Drift detected but not corrected when auto-correct disabled
- Status conditions clearly show drift state and correction attempts
- Failed corrections stop after MaxRetries to avoid infinite loops
- You can explain why drift checking uses a separate job instead of inline logic
- You understand the difference between detecting drift and correcting drift

---

## Task 2: Metrics and Observability

**The problem we're solving**: Operators need metrics for monitoring, alerting, and capacity planning. Users need to know: registry poll success rate, job execution time, drift detection frequency, error rates. Without metrics, operator is a black box and problems go unnoticed until failure.

**Why Prometheus metrics**:
- Prometheus is standard for Kubernetes monitoring - integrates with existing infrastructure
- Time-series data enables trend analysis (error rate increasing?) and alerting
- Trade-off: Metrics add overhead (memory, CPU), but observability is worth it
- Operational benefit: Proactive problem detection before user impact

**Architecture decisions**:
  1. **Use controller-runtime metrics**: Built-in metrics for reconciliation timing, queue depth
    - Why: Comes free with controller-runtime, well-tested, standard metrics
    - Alternative: Building from scratch would miss important operational metrics
    - Includes: reconcile duration, reconcile errors, workqueue depth

  2. **Add custom metrics for operator-specific operations**: Registry polls, drift checks, job results
    - Why: Built-in metrics don't cover domain-specific operations
    - Alternative: Relying only on built-in metrics would miss key operational data
    - Custom metrics: registry_poll_total, converge_job_duration_seconds, drift_detection_total

  3. **Metric cardinality limits**: Avoid high-cardinality labels (bundle name OK, tag value NO)
    - Why: High cardinality causes memory issues in Prometheus
    - Alternative: Including every tag/version would create millions of series
    - Use labels: namespace, bundle_name, result (success/failure) - avoid: tag, error_message

  4. **Expose metrics on /metrics endpoint**: Standard Prometheus scraping
    - Why: Prometheus scrapes HTTP endpoints, this is the standard contract
    - Alternative: Push metrics would require external dependency and more complexity
    - Controller-runtime sets this up automatically on port 8080

**Testing strategy**:
Test these scenarios:
  1. Reconcile triggers, metrics updated → Proves basic metric recording works
  2. Registry poll success/failure → Proves registry metrics increment correctly
  3. Converge job completion → Proves job duration metric recorded
  4. Drift check performed → Proves drift metrics increment
  5. Multiple bundles → Proves metrics properly labeled by namespace/bundle
  6. Scrape /metrics endpoint → Proves Prometheus format valid

**Implementation approach**:
You need 3 components:

  1. **Metric definitions**: Create internal/metrics/metrics.go
    - Define metrics using prometheus client:
      - `registry_poll_total` (Counter with labels: namespace, bundle, result)
      - `registry_poll_duration_seconds` (Histogram with labels: namespace, bundle)
      - `converge_job_total` (Counter with labels: namespace, bundle, result)
      - `converge_job_duration_seconds` (Histogram with labels: namespace, bundle)
      - `drift_detection_total` (Counter with labels: namespace, bundle, drifted)
      - `drift_correction_total` (Counter with labels: namespace, bundle, result)
    - Register metrics with prometheus.DefaultRegisterer in init()

  2. **Metric recording**: Update reconciler and internal packages
    - In registry client: record poll timing and result
      ```go
      start := time.Now()
      // ... do poll ...
      registry_poll_duration_seconds.WithLabelValues(ns, name).Observe(time.Since(start).Seconds())
      registry_poll_total.WithLabelValues(ns, name, "success").Inc()
      ```
    - In job monitoring: record job duration and result
    - In drift detector: record detection and correction metrics

  3. **Status condition tracking**: Update WerfBundleStatus conditions
    - Add conditions for visibility: `Ready`, `Synced`, `Drifted`, `Failed`
    - Conditions provide human-readable status for kubectl
    - Metrics provide machine-readable data for monitoring
    - Both are complementary - conditions for debugging, metrics for alerting

**Key Go patterns**:
- **Counter definition**: `prometheus.NewCounterVec(prometheus.CounterOpts{Name: "...", Help: "..."}, []string{"label1", "label2"})`
- **Histogram definition**: `prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "...", Buckets: prometheus.DefBuckets}, []string{"label"})`
- **Metric recording**: `metric.WithLabelValues("value1", "value2").Inc()` or `.Observe(duration)`
- **Condition management**: `meta.SetStatusCondition()` from `k8s.io/apimachinery/pkg/api/meta`

**What success looks like**:
- Prometheus scrapes operator metrics successfully
- Metrics show accurate counts of polls, jobs, drift checks
- Duration histograms enable latency percentile analysis (p50, p95, p99)
- Alert rules can trigger on error rate thresholds
- You can explain why we avoid high-cardinality labels
- You understand the difference between Counters and Histograms

---

## Task 3: Testing

**The problem we're solving**: Drift detection involves timing (periodic checks), state comparison (detecting changes), and retry logic (correction attempts). Metrics must be accurately recorded. These are complex interactions that need thorough automated testing.

**Why test timing and state transitions**:
- Drift detection timing is critical - too fast wastes resources, too slow misses issues
- State transitions (Synced → Drifted → Synced) must be correct for reliability
- Trade-off: Time-based tests are slower, but timing bugs cause production issues
- Operational benefit: Confidence that drift correction actually works as designed

**Architecture decisions**:
  1. **Use fake clock for timing tests**: Control time instead of real delays
    - Why: Tests using real time.Sleep are slow and flaky
    - Alternative: Mocking time enables instant "fast-forward" in tests
    - Use controller-runtime's fake clock or custom time interface

  2. **Simulate drift by modifying resources**: In tests, directly edit resources after deployment
    - Why: This is how real drift happens - external modification
    - Alternative: Can't rely on real external changes in automated tests
    - Use envtest to deploy, then modify resources, verify detection

  3. **Mock metrics registry for verification**: Count metric recordings in tests
    - Why: Can't scrape real Prometheus in unit tests, need to verify calls
    - Alternative: Testing against real Prometheus is slow and complex
    - Mock registry tracks Inc() and Observe() calls

  4. **Table-driven condition tests**: Test all state transitions
    - Why: Conditions have many combinations (Synced+Drifted, Failed+DriftCorrectionFailed, etc.)
    - Alternative: Individual tests would miss transition bugs
    - Each row: (initial conditions, event, expected conditions)

**Testing strategy**:
Test these scenarios:

**Unit Tests** (internal/drift/detector_test.go):
  1. Resources unchanged → Returns no drift
  2. Resource modified → Returns drift detected
  3. Resource deleted → Returns drift detected
  4. Check timeout → Returns error
  5. Dry-run job parsing → Extracts change description

**Unit Tests** (internal/metrics/metrics_test.go):
  1. Record registry poll → Counter increments
  2. Record job duration → Histogram observes value
  3. Multiple recordings → Metrics aggregate correctly
  4. Metric labels → Proper namespace/bundle labels

**Integration Tests** (controllers/werfbundle_controller_test.go):
  1. Deploy bundle, modify resource, wait → Drift detected within interval
  2. Drift detected, auto-correct enabled → Job created, resource restored
  3. Drift detected, auto-correct disabled → Condition set, no job
  4. Drift correction fails 3 times → DriftCorrectionFailed condition set
  5. Registry update during drift → Registry update takes precedence
  6. Metrics recorded for all operations → Verify metric counts

**E2E Tests** (test/e2e/):
  1. Deploy operator and bundle, verify resources → Baseline
  2. Modify deployed resource → Drift detected
  3. Wait for correction → Resource restored automatically
  4. Scrape /metrics endpoint → Verify metrics exposed

**Implementation approach**:
You need 4 components:

  1. **Drift test helpers**: Create internal/drift/testing.go
    - Helper: `SimulateDrift(client, resource) error` modifies deployed resource
    - Helper: `VerifyDriftDetected(bundle) bool` checks Drifted condition
    - Helper: `WaitForCorrection(client, resource, timeout) error` waits for resource restore

  2. **Metrics test fixtures**: Create internal/metrics/testing.go
    - Mock registry: `type MockRegistry struct { Counts map[string]int; Observations map[string][]float64 }`
    - Helper: `GetMetricValue(name, labels) float64` extracts metric for assertion
    - Helper: `ResetMetrics()` clears metrics between tests

  3. **Time-based test helpers**: Update controllers/suite_test.go
    - Helper: `AdvanceTime(duration)` for fake clock tests
    - Helper: `WaitForReconciliation(bundle, timeout)` waits for requeue
    - Use Eventually() with polling for time-based operations

  4. **Integration test cases**: Update controllers/werfbundle_controller_test.go
    - Use Ginkgo Context for "Drift Detection" suite
    - Test full lifecycle: deploy → drift → detect → correct → synced
    - Assert on conditions using helpers
    - Assert on metrics using mock registry

**Key Go patterns**:
- **Fake time**: `clock := clock.NewFakeClock(time.Now())` then `clock.Step(duration)`
- **Resource modification**: `obj.Spec.Replicas = ptr.To(int32(5)); client.Update(ctx, obj)`
- **Condition assertions**: `Expect(meta.IsStatusConditionTrue(bundle.Status.Conditions, "Drifted")).To(BeTrue())`
- **Metric assertions**: `Expect(mockRegistry.Counts["registry_poll_total"]).To(Equal(5))`

**What success looks like**:
- Drift detection timing verified with fast tests (no real delays)
- State transitions tested for all combinations
- Metrics accurately recorded for all operations
- Correction retry logic prevents infinite loops
- Test coverage > 85% for drift and metrics packages
- You can explain why we use fake time instead of real delays
- You understand how to test asynchronous operations with Eventually()

---

## Task 4: Documentation

**The problem we're solving**: Users need to understand drift detection (what it is, why it matters), configure it appropriately (interval, auto-correct), and monitor it (metrics, conditions). Documentation must explain the concept, provide configuration examples, and show how to observe drift in practice.

**Why explain drift detection concept**:
- Not all users familiar with GitOps/desired state concepts
- Understanding drift helps users decide on auto-correction settings
- Trade-off: Educational content makes docs longer, but reduces misconfigurations
- Operational benefit: Users make informed decisions about drift handling

**Implementation approach**:
You need 4 components:

  1. **README updates**: Update root README.md
    - Add "Drift Detection" section explaining concept
    - Explain manual changes vs desired state
    - Show example with driftDetection enabled and interval configured
    - Show example with auto-correction disabled (detect-only mode)
    - Update "What works now" to include "Automatic drift detection and correction"
    - Update "Next steps" to point to Slice 5 (advanced features)

  2. **Metrics documentation**: Create docs/metrics.md
    - List all custom metrics with descriptions
    - Explain metric types (Counter vs Histogram)
    - Provide example Prometheus queries for common scenarios:
      - Overall success rate: `rate(registry_poll_total{result="success"}[5m])`
      - Job duration p95: `histogram_quantile(0.95, rate(converge_job_duration_seconds_bucket[5m]))`
      - Drift detection rate: `rate(drift_detection_total{drifted="true"}[1h])`
    - Explain how to scrape metrics (ServiceMonitor example)
    - Provide sample Grafana dashboard JSON

  3. **Monitoring guide**: Create docs/monitoring.md
    - Explain Status conditions and how to query them
    - kubectl commands: `kubectl get werfbundle -o jsonpath='{.status.conditions}'`
    - Recommended alert rules:
      - High error rate: `rate(registry_poll_total{result="error"}[5m]) > 0.1`
      - Drift correction failing: `drift_correction_total{result="failure"} > 3`
      - Job duration too long: `histogram_quantile(0.95, converge_job_duration_seconds_bucket) > 600`
    - Link to metrics.md for details

  4. **Troubleshooting additions**: Update docs/troubleshooting.md
    - "Drift always detected" → Check if external system modifying resources, disable auto-correct if intentional
    - "Drift correction fails" → Check job logs, verify bundle is valid, check RBAC
    - "No drift detected" → Verify interval configured, check operator logs for errors
    - "Metrics not updating" → Verify Prometheus scraping, check /metrics endpoint directly

**What success looks like**:
- Users understand what drift detection is and why it's valuable
- Configuration examples show both auto-correct and detect-only modes
- Metrics documentation enables users to build dashboards and alerts
- Alert rule examples cover common failure scenarios
- "What works now" accurately describes drift detection capabilities

## Slice 5: Advanced Features
Add enhanced functionality and polish for production readiness.

---

## Task 1: Advanced Authentication

**The problem we're solving**: Slice 1 only supports access token auth (Bearer tokens). Production registries often use username/password (basic auth), custom TLS certificates (private CAs), or multiple auth methods. We need comprehensive authentication support to work with all common registry types.

**Why support multiple auth methods**:
- Different registries use different auth schemes (Docker Hub: basic, GCR: token, Harbor: both)
- Private registries often use custom TLS certificates for security
- Trade-off: More auth methods means more configuration complexity, but enables more use cases
- Operational benefit: Works with existing infrastructure, no registry migration needed

**Architecture decisions**:
  1. **Auth provider interface**: Abstract auth logic behind interface
    - Why: Enables adding new auth methods without changing core code
    - Alternative: Hardcoding auth methods would require core changes for each new type
    - Interface: `type AuthProvider interface { GetCredentials(ctx, secret) (username, password string, err error) }`

  2. **Multiple auth methods in same Secret**: Support different Secret keys for different auth types
    - Why: Users shouldn't need to change Secrets when switching auth methods
    - Alternative: Separate Secret types would require more user configuration
    - Keys: `token` for Bearer, `username`+`password` for basic auth, `ca.crt` for TLS

  3. **TLS configuration separate from auth**: Add tlsConfig field to registry spec
    - Why: TLS is transport security, auth is identity - separate concerns
    - Alternative: Bundling would conflate two different configuration domains
    - Supports: custom CA cert, skip verify (for testing), client cert auth

  4. **Precedence for multiple auth methods**: Token > username/password > anonymous
    - Why: Token is more secure than password, explicit auth beats implicit
    - Alternative: Trying all methods would be slow and hide configuration errors
    - If multiple present in Secret, use most secure available

  5. **Auth caching**: Cache credentials per registry URL to avoid repeated Secret reads
    - Why: Secret reads are API calls - caching reduces API server load
    - Alternative: Reading every poll would waste resources
    - Cache invalidated on Secret update (watch Secret changes)

**Testing strategy**:
Test these scenarios:
  1. Bearer token auth → Proves token header set correctly
  2. Basic auth (username/password) → Proves Authorization header with base64 encoding
  3. Custom CA certificate → Proves TLS verification with custom cert
  4. Skip TLS verify → Proves insecure connection allowed (test only)
  5. Client certificate auth → Proves mutual TLS works
  6. Multiple auth methods in Secret → Proves precedence (token wins)
  7. Auth cache hit → Proves Secret not read on second poll
  8. Secret update → Proves cache invalidated, new credentials used

**Implementation approach**:
You need 5 components:

  1. **Auth provider interface**: Create internal/registry/auth/provider.go
    - Interface: `type Provider interface { Authenticate(ctx, client, secretRef, namespace) (Credentials, error) }`
    - Type: `type Credentials struct { Token string; Username string; Password string; TLSConfig *tls.Config }`
    - Implementations: TokenProvider, BasicAuthProvider, TLSProvider
    - Factory: `NewProvider(authType) Provider` returns appropriate implementation

  2. **WerfBundleSpec additions**: Update api/v1alpha1/werfbundle_types.go
    - Add `TLSConfig` struct to Registry spec
    - Fields: `CASecretRef *LocalObjectReference`, `InsecureSkipVerify bool`, `ClientCertSecretRef *LocalObjectReference`
    - Validation: CASecretRef and InsecureSkipVerify are mutually exclusive
    - Add comments explaining security implications of InsecureSkipVerify

  3. **Enhanced registry client**: Update internal/registry/client.go
    - Accept Credentials parameter in constructor
    - Build http.Client with custom TLS config if provided
    - Set Authorization header based on credentials type (Bearer vs Basic)
    - Handle auth errors (401/403) distinctly from other errors

  4. **Auth cache**: Create internal/registry/auth/cache.go
    - Type: `type Cache struct { entries map[string]*CacheEntry; mutex sync.RWMutex }`
    - Method: `Get(key string) (Credentials, bool)` returns cached credentials
    - Method: `Set(key string, creds Credentials, ttl time.Duration)` stores credentials
    - Method: `Invalidate(secretName string)` clears entries for Secret
    - Watch Secrets and invalidate cache on update

  5. **Reconciler integration**: Update controllers/werfbundle_controller.go
    - Before registry operations, resolve auth using auth provider
    - Check cache first, fall back to Secret read if miss
    - Pass Credentials to registry client
    - Handle auth errors with clear status messages
    - Invalidate cache if registry returns 401 (credentials may have expired)

**Key Go patterns**:
- **Interface-based design**: `type Provider interface { Authenticate(...) (Credentials, error) }` enables polymorphism
- **TLS config**: `tls.Config{RootCAs: certPool, InsecureSkipVerify: false}` for custom CA
- **Client cert**: `tls.Config{Certificates: []tls.Certificate{cert}}` for mutual TLS
- **Basic auth encoding**: `base64.StdEncoding.EncodeToString([]byte(username + ":" + password))`
- **Cache with mutex**: `sync.RWMutex` for concurrent safe cache access

**What success looks like**:
- Username/password auth works with Docker Hub, Harbor, Artifactory
- Custom CA certificates enable private registry access
- Auth cache reduces Secret reads by 90%+
- Clear error messages distinguish auth failures from network failures
- You can explain why we separate TLS config from auth credentials
- You understand the security implications of InsecureSkipVerify

---

## Task 2: Version Constraint and Bundle Management

**The problem we're solving**: Currently operator deploys latest tag found in registry. Production needs version pinning (deploy only v1.x.x), constraint matching (deploy >=1.2.0 <2.0.0), and manual rollback (revert to previous version). We need semver-aware version selection and rollback support.

**Why semver constraints matter**:
- Production shouldn't auto-upgrade to breaking changes (v1 → v2)
- Testing environments want latest patches (1.2.x) but not minor/major updates
- Trade-off: Version parsing adds complexity, but prevents accidental breaking updates
- Operational benefit: Safe automated updates - only compatible versions deployed

**Architecture decisions**:
  1. **Use semver constraint syntax**: Standard syntax like `>=1.2.0 <2.0.0`
    - Why: Industry standard used by npm, cargo, helm - familiar to users
    - Alternative: Custom syntax would require learning and be error-prone
    - Library: Use github.com/Masterminds/semver for parsing and matching

  2. **Version constraint in CRD spec**: Add versionConstraint field to registry spec
    - Why: Explicit configuration - users declare what versions are acceptable
    - Alternative: Implicit latest selection is dangerous in production
    - Empty constraint means "latest" (backward compatible with Slice 1-4)

  3. **Track version history in Status**: Store last N deployed versions
    - Why: Enables rollback and audit trail
    - Alternative: External tracking would require additional infrastructure
    - Limit to 10 versions to prevent Status size explosion

  4. **Manual rollback via annotation**: Add `werf.io/rollback-to-version` annotation
    - Why: Annotations are standard Kubernetes mechanism for one-time operations
    - Alternative: Separate Rollback CRD would be overkill for simple use case
    - Controller removes annotation after processing (one-shot operation)

  5. **Bundle validation before deployment**: Parse bundle manifest to check health
    - Why: Catch invalid bundles before creating jobs
    - Alternative: Letting werf fail wastes time and creates confusing errors
    - Check: Bundle exists, manifest parseable, required resources valid

**Testing strategy**:
Test these scenarios:
  1. Constraint `>=1.0.0` with tags 0.9.0, 1.0.0, 1.1.0 → Selects 1.1.0
  2. Constraint `~1.2.0` with tags 1.1.9, 1.2.5, 1.3.0 → Selects 1.2.5 (latest 1.2.x)
  3. Constraint `<2.0.0` with tags 1.9.9, 2.0.0, 2.1.0 → Selects 1.9.9
  4. Invalid version in registry → Skipped, valid versions selected
  5. No versions match constraint → Status shows "No matching versions"
  6. Rollback annotation added → Previous version deployed
  7. Version history tracks deployments → Status shows last 10 versions
  8. Invalid bundle manifest → Validation fails before job creation

**Implementation approach**:
You need 5 components:

  1. **WerfBundleSpec additions**: Update api/v1alpha1/werfbundle_types.go
    - Add `VersionConstraint string` field to Registry spec
    - Add kubebuilder validation: must be valid semver constraint or empty
    - Add example in comments: `">=1.2.0 <2.0.0"` for patch updates

  2. **WerfBundleStatus additions**: Update api/v1alpha1/werfbundle_types.go
    - Add `VersionHistory []VersionRecord` field
    - Type: `type VersionRecord struct { Version string; DeployedAt metav1.Time; JobName string }`
    - Limit to 10 entries (evict oldest when adding 11th)

  3. **Version selector**: Create internal/version/selector.go
    - Function: `SelectVersion(tags []string, constraint string) (string, error)`
    - Parse constraint using semver library
    - Filter tags to valid semver versions (skip non-semver tags)
    - Apply constraint to filter matching versions
    - Return highest matching version
    - Handle empty constraint as "latest"

  4. **Bundle validator**: Create internal/bundle/validator.go
    - Function: `ValidateBundle(ctx, registryClient, url, tag) error`
    - Download bundle manifest
    - Parse YAML to check structure
    - Validate required fields present
    - Check for obvious errors (invalid resource types, syntax errors)
    - Return descriptive error if validation fails

  5. **Rollback handler**: Update controllers/werfbundle_controller.go
    - Check for `werf.io/rollback-to-version` annotation
    - If present: validate version exists in history
    - Create converge job for specified version
    - Remove annotation after processing (use Patch to avoid race)
    - Update status with rollback operation details

**Key Go patterns**:
- **Semver parsing**: `constraint, err := semver.NewConstraint(">=1.2.0")` then `constraint.Check(version)`
- **Version sorting**: Use `semver.Collection` with `Sort()` for ordering
- **Annotation handling**: Check `bundle.Annotations["werf.io/rollback-to-version"]` in reconcile
- **History management**: Append new version, slice `history[:10]` to trim
- **YAML validation**: Use `yaml.Unmarshal(data, &manifest)` to check parseability

**What success looks like**:
- Version constraints prevent unwanted major version upgrades
- Only compatible versions deployed automatically
- Rollback to previous version works via annotation
- Version history visible in status for audit
- Invalid bundles caught before job creation
- You can explain the difference between `~1.2.0` and `^1.2.0` constraints
- You understand why we limit version history to 10 entries

---

## Task 3: Testing

**The problem we're solving**: Advanced auth (TLS, multiple methods) and version management (constraints, rollback) have many edge cases. We need comprehensive tests covering all auth combinations, version constraint scenarios, and rollback flows.

**Why test auth thoroughly**:
- Auth bugs leak credentials or block legitimate access - both are security issues
- Different registries have subtle auth requirements (trailing slash, realm, etc.)
- Trade-off: Auth tests need real or very realistic mocks, more setup complexity
- Operational benefit: Confidence that auth works with production registries

**Architecture decisions**:
  1. **Use test registries for auth tests**: Run actual registry containers with different auth configs
    - Why: Auth protocols are complex - mocking misses subtle implementation details
    - Alternative: HTTP mocks work for unit tests but miss real protocol quirks
    - Use Docker registry image with htpasswd, TLS certs for realistic testing

  2. **Table-driven version tests**: Matrix of constraints and tag lists
    - Why: Version logic is pure function with many input combinations
    - Alternative: Individual tests would be verbose and miss edge cases
    - Each row: (tags, constraint, expected selection)

  3. **Rollback integration test**: Full deploy → rollback → verify old version
    - Why: Rollback involves annotation → reconcile → job → status update - need full flow
    - Alternative: Unit testing each piece wouldn't catch integration issues
    - Use envtest for full Kubernetes API semantics

  4. **TLS certificate generation in tests**: Create self-signed certs for TLS tests
    - Why: Need valid certificates to test TLS code paths
    - Alternative: Skipping TLS tests would leave major feature untested
    - Use crypto/x509 to generate test certs on the fly

**Testing strategy**:
Test these scenarios:

**Unit Tests** (internal/registry/auth/provider_test.go):
  1. Token auth → Proves Bearer header set
  2. Basic auth → Proves Authorization header with base64
  3. Multiple auth methods → Proves precedence (token wins)
  4. Auth cache hit → Proves credentials retrieved from cache
  5. Auth cache miss → Proves Secret read
  6. Secret update → Proves cache invalidated

**Unit Tests** (internal/version/selector_test.go):
  1. Constraint `>=1.0.0` → Selects highest matching version
  2. Constraint `~1.2.0` → Selects latest 1.2.x
  3. Constraint `^1.0.0` → Selects latest 1.x.x
  4. Empty constraint → Selects absolute latest
  5. No matching versions → Returns error
  6. Invalid semver tags → Skipped, valid tags processed
  7. Pre-release versions → Handled according to semver rules

**Integration Tests** (controllers/werfbundle_controller_test.go):
  1. Deploy with version constraint → Correct version selected
  2. New version published matching constraint → Auto-deploys
  3. New version published not matching → Ignored
  4. Rollback annotation added → Previous version deployed
  5. Rollback to non-existent version → Error in status
  6. Version history tracks deployments → Status has correct history
  7. Auth with username/password → Registry accessed successfully
  8. Auth with custom CA → TLS verification succeeds

**E2E Tests** (test/e2e/):
  1. Deploy with real private registry (Harbor/Artifactory) → Full auth flow works
  2. Version constraint with multiple tags → Correct auto-selection
  3. Rollback via annotation → Previous version restored
  4. Bundle validation catches invalid manifests → Job not created

**Implementation approach**:
You need 4 components:

  1. **Auth test fixtures**: Create test/fixtures/auth/
    - Generate self-signed CA and certificates: `openssl req -x509 -newkey rsa:4096 ...`
    - Create htpasswd file for basic auth: `htpasswd -Bbn user pass`
    - Create test Secrets with various auth combinations
    - Helper: `StartTestRegistry(auth, tls) (*Registry, cleanup func())` for auth tests

  2. **Version test cases**: Create internal/version/selector_test.go
    - Table-driven tests with comprehensive constraint coverage
    - Test edge cases: empty tags, malformed versions, pre-releases
    - Verify error messages are helpful

  3. **Rollback test helpers**: Create controllers/rollback_test_helpers.go
    - Helper: `AddRollbackAnnotation(client, bundle, version) error`
    - Helper: `WaitForVersion(client, bundle, expectedVersion, timeout) error`
    - Helper: `VerifyVersionHistory(bundle, expectedVersions) error`

  4. **Integration test suite**: Update controllers/werfbundle_controller_test.go
    - Create "Advanced Auth" test context
    - Create "Version Management" test context
    - Create "Rollback" test context
    - Use BeforeEach to set up auth fixtures and test registry
    - Use AfterEach to clean up test registry

**Key Go patterns**:
- **Test registry**: `httptest.NewServer()` or real Docker registry via testcontainers
- **Certificate generation**: `x509.CreateCertificate()` for self-signed certs in tests
- **Version assertions**: `Expect(selectedVersion).To(Equal("1.2.5"))`
- **Annotation handling in tests**: `bundle.Annotations["werf.io/rollback-to-version"] = "1.0.0"`

**What success looks like**:
- All auth methods tested against realistic registry setups
- Version constraint logic verified with exhaustive test matrix
- Rollback flow tested end-to-end
- TLS configuration tested with real certificates
- Test coverage > 85% for auth and version packages
- You can explain why we use real registries instead of mocks for auth tests
- You understand the tradeoff between test speed and test realism

---

## Task 4: Documentation

**The problem we're solving**: Users need comprehensive documentation for production deployment. Must cover all auth methods, version constraints, rollback procedures, and operational best practices. Documentation should guide users from dev to production with confidence.

**Why production deployment guide is critical**:
- Production has different requirements than dev (HA, monitoring, security, backup)
- Users need to understand operational aspects (upgrades, troubleshooting, capacity planning)
- Trade-off: Comprehensive docs are long, but incomplete docs cause production incidents
- Operational benefit: Self-service production deployment reduces support burden

**Implementation approach**:
You need 6 components:

  1. **README updates**: Update root README.md
    - Add "Authentication" section with all auth method examples
    - Add "Version Management" section with constraint syntax reference
    - Add "Rollback" section with annotation example
    - Update "What works now" to list all production-ready features
    - Add "Production Readiness" section summarizing NFRs
    - Add "Upgrading" section for migrating from v1alpha1 to v1beta1 (if promoting API)

  2. **Authentication guide**: Create docs/authentication.md
    - Section for each auth type: Bearer token, Basic auth, Custom CA, Client certs
    - Example Secret YAML for each type
    - Security best practices (rotate credentials, use RBAC, avoid insecure-skip-verify)
    - Troubleshooting auth failures (check logs, verify Secret, test credentials manually)
    - Registry-specific notes (Docker Hub, GCR, ECR, Harbor, Artifactory)

  3. **Version management guide**: Create docs/versioning.md
    - Semver constraint syntax reference with examples
    - Explain `~`, `^`, `>=`, `<`, ranges
    - Show common patterns: patch updates only, minor updates, pre-releases
    - Rollback procedure with kubectl commands
    - Version history visibility and audit
    - Best practices: test constraints in dev, monitor version updates

  4. **Production deployment guide**: Create docs/production.md
    - Deployment topology (operator namespace, app namespaces)
    - High availability: operator replicas, leader election
    - Monitoring setup: Prometheus, Grafana, alert rules
    - Backup and recovery: CRD backup, version history, disaster recovery
    - Security hardening: RBAC review, network policies, Secret management
    - Capacity planning: resource limits, scale estimates
    - Upgrade procedure: operator upgrade, CRD migration, rollback plan

  5. **Complete feature matrix**: Update README.md
    - Table showing all features and which version introduced them
    - Mark which are stable vs alpha vs beta
    - Link to detailed docs for each feature
    - Example:
      ```
      | Feature | Status | Docs |
      |---------|--------|------|
      | Basic deployment | Stable | - |
      | ETag caching | Stable | configuration.md |
      | Values management | Stable | examples/ |
      | Drift detection | Beta | drift-detection.md |
      | Version constraints | Beta | versioning.md |
      ```

  6. **API version promotion**: If promoting to v1beta1
    - Create api/v1beta1/werfbundle_types.go
    - Copy from v1alpha1 with any breaking changes documented
    - Add conversion webhook between v1alpha1 and v1beta1
    - Update docs to reference v1beta1
    - Deprecation notice in v1alpha1 docs (supported for 2 more releases)
    - Migration guide for users on v1alpha1

**What success looks like**:
- Users can deploy to production following documentation alone
- All auth scenarios documented with working examples
- Version constraint syntax clearly explained with examples
- Production best practices comprehensive and actionable
- Feature matrix shows complete capability overview
- API stability indicated (v1alpha1 vs v1beta1)
- Migration path documented for API version upgrades

## Development Guidelines

1. Each slice should be implemented in its own branch
2. Write tests before implementing features (TDD approach)
3. Regular commits with clear messages
4. Maintain metrics and logging throughout
5. Keep README.md synchronized with actual capabilities (no aspirational features)

## Initial Focus
Starting with Slice 1, we should:

1. Set up the project structure
2. Implement the basic CRD
3. Create a simple controller that can poll a registry
4. Get a basic werf converge job running
