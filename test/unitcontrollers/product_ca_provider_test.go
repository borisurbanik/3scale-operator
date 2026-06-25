package test

import (
	"context"
	"testing"

	capabilitiesv1beta1 "github.com/3scale/3scale-operator/apis/capabilities/v1beta1"
	capabilitiescontrollers "github.com/3scale/3scale-operator/controllers/capabilities"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// TestProductReconciler_Reconcile is a table-driven test suite that verifies
// ProductReconciler behaviour under an invalid CA bundle.
func TestProductReconciler_Reconcile(t *testing.T) {
	tests := []struct {
		name           string
		objects        []runtime.Object
		conditionCheck func(err error, cl client.Client) bool
	}{
		{
			// ProductReconciler.reconcile() calls CAProvider after the finalizer
			// guard. The CR is pre-seeded with the finalizer, a "hits" metric, and
			// the required "apicast" policy so that SetDefaults() returns false and
			// no early requeue occurs.
			name: "InvalidCABundle",
			objects: []runtime.Object{
				func() *capabilitiesv1beta1.Product {
					p := &capabilitiesv1beta1.Product{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-product",
							Namespace: caTestNamespace,
						},
						Spec: capabilitiesv1beta1.ProductSpec{
							Name:       "test",
							SystemName: "test",
							Metrics: map[string]capabilitiesv1beta1.MetricSpec{
								"hits": {Name: "Hits", Unit: "hit", Description: "Number of API hits"},
							},
							Policies: []capabilitiesv1beta1.PolicyConfig{
								{
									Name:          "apicast",
									Version:       "builtin",
									Configuration: k8sruntime.RawExtension{Raw: []byte(`{}`)},
									Enabled:       true,
								},
							},
						},
					}
					controllerutil.AddFinalizer(p, "product.capabilities.3scale.net/finalizer")
					return p
				}(),
				providerAccountSecret(),
				caBundle(),
			},
			conditionCheck: func(_ error, cl client.Client) bool {
				cr := &capabilitiesv1beta1.Product{}
				if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-product"}, cr); err != nil {
					t.Fatalf("get Product: %v", err)
				}
				cond := cr.Status.Conditions.GetCondition(capabilitiesv1beta1.ProductFailedConditionType)
				return cond != nil && cond.Status == corev1.ConditionTrue
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base, caProvider := setupCATestReconciler(t, tc.objects...)
			r := &capabilitiescontrollers.ProductReconciler{BaseReconciler: base, CAProvider: caProvider}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-product"))

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
