# Configuration Values Examples

This directory contains complete, working examples demonstrating how to use ConfigMaps and Secrets to provide configuration values to your Werf operator deployments.

## Overview

The Werf operator supports external configuration through Kubernetes ConfigMaps and Secrets. This allows you to:
- Provide environment-specific values without rebuilding bundles
- Deploy the same bundle to development, staging, and production with different configurations
- Keep sensitive data (credentials, API keys) separate from application config
- Override specific values while keeping base defaults

## Quick Start

If you're new to the Werf operator's values management:

1. Start with **values-basic-configmap.yaml** - Learn the simplest pattern
2. Move to **values-precedence.yaml** - Understand how multiple sources merge
3. Try **values-configmap-and-secret.yaml** - Mix configuration types
4. Explore **values-optional-secret.yaml** - Handle environment-specific overrides

## Examples in This Directory

| Example | What It Demonstrates | When to Use |
|---------|---------------------|-------------|
| [values-basic-configmap.yaml](values-basic-configmap.yaml) | Single ConfigMap providing values | Starting point - simplest values pattern |
| [values-precedence.yaml](values-precedence.yaml) | Multiple ConfigMaps with merge precedence | Base config + environment-specific overrides |
| [values-configmap-and-secret.yaml](values-configmap-and-secret.yaml) | Mixing ConfigMap and Secret sources | Separating config from credentials |
| [values-optional-secret.yaml](values-optional-secret.yaml) | Optional sources with graceful fallback | Environment-specific config that might not exist |

### values-basic-configmap.yaml

The simplest pattern: one ConfigMap with application configuration. Perfect for learning how the `valuesFrom` field works and understanding how values become `--set` flags.

**Key concepts:**
- Required `values.yaml` key in ConfigMap
- How YAML values flatten to dot notation
- Basic `valuesFrom` array structure

### values-precedence.yaml

Demonstrates merge behavior when multiple ConfigMaps are specified. Shows a realistic pattern of base configuration plus environment-specific overrides.

**Key concepts:**
- Array order determines precedence (later wins)
- Overlapping keys are overridden
- Non-overlapping keys are preserved
- Common pattern: base-config + production-overrides

### values-configmap-and-secret.yaml

Shows how to mix ConfigMap (for non-sensitive data) and Secret (for credentials) in the same WerfBundle.

**Key concepts:**
- When to use ConfigMap vs Secret
- Proper base64 encoding for Secret data
- Both sources merge together
- Best practices for sensitive data

### values-optional-secret.yaml

Demonstrates the `optional: true` flag for handling values sources that may or may not exist, allowing the same WerfBundle to work across multiple environments.

**Key concepts:**
- Required vs optional sources
- Deployment continues when optional source is missing
- Pattern for environment-agnostic deployments
- Same WerfBundle in dev/staging/production

## Before You Apply

These examples are ready to use, but you'll need to:

1. Update the registry URL in each WerfBundle to point to your actual OCI registry
2. Ensure the target namespace exists (most examples use `production`)
3. Create the required ServiceAccount with appropriate RBAC permissions

For detailed RBAC setup instructions, see [docs/job-rbac.md](../docs/job-rbac.md).

## Learn More

- [Configuration Reference](../docs/configuration.md) - Complete field documentation
- [README](../README.md) - Operator overview and quick start
- [RBAC Setup Guide](../docs/job-rbac.md) - Detailed RBAC configuration
