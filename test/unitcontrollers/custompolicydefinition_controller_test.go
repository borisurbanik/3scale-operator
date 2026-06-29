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

// TestCustomPolicyDefinitionReconciler_Reconcile is a table-driven test suite
// that verifies CustomPolicyDefinitionReconciler behaviour under an invalid CA bundle.
func TestCustomPolicyDefinitionReconciler_Reconcile(t *testing.T) {
	tests := []struct {
		name           string
		objects        []runtime.Object
		conditionCheck func(err error, cl client.Client) bool
	}{
		{
			// CustomPolicyDefinitionReconciler.reconcileSpec() calls PortaClientFromAccount
			// directly with no finalizer guard.
			name: "InvalidCABundle",
			objects: []runtime.Object{
				&capabilitiesv1beta1.CustomPolicyDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: "test-cpd", Namespace: caTestNamespace},
					Spec: capabilitiesv1beta1.CustomPolicyDefinitionSpec{
						Name:    "test",
						Version: "0.1.0",
						Schema: capabilitiesv1beta1.CustomPolicySchemaSpec{
							Name:    "test",
							Version: "0.1.0",
							Summary: "test",
							Schema:  "http://json-schema.org/draft-07/schema#",
						},
					},
				},
				providerAccountSecret(),
			},
			conditionCheck: func(_ error, cl client.Client) bool {
				cr := &capabilitiesv1beta1.CustomPolicyDefinition{}
				if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-cpd"}, cr); err != nil {
					t.Fatalf("get CustomPolicyDefinition: %v", err)
				}
				cond := cr.Status.Conditions.GetCondition(capabilitiesv1beta1.CustomPolicyDefinitionFailedConditionType)
				return cond != nil && cond.Status == corev1.ConditionTrue
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := setupCATestReconciler(t, tc.objects...)
			setupCAWithFailingTLS(t)
			r := &capabilitiescontrollers.CustomPolicyDefinitionReconciler{BaseReconciler: base}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-cpd"))

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
