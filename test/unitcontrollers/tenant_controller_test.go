package test

import (
	"context"
	"testing"

	capabilitiesv1alpha1 "github.com/3scale/3scale-operator/apis/capabilities/v1alpha1"
	capabilitiescontrollers "github.com/3scale/3scale-operator/controllers/capabilities"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// TestTenantReconciler_Reconcile is a table-driven test suite that verifies
// TenantReconciler behaviour under an invalid CA bundle.
func TestTenantReconciler_Reconcile(t *testing.T) {
	tests := []struct {
		name           string
		objects        []runtime.Object
		conditionCheck func(err error, cl client.Client) bool
	}{
		{
			// TenantReconciler calls setupPortaClient (which calls PortaClientFromURL)
			// as the very first action after fetching the CR — no guards to bypass.
			// The CA error is absorbed by reconcileStatus() and returned as
			// {Requeue: true}, nil, so the error is checked via the status
			// condition rather than the return value.
			//
			// The CR is pre-seeded with the tenant finalizer so that
			// reconcileMetadata() returns false, and the reconciler proceeds to
			// internalReconciler.Run() where the first 3scale API call hits the
			// failing transport.
			name: "InvalidCABundle",
			objects: []runtime.Object{
				func() *capabilitiesv1alpha1.Tenant {
					ten := &capabilitiesv1alpha1.Tenant{
						ObjectMeta: metav1.ObjectMeta{Name: "test-tenant", Namespace: caTestNamespace},
						Spec: capabilitiesv1alpha1.TenantSpec{
							Username:         "admin",
							Email:            "admin@example.com",
							OrganizationName: "testorg",
							SystemMasterUrl:  "https://master.example.com",
							MasterCredentialsRef: corev1.SecretReference{
								Name:      "master-credentials",
								Namespace: caTestNamespace,
							},
							TenantSecretRef: corev1.SecretReference{
								Name:      "tenant-secret",
								Namespace: caTestNamespace,
							},
							PasswordCredentialsRef: corev1.SecretReference{
								Name:      "password-credentials",
								Namespace: caTestNamespace,
							},
						},
					}
					controllerutil.AddFinalizer(ten, "tenant.capabilities.3scale.net/finalizer")
					return ten
				}(),
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "master-credentials", Namespace: caTestNamespace},
					Data:       map[string][]byte{"MASTER_ACCESS_TOKEN": []byte("test-token")},
					Type:       corev1.SecretTypeOpaque,
				},
			},
			conditionCheck: func(_ error, cl client.Client) bool {
				cr := &capabilitiesv1alpha1.Tenant{}
				if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-tenant"}, cr); err != nil {
					t.Fatalf("get Tenant: %v", err)
				}
				cond := cr.Status.Conditions.GetCondition(capabilitiesv1alpha1.TenantReadyConditionType)
				return cond != nil && cond.Status == corev1.ConditionFalse && cond.Message != ""
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := setupCATestReconciler(t, tc.objects...)
			setupCAWithFailingTLS(t)
			r := &capabilitiescontrollers.TenantReconciler{BaseReconciler: base}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-tenant"))

			if tc.conditionCheck != nil {
				if !tc.conditionCheck(err, base.Client()) {
					t.Error("expected CA validation error to be visible via status condition but it was not")
				}
				return
			}

			if err == nil {
				t.Fatal("CA error was not surfaced via return value and no conditionCheck is configured")
			}
		})
	}
}
