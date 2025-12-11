package controllers

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testingutil "github.com/werf/k8s-werf-operator-go/internal/testing"
)

func TestCreateTestNamespace_CreatesNamespace(t *testing.T) {
	ctx := context.Background()

	ns, err := testingutil.CreateTestNamespace(ctx, testk8sClient, "test-namespace")
	if err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}

	if ns == nil {
		t.Fatal("expected namespace to be returned, got nil")
	}

	if ns.Name != "test-namespace" {
		t.Errorf("expected namespace name 'test-namespace', got '%s'", ns.Name)
	}

	// Verify namespace exists in API server
	retrieved := &corev1.Namespace{}
	if err := testk8sClient.Get(ctx, client.ObjectKey{Name: "test-namespace"}, retrieved); err != nil {
		t.Errorf("failed to retrieve created namespace: %v", err)
	}

	// Cleanup
	if err := testk8sClient.Delete(ctx, ns); err != nil {
		t.Errorf("failed to delete namespace: %v", err)
	}
}

func TestCreateTestNamespace_UniqueName(t *testing.T) {
	ctx := context.Background()

	// Create two namespaces with different names
	ns1, err := testingutil.CreateTestNamespace(ctx, testk8sClient, "test-ns-helper-1")
	if err != nil {
		t.Fatalf("failed to create first namespace: %v", err)
	}
	defer testk8sClient.Delete(ctx, ns1)

	ns2, err := testingutil.CreateTestNamespace(ctx, testk8sClient, "test-ns-helper-2")
	if err != nil {
		t.Fatalf("failed to create second namespace: %v", err)
	}
	defer testk8sClient.Delete(ctx, ns2)

	if ns1.Name == ns2.Name {
		t.Errorf("expected different namespace names, got both '%s'", ns1.Name)
	}
}

func TestCreateTestServiceAccount_CreatesServiceAccount(t *testing.T) {
	ctx := context.Background()

	// Create namespace first
	ns, err := testingutil.CreateTestNamespace(ctx, testk8sClient, "test-sa-ns")
	if err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}
	defer testk8sClient.Delete(ctx, ns)

	// Create ServiceAccount
	sa, err := testingutil.CreateTestServiceAccount(ctx, testk8sClient, "test-sa-ns", "test-sa")
	if err != nil {
		t.Fatalf("failed to create ServiceAccount: %v", err)
	}

	if sa == nil {
		t.Fatal("expected ServiceAccount to be returned, got nil")
	}

	if sa.Name != "test-sa" {
		t.Errorf("expected SA name 'test-sa', got '%s'", sa.Name)
	}

	if sa.Namespace != "test-sa-ns" {
		t.Errorf("expected SA namespace 'test-sa-ns', got '%s'", sa.Namespace)
	}

	// Verify SA exists in API server
	retrieved := &corev1.ServiceAccount{}
	if err := testk8sClient.Get(ctx, client.ObjectKey{Name: "test-sa", Namespace: "test-sa-ns"}, retrieved); err != nil {
		t.Errorf("failed to retrieve created ServiceAccount: %v", err)
	}

	// Cleanup
	if err := testk8sClient.Delete(ctx, sa); err != nil {
		t.Errorf("failed to delete ServiceAccount: %v", err)
	}
}

func TestCreateTestServiceAccount_CorrectNamespace(t *testing.T) {
	ctx := context.Background()

	// Create two namespaces
	ns1, err := testingutil.CreateTestNamespace(ctx, testk8sClient, "test-sa-ns-helper-1")
	if err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}
	defer testk8sClient.Delete(ctx, ns1)

	ns2, err := testingutil.CreateTestNamespace(ctx, testk8sClient, "test-sa-ns-helper-2")
	if err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}
	defer testk8sClient.Delete(ctx, ns2)

	// Create SAs in different namespaces
	sa1, err := testingutil.CreateTestServiceAccount(ctx, testk8sClient, "test-sa-ns-helper-1", "deployer")
	if err != nil {
		t.Fatalf("failed to create ServiceAccount in ns1: %v", err)
	}
	defer testk8sClient.Delete(ctx, sa1)

	sa2, err := testingutil.CreateTestServiceAccount(ctx, testk8sClient, "test-sa-ns-helper-2", "deployer")
	if err != nil {
		t.Fatalf("failed to create ServiceAccount in ns2: %v", err)
	}
	defer testk8sClient.Delete(ctx, sa2)

	// Verify they're in correct namespaces
	if sa1.Namespace != "test-sa-ns-helper-1" {
		t.Errorf("expected sa1 in 'test-sa-ns-helper-1', got '%s'", sa1.Namespace)
	}
	if sa2.Namespace != "test-sa-ns-helper-2" {
		t.Errorf("expected sa2 in 'test-sa-ns-helper-2', got '%s'", sa2.Namespace)
	}
}

