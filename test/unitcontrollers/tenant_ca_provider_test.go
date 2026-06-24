package test

import (
	"context"
	"testing"

	capabilitiesv1alpha1 "github.com/3scale/3scale-operator/apis/capabilities/v1alpha1"
	capabilitiescontrollers "github.com/3scale/3scale-operator/controllers/capabilities"
	controllerhelper "github.com/3scale/3scale-operator/pkg/controller/helper"
	"github.com/3scale/3scale-operator/pkg/reconcilers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestTenantReconciler_InvalidCABundle verifies that TenantReconciler surfaces a
// CA validation error as a non-empty message on the TenantReadyConditionType=False
// status condition without attempting the 3scale API call.
//
// TenantReconciler calls setupPortaClient (which calls CAProvider) as the very
// first action after fetching the CR — no guards to bypass. The CA error is
// absorbed by reconcileStatus() and returned as {Requeue: true}, nil, so the
// error is checked via the status condition rather than the return value.
func TestTenantReconciler_InvalidCABundle(t *testing.T) {
	objects := []runtime.Object{
		&capabilitiesv1alpha1.Tenant{
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
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "master-credentials", Namespace: caTestNamespace},
			Data:       map[string][]byte{"MASTER_ACCESS_TOKEN": []byte("test-token")},
			Type:       corev1.SecretTypeOpaque,
		},
		caBundle(),
	}

	// TenantReconciler absorbs the CA error via reconcileStatus, so the error is
	// surfaced as a non-empty message on the Ready=False condition.
	runCAProviderTest(t, objects,
		func(base *reconcilers.BaseReconciler, ca *controllerhelper.CAProvider) bool {
			r := &capabilitiescontrollers.TenantReconciler{BaseReconciler: base, CAProvider: ca}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-tenant"))
			return err != nil
		},
		func(cl client.Client) bool {
			cr := &capabilitiesv1alpha1.Tenant{}
			if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-tenant"}, cr); err != nil {
				t.Fatalf("get Tenant: %v", err)
			}
			cond := cr.Status.Conditions.GetCondition(capabilitiesv1alpha1.TenantReadyConditionType)
			return cond != nil && cond.Status == corev1.ConditionFalse && cond.Message != ""
		},
	)
}
