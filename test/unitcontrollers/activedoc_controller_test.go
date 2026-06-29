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

// TestActiveDocReconciler_Reconcile is a table-driven test suite that verifies
// ActiveDocReconciler behaviour under an invalid CA bundle.
func TestActiveDocReconciler_Reconcile(t *testing.T) {
	tests := []struct {
		name           string
		objects        []runtime.Object
		conditionCheck func(err error, cl client.Client) bool
	}{
		{
			// ActiveDocReconciler.reconcileSpec() calls LookupProviderAccount and then PortaClientFromAccount after
			// checkExternalRefs. SystemName must be pre-set so that SetDefaults()
			// returns false and no early requeue occurs.
			name: "InvalidCABundle",
			objects: []runtime.Object{
				func() *capabilitiesv1beta1.ActiveDoc {
					systemName := "test"
					return &capabilitiesv1beta1.ActiveDoc{
						ObjectMeta: metav1.ObjectMeta{Name: "test-activedoc", Namespace: caTestNamespace},
						Spec: capabilitiesv1beta1.ActiveDocSpec{
							Name:       "test",
							SystemName: &systemName,
						},
					}
				}(),
				providerAccountSecret(),
			},
			conditionCheck: func(_ error, cl client.Client) bool {
				cr := &capabilitiesv1beta1.ActiveDoc{}
				if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-activedoc"}, cr); err != nil {
					t.Fatalf("get ActiveDoc: %v", err)
				}
				cond := cr.Status.Conditions.GetCondition(capabilitiesv1beta1.ActiveDocFailedConditionType)
				return cond != nil && cond.Status == corev1.ConditionTrue
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := setupCATestReconciler(t, tc.objects...)
			setupCAWithFailingTLS(t)
			r := &capabilitiescontrollers.ActiveDocReconciler{BaseReconciler: base}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-activedoc"))

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