func TestCreateTestRole_CreatesRole(t *testing.T) {
	ctx := context.Background()

	// Create namespace first
	ns, err := testingutil.CreateTestNamespace(ctx, testk8sClient, "test-role-ns")
	if err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}
	defer testk8sClient.Delete(ctx, ns)

	// Define permissions
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}

	// Create Role
	role, err := testingutil.CreateTestRole(ctx, testk8sClient, "test-role-ns", "deployer", rules)
	if err != nil {
		t.Fatalf("failed to create Role: %v", err)
	}

	if role == nil {
		t.Fatal("expected Role to be returned, got nil")
	}

	if role.Name != "deployer" {
		t.Errorf("expected role name 'deployer', got '%s'", role.Name)
	}

	if len(role.Rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(role.Rules))
	}

	// Verify Role exists in API server
	retrieved := &rbacv1.Role{}
	if err := testk8sClient.Get(ctx, client.ObjectKey{Name: "deployer", Namespace: "test-role-ns"}, retrieved); err != nil {
		t.Errorf("failed to retrieve created Role: %v", err)
	}

	// Cleanup
	if err := testk8sClient.Delete(ctx, role); err != nil {
		t.Errorf("failed to delete Role: %v", err)
	}
}

func TestCreateTestRole_IncludesRules(t *testing.T) {
	ctx := context.Background()

	// Create namespace
	ns, err := testingutil.CreateTestNamespace(ctx, testk8sClient, "test-rule-ns")
	if err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}
	defer testk8sClient.Delete(ctx, ns)

	// Define multiple permissions
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments"},
			Verbs:     []string{"get", "list", "watch", "create", "update"},
		},
		{
			APIGroups: []string{"batch"},
			Resources: []string{"jobs"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}

	// Create Role
	role, err := testingutil.CreateTestRole(ctx, testk8sClient, "test-rule-ns", "test-role", rules)
	if err != nil {
		t.Fatalf("failed to create Role: %v", err)
	}
	defer testk8sClient.Delete(ctx, role)

	if len(role.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(role.Rules))
	}

	// Verify first rule
	if len(role.Rules[0].APIGroups) != 1 || role.Rules[0].APIGroups[0] != "apps" {
		t.Errorf("expected first rule APIGroup 'apps'")
	}

	// Verify second rule
	if len(role.Rules[1].APIGroups) != 1 || role.Rules[1].APIGroups[0] != "batch" {
		t.Errorf("expected second rule APIGroup 'batch'")
	}
}

func TestCreateTestRoleBinding_CreatesRoleBinding(t *testing.T) {
	ctx := context.Background()

	// Create namespace
	ns, err := testingutil.CreateTestNamespace(ctx, testk8sClient, "test-rb-ns")
	if err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}
	defer testk8sClient.Delete(ctx, ns)

	// Create ServiceAccount
	sa, err := testingutil.CreateTestServiceAccount(ctx, testk8sClient, "test-rb-ns", "deployer")
	if err != nil {
		t.Fatalf("failed to create ServiceAccount: %v", err)
	}
	defer testk8sClient.Delete(ctx, sa)

	// Create Role
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments"},
			Verbs:     []string{"*"},
		},
	}
	role, err := testingutil.CreateTestRole(ctx, testk8sClient, "test-rb-ns", "deployer", rules)
	if err != nil {
		t.Fatalf("failed to create Role: %v", err)
	}
	defer testk8sClient.Delete(ctx, role)

	// Create RoleBinding
	rb, err := testingutil.CreateTestRoleBinding(ctx, testk8sClient, "test-rb-ns", "deployer", "deployer")
	if err != nil {
		t.Fatalf("failed to create RoleBinding: %v", err)
	}

	if rb == nil {
		t.Fatal("expected RoleBinding to be returned, got nil")
	}

	// Verify RoleBinding exists in API server
	retrieved := &rbacv1.RoleBinding{}
	if err := testk8sClient.Get(ctx, client.ObjectKey{Name: rb.Name, Namespace: "test-rb-ns"}, retrieved); err != nil {
		t.Errorf("failed to retrieve created RoleBinding: %v", err)
	}

	// Cleanup
	if err := testk8sClient.Delete(ctx, rb); err != nil {
		t.Errorf("failed to delete RoleBinding: %v", err)
	}
}

