package test

import (
	"context"
	"testing"

	capabilitiesv1beta1 "github.com/3scale/3scale-operator/apis/capabilities/v1beta1"
	capabilitiescontrollers "github.com/3scale/3scale-operator/controllers/capabilities"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestApplicationAuthReconciler_Reconcile is a table-driven test suite that
// verifies ApplicationAuthReconciler behaviour under an invalid CA bundle.
func TestApplicationAuthReconciler_Reconcile(t *testing.T) {
	tests := []struct {
		name           string
		objects        []runtime.Object
		conditionCheck func(err error, cl client.Client) bool
	}{
		{
			// ApplicationAuthReconciler has no finalizer guard. All dependencies
			// (Application, DeveloperAccount, Product, provider-account secret) are
			// pre-seeded so the reconciler reaches CAProvider on the first call.
			name: "InvalidCABundle",
			objects: []runtime.Object{
				&capabilitiesv1beta1.ApplicationAuth{
					ObjectMeta: metav1.ObjectMeta{Name: "test-appauth", Namespace: caTestNamespace},
					Spec: capabilitiesv1beta1.ApplicationAuthSpec{
						ApplicationCRName: "test-app",
						AuthSecretRef:     &corev1.LocalObjectReference{Name: "auth-secret"},
					},
				},
				&capabilitiesv1beta1.Application{
					ObjectMeta: metav1.ObjectMeta{Name: "test-app", Namespace: caTestNamespace},
					Spec: capabilitiesv1beta1.ApplicationSpec{
						AccountCR:           &corev1.LocalObjectReference{Name: "test-account"},
						ProductCR:           &corev1.LocalObjectReference{Name: "test-product"},
						ApplicationPlanName: "basic",
						Name:                "test",
						Description:         "test",
					},
				},
				&capabilitiesv1beta1.DeveloperAccount{
					ObjectMeta: metav1.ObjectMeta{Name: "test-account", Namespace: caTestNamespace},
					Spec:       capabilitiesv1beta1.DeveloperAccountSpec{OrgName: "testorg"},
				},
				&capabilitiesv1beta1.Product{
					ObjectMeta: metav1.ObjectMeta{Name: "test-product", Namespace: caTestNamespace},
					Spec:       capabilitiesv1beta1.ProductSpec{Name: "test", SystemName: "test"},
				},
				providerAccountSecret(),
				caBundle(),
			},
			conditionCheck: func(_ error, cl client.Client) bool {
				cr := &capabilitiesv1beta1.ApplicationAuth{}
				if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-appauth"}, cr); err != nil {
					t.Fatalf("get ApplicationAuth: %v", err)
				}
				cond := cr.Status.Conditions.GetCondition(capabilitiesv1beta1.ApplicationAuthFailedConditionType)
				return cond != nil && cond.Status == corev1.ConditionTrue && cond.Message != ""
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base, caProvider := setupCATestReconciler(t, tc.objects...)
			r := &capabilitiescontrollers.ApplicationAuthReconciler{BaseReconciler: base, CAProvider: caProvider}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-appauth"))

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
