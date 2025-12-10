# Test Fixtures for Values Integration

This directory contains reusable YAML test fixtures for ConfigMaps and Secrets used in values integration tests.

## Purpose

Test fixtures provide realistic configuration data for integration tests without manually constructing Kubernetes objects in test code. Fixtures:

- Make tests more readable: `LoadConfigMapFixture("simple")` vs 50 lines of object construction
- Are reusable across multiple test cases
- Can be applied to real clusters for manual testing
- Clearly show what test data looks like (YAML is human-readable)

## Directory Structure

```
testdata/
├── configmaps/      # ConfigMap fixtures
├── secrets/         # Secret fixtures
└── README.md        # This file
```

## ConfigMap Fixtures

### `simple-values.yaml`
Basic configuration with 3-5 simple key-value pairs. Tests basic YAML parsing.
- Keys: `app.name`, `app.replicas`, `app.environment`
- Use case: Testing simple value resolution

### `nested-values.yaml`
Nested YAML structure demonstrating complex key flattening. Tests dot-notation flattening.
- Keys: `database.host`, `database.port`, `database.ssl.enabled`, etc.
- Use case: Testing complex YAML parsing with nested maps

### `override-values.yaml`
ConfigMap with keys that overlap with `simple-values.yaml` but with different values. Tests merge precedence.
- Keys: `app.name` (different value), `app.replicas` (different), plus unique keys like `monitoring.enabled`
- Use case: Testing that later sources override earlier ones

### `multi-file-values.yaml`
Multiple YAML documents in separate data keys. Tests behavior with multiple value files.
- Keys: `config1.yaml`, `config2.yaml` (separate YAML documents)
- Use case: Testing ConfigMaps with multiple configuration files

### `special-chars-values.yaml`
Values containing Helm special characters that need escaping. Tests CLI flag generation.
- Values: comma-separated lists, equals signs, brackets, backslashes, URLs
- Use case: Testing that special characters are properly escaped for `--set` flags

## Secret Fixtures

### `database-credentials.yaml`
Secret with base64-encoded database credentials. Tests Secret value resolution.
- Keys: `db.username`, `db.password`, `db.host`, `db.port`
- Use case: Testing values resolution from Secrets and mixing ConfigMaps with Secrets

## Naming Convention

Fixture files follow this pattern: `{description}-{type}.yaml`

- `simple-values.yaml` - simple configuration
- `nested-values.yaml` - nested/complex configuration
- `database-credentials.yaml` - credentials/sensitive data

## Using Fixtures in Tests

### Loading a ConfigMap Fixture

```go
import "github.com/yourorg/k8s-werf-operator-go/internal/values/testdata"

// Load fixture
cm, err := testdata.LoadConfigMapFixture("simple-values.yaml")
if err != nil {
    t.Fatalf("failed to load fixture: %v", err)
}

// Assign namespace if needed
cm.Namespace = "test-ns"

// Use in test
client := fake.NewClientBuilder().WithObjects(cm).Build()
```

### Loading a Secret Fixture

```go
// Load fixture
secret, err := testdata.LoadSecretFixture("database-credentials.yaml")
if err != nil {
    t.Fatalf("failed to load fixture: %v", err)
}

// Assign namespace if needed
secret.Namespace = "test-ns"

// Use in test
client := fake.NewClientBuilder().WithObjects(secret).Build()
```

### Using Helper for Namespace Assignment

```go
cm := testdata.LoadConfigMapFixture("simple-values.yaml")
cm = testdata.WithNamespace(cm, "test-ns").(*corev1.ConfigMap)
```

## When to Use Fixtures vs Inline Data

Use fixtures when:
- Test data is realistic and reusable
- The same test data is used in multiple tests
- The data is complex or verbose (nested YAML, multiple keys)
- You want to be able to apply the data to real clusters manually

Use inline data when:
- Test data is minimal (1-2 simple values)
- The data is test-specific and not reused
- You need to dynamically construct data based on test parameters

## Adding New Fixtures

When adding a new fixture:

1. Create the YAML file in the appropriate subdirectory (configmaps/ or secrets/)
2. Use a descriptive filename that clearly indicates the fixture's purpose
3. Include comments in the YAML explaining what the fixture tests
4. Ensure the YAML is valid Kubernetes format (apiVersion, kind, metadata, data/stringData)
5. For Secrets, ensure values are base64-encoded where appropriate
6. Add the fixture filename to the validation test in `validation_test.go`
7. Document the fixture in this README

## Validation

All fixtures are validated by `validation_test.go` to ensure they are:
- Valid Kubernetes YAML
- Have required fields (apiVersion, kind, metadata, name)
- Contain expected data sections