func TestCreateTestRoleBinding_CorrectSubject(t *testing.T) {
	ctx := context.Background()

	// Create namespace
	ns, err := testingutil.CreateTestNamespace(ctx, testk8sClient, "test-subject-ns")
	if err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}
	defer testk8sClient.Delete(ctx, ns)

	// Create ServiceAccount
	sa, err := testingutil.CreateTestServiceAccount(ctx, testk8sClient, "test-subject-ns", "my-sa")
	if err != nil {
		t.Fatalf("failed to create ServiceAccount: %v", err)
	}
	defer testk8sClient.Delete(ctx, sa)

	// Create Role
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list"},
		},
	}
	role, err := testingutil.CreateTestRole(ctx, testk8sClient, "test-subject-ns", "my-role", rules)
	if err != nil {
		t.Fatalf("failed to create Role: %v", err)
	}
	defer testk8sClient.Delete(ctx, role)

	// Create RoleBinding
	rb, err := testingutil.CreateTestRoleBinding(ctx, testk8sClient, "test-subject-ns", "my-role", "my-sa")
	if err != nil {
		t.Fatalf("failed to create RoleBinding: %v", err)
	}
	defer testk8sClient.Delete(ctx, rb)

	// Verify subject
	if len(rb.Subjects) != 1 {
		t.Errorf("expected 1 subject, got %d", len(rb.Subjects))
	}

	subject := rb.Subjects[0]
	if subject.Kind != "ServiceAccount" {
		t.Errorf("expected subject Kind 'ServiceAccount', got '%s'", subject.Kind)
	}
	if subject.Name != "my-sa" {
		t.Errorf("expected subject Name 'my-sa', got '%s'", subject.Name)
	}
	if subject.Namespace != "test-subject-ns" {
		t.Errorf("expected subject Namespace 'test-subject-ns', got '%s'", subject.Namespace)
	}
}

func TestCreateTestRoleBinding_CorrectRole(t *testing.T) {
	ctx := context.Background()

	// Create namespace
	ns, err := testingutil.CreateTestNamespace(ctx, testk8sClient, "test-roleref-ns")
	if err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}
	defer testk8sClient.Delete(ctx, ns)

	// Create ServiceAccount
	sa, err := testingutil.CreateTestServiceAccount(ctx, testk8sClient, "test-roleref-ns", "deployer")
	if err != nil {
		t.Fatalf("failed to create ServiceAccount: %v", err)
	}
	defer testk8sClient.Delete(ctx, sa)

	// Create Role
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments"},
			Verbs:     []string{"*"},
		},
	}
	role, err := testingutil.CreateTestRole(ctx, testk8sClient, "test-roleref-ns", "my-role", rules)
	if err != nil {
		t.Fatalf("failed to create Role: %v", err)
	}
	defer testk8sClient.Delete(ctx, role)

	// Create RoleBinding
	rb, err := testingutil.CreateTestRoleBinding(ctx, testk8sClient, "test-roleref-ns", "my-role", "deployer")
	if err != nil {
		t.Fatalf("failed to create RoleBinding: %v", err)
	}
	defer testk8sClient.Delete(ctx, rb)

	// Verify RoleRef
	if rb.RoleRef.Kind != "Role" {
		t.Errorf("expected RoleRef Kind 'Role', got '%s'", rb.RoleRef.Kind)
	}
	if rb.RoleRef.Name != "my-role" {
		t.Errorf("expected RoleRef Name 'my-role', got '%s'", rb.RoleRef.Name)
	}
	if rb.RoleRef.APIGroup != rbacv1.GroupName {
		t.Errorf("expected RoleRef APIGroup '%s', got '%s'", rbacv1.GroupName, rb.RoleRef.APIGroup)
	}
}

