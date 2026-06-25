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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// TestDeveloperAccountReconciler_Reconcile is a table-driven test suite that
// verifies DeveloperAccountReconciler behaviour under an invalid CA bundle.
func TestDeveloperAccountReconciler_Reconcile(t *testing.T) {
	tests := []struct {
		name           string
		objects        []runtime.Object
		conditionCheck func(err error, cl client.Client) bool
	}{
		{
			// DeveloperAccountReconciler.reconcileSpec() calls CAProvider after the
			// metadata guard (finalizer). The CR is pre-seeded with the finalizer
			// already set.
			name: "InvalidCABundle",
			objects: []runtime.Object{
				func() *capabilitiesv1beta1.DeveloperAccount {
					a := &capabilitiesv1beta1.DeveloperAccount{
						ObjectMeta: metav1.ObjectMeta{Name: "test-account", Namespace: caTestNamespace},
						Spec:       capabilitiesv1beta1.DeveloperAccountSpec{OrgName: "testorg"},
					}
					controllerutil.AddFinalizer(a, "developeraccount.capabilities.3scale.net/finalizer")
					return a
				}(),
				providerAccountSecret(),
				caBundle(),
			},
			conditionCheck: func(_ error, cl client.Client) bool {
				cr := &capabilitiesv1beta1.DeveloperAccount{}
				if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-account"}, cr); err != nil {
					t.Fatalf("get DeveloperAccount: %v", err)
				}
				cond := cr.Status.Conditions.GetCondition(capabilitiesv1beta1.DeveloperAccountFailedConditionType)
				return cond != nil && cond.Status == corev1.ConditionTrue
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base, caProvider := setupCATestReconciler(t, tc.objects...)
			r := &capabilitiescontrollers.DeveloperAccountReconciler{BaseReconciler: base, CAProvider: caProvider}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-account"))

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
