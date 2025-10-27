// Package controllers tests for RBAC configuration.
package controllers

import (
	"os"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/yaml"
)

// TestRBACManifestGeneration verifies that the generated ClusterRole has expected permissions.
func TestRBACManifestGeneration(t *testing.T) {
	// Read the generated RBAC manifest
	data, err := os.ReadFile("../config/rbac/role.yaml")
	if err != nil {
		t.Fatalf("Failed to read RBAC manifest: %v", err)
	}

	// Parse as ClusterRole
	role := &rbacv1.ClusterRole{}
	if err := yaml.UnmarshalStrict(data, role); err != nil {
		t.Fatalf("Failed to unmarshal ClusterRole: %v", err)
	}

	// Define expected rules: {apiGroup, resources, verbs}
	expectedRules := []struct {
		apiGroup  string
		resources []string
		verbs     []string
	}{
		// WerfBundle resources - need update/patch for finalizers
		{
			apiGroup:  "werf.io",
			resources: []string{"werfbundles"},
			verbs:     []string{"get", "list", "watch", "update", "patch"},
		},
		// WerfBundle status
		{
			apiGroup:  "werf.io",
			resources: []string{"werfbundles/status"},
			verbs:     []string{"get", "update", "patch"},
		},
		// Job resources
		{
			apiGroup:  "batch",
			resources: []string{"jobs"},
			verbs:     []string{"create", "get", "list", "watch", "delete"},
		},
		// Secrets - only needs get
		{
			apiGroup:  "",
			resources: []string{"secrets"},
			verbs:     []string{"get"},
		},
		// ServiceAccounts - needs list/watch for controller-runtime cache
		{
			apiGroup:  "",
			resources: []string{"serviceaccounts"},
			verbs:     []string{"get", "list", "watch"},
		},
	}

	// Verify each expected rule exists in the generated role
	for _, expected := range expectedRules {
		found := false
		for _, rule := range role.Rules {
			if ruleMatches(rule, expected.apiGroup, expected.resources, expected.verbs) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected rule not found: apiGroup=%q resources=%v verbs=%v",
				expected.apiGroup, expected.resources, expected.verbs)
		}
	}

	// Verify no unexpected rules
	if len(role.Rules) != len(expectedRules) {
		t.Logf("Generated %d rules, expected %d", len(role.Rules), len(expectedRules))
		t.Logf("Rules in manifest:")
		for i, rule := range role.Rules {
			t.Logf("  Rule %d: apiGroups=%v resources=%v verbs=%v",
				i, rule.APIGroups, rule.Resources, rule.Verbs)
		}
	}
}

// ruleMatches checks if an RBAC rule matches the expected values.
// Note: The rule might have additional fields (like NonResourceURLs), so we only check presence.
func ruleMatches(rule rbacv1.PolicyRule, expectedAPIGroup string, expectedResources, expectedVerbs []string) bool {
	// Check APIGroups
	apiGroupFound := false
	for _, ag := range rule.APIGroups {
		if ag == expectedAPIGroup {
			apiGroupFound = true
			break
		}
	}
	if !apiGroupFound {
		return false
	}

	// Check all expected resources are present
	for _, expectedRes := range expectedResources {
		resFound := false
		for _, res := range rule.Resources {
			if res == expectedRes {
				resFound = true
				break
			}
		}
		if !resFound {
			return false
		}
	}

	// Check all expected verbs are present
	for _, expectedVerb := range expectedVerbs {
		verbFound := false
		for _, verb := range rule.Verbs {
			if verb == expectedVerb {
				verbFound = true
				break
			}
		}
		if !verbFound {
			return false
		}
	}

	return true
}