func TestCreateNamespaceWithRBAC_CreatesAllResources(t *testing.T) {
	ctx := context.Background()

	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments"},
			Verbs:     []string{"get", "list", "watch", "create", "update"},
		},
	}

	ns, sa, err := testingutil.CreateNamespaceWithRBAC(ctx, testk8sClient, "test-rbac-ns", "deployer", rules)
	if err != nil {
		t.Fatalf("failed to create namespace with RBAC: %v", err)
	}
	defer testk8sClient.Delete(ctx, ns)
	defer testk8sClient.Delete(ctx, sa)

	if ns == nil {
		t.Fatal("expected namespace, got nil")
	}
	if sa == nil {
		t.Fatal("expected ServiceAccount, got nil")
	}

	// Verify namespace was created
	retrievedNs := &corev1.Namespace{}
	if err := testk8sClient.Get(ctx, client.ObjectKey{Name: "test-rbac-ns"}, retrievedNs); err != nil {
		t.Errorf("namespace not found: %v", err)
	}

	// Verify ServiceAccount was created
	retrievedSa := &corev1.ServiceAccount{}
	if err := testk8sClient.Get(ctx, client.ObjectKey{Name: "deployer", Namespace: "test-rbac-ns"}, retrievedSa); err != nil {
		t.Errorf("ServiceAccount not found: %v", err)
	}

	// Verify Role was created
	role := &rbacv1.Role{}
	if err := testk8sClient.Get(ctx, client.ObjectKey{Name: "deployer-role", Namespace: "test-rbac-ns"}, role); err != nil {
		t.Errorf("Role not found: %v", err)
	}

	// Verify RoleBinding was created
	rb := &rbacv1.RoleBinding{}
	if err := testk8sClient.Get(ctx, client.ObjectKey{Name: sa.Name + "-" + role.Name, Namespace: "test-rbac-ns"}, rb); err != nil {
		t.Errorf("RoleBinding not found: %v", err)
	}

	// Cleanup
	testk8sClient.Delete(ctx, role)
	testk8sClient.Delete(ctx, rb)
}

func TestCreateNamespaceWithRBAC_ResourcesLinked(t *testing.T) {
	ctx := context.Background()

	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"batch"},
			Resources: []string{"jobs"},
			Verbs:     []string{"create", "get", "list"},
		},
	}

	ns, sa, err := testingutil.CreateNamespaceWithRBAC(ctx, testk8sClient, "test-linked-ns", "job-runner", rules)
	if err != nil {
		t.Fatalf("failed to create namespace with RBAC: %v", err)
	}
	defer testk8sClient.Delete(ctx, ns)
	defer testk8sClient.Delete(ctx, sa)

	// Verify the resources are properly linked
	role := &rbacv1.Role{}
	if err := testk8sClient.Get(ctx, client.ObjectKey{Name: "job-runner-role", Namespace: "test-linked-ns"}, role); err != nil {
		t.Fatalf("failed to get role: %v", err)
	}

	rb := &rbacv1.RoleBinding{}
	if err := testk8sClient.Get(ctx, client.ObjectKey{Name: sa.Name + "-" + role.Name, Namespace: "test-linked-ns"}, rb); err != nil {
		t.Fatalf("failed to get rolebinding: %v", err)
	}

	// Verify RoleBinding points to correct Role
	if rb.RoleRef.Name != role.Name {
		t.Errorf("RoleBinding points to wrong role: expected %s, got %s", role.Name, rb.RoleRef.Name)
	}

	// Verify RoleBinding grants permissions to correct SA
	if len(rb.Subjects) != 1 || rb.Subjects[0].Name != sa.Name {
		t.Errorf("RoleBinding grants permissions to wrong SA")
	}

	// Cleanup
	testk8sClient.Delete(ctx, role)
	testk8sClient.Delete(ctx, rb)
}

func TestCreateNamespaceWithDeployPermissions_IncludesPermissions(t *testing.T) {
	ctx := context.Background()

	ns, sa, err := testingutil.CreateNamespaceWithDeployPermissions(ctx, testk8sClient, "test-deploy-ns", "werf-deployer")
	if err != nil {
		t.Fatalf("failed to create namespace with deploy permissions: %v", err)
	}
	defer testk8sClient.Delete(ctx, ns)
	defer testk8sClient.Delete(ctx, sa)

	// Verify Role includes deployment permissions
	role := &rbacv1.Role{}
	if err := testk8sClient.Get(ctx, client.ObjectKey{Name: "werf-deployer-role", Namespace: "test-deploy-ns"}, role); err != nil {
		t.Fatalf("failed to get role: %v", err)
	}

	// Check that we have rules for apps group
	foundAppsRule := false
	for _, rule := range role.Rules {
		if len(rule.APIGroups) > 0 && rule.APIGroups[0] == "apps" {
			foundAppsRule = true
			break
		}
	}

	if !foundAppsRule {
		t.Error("expected to find apps permissions in role")
	}

	// Cleanup
	testk8sClient.Delete(ctx, role)
	rb := &rbacv1.RoleBinding{}
	if err := testk8sClient.Get(ctx, client.ObjectKey{Name: sa.Name + "-" + role.Name, Namespace: "test-deploy-ns"}, rb); err == nil {
		testk8sClient.Delete(ctx, rb)
	}
}
