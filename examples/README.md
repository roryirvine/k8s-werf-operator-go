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

More examples will be added as they're created.

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
