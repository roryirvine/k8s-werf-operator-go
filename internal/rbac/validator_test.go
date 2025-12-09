// Package rbac provides RBAC validation utilities for the Werf operator.
package rbac

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestValidateServiceAccountExists(t *testing.T) {
	tests := []struct {
		name          string
		saName        string
		saNamespace   string
		existingSA    *corev1.ServiceAccount
		wantErr       bool
		wantErrSubstr string
	}{
		{
			name:        "ServiceAccount exists",
			saName:      "werf-deploy",
			saNamespace: "target-ns",
			existingSA: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "werf-deploy",
					Namespace: "target-ns",
				},
			},
			wantErr: false,
		},
		{
			name:          "ServiceAccount not found",
			saName:        "missing-sa",
			saNamespace:   "target-ns",
			existingSA:    nil,
			wantErr:       true,
			wantErrSubstr: "ServiceAccount 'missing-sa' not found in namespace 'target-ns'",
		},
		{
			name:        "ServiceAccount in different namespace",
			saName:      "werf-deploy",
			saNamespace: "other-ns",
			existingSA: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "werf-deploy",
					Namespace: "target-ns",
				},
			},
			wantErr:       true,
			wantErrSubstr: "ServiceAccount 'werf-deploy' not found in namespace 'other-ns'",
		},
		{
			name:          "Error message includes SA name",
			saName:        "test-sa",
			saNamespace:   "test-namespace",
			existingSA:    nil,
			wantErr:       true,
			wantErrSubstr: "test-sa",
		},
		{
			name:          "Error message includes namespace",
			saName:        "test-sa",
			saNamespace:   "test-namespace",
			existingSA:    nil,
			wantErr:       true,
			wantErrSubstr: "test-namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build fake client with or without the ServiceAccount
			var objs []client.Object
			if tt.existingSA != nil {
				objs = append(objs, tt.existingSA)
			}
			fakeClient := fake.NewClientBuilder().
				WithObjects(objs...).
				Build()

			// Call validator
			err := ValidateServiceAccountExists(
				context.Background(),
				fakeClient,
				tt.saName,
				tt.saNamespace,
			)

			// Check result
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Errorf("error message %q does not contain %q", err.Error(), tt.wantErrSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
