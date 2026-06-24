package test

import (
	"context"
	"testing"

	capabilitiesv1beta1 "github.com/3scale/3scale-operator/apis/capabilities/v1beta1"
	capabilitiescontrollers "github.com/3scale/3scale-operator/controllers/capabilities"
	controllerhelper "github.com/3scale/3scale-operator/pkg/controller/helper"
	"github.com/3scale/3scale-operator/pkg/reconcilers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// TestProductReconciler_InvalidCABundle verifies that ProductReconciler surfaces
// a CA validation error as a ProductFailedConditionType=True status condition
// without attempting the 3scale API call.
//
// ProductReconciler.reconcile() calls CAProvider after the finalizer guard.
// The CR is pre-seeded with the finalizer, a "hits" metric, and the required
// "apicast" policy so that SetDefaults() returns false and no early requeue occurs.
func TestProductReconciler_InvalidCABundle(t *testing.T) {
	objects := []runtime.Object{
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
	}

	runCAProviderTest(t, objects,
		func(base *reconcilers.BaseReconciler, ca *controllerhelper.CAProvider) bool {
			r := &capabilitiescontrollers.ProductReconciler{BaseReconciler: base, CAProvider: ca}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-product"))
			return err != nil
		},
		func(cl client.Client) bool {
			cr := &capabilitiesv1beta1.Product{}
			if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-product"}, cr); err != nil {
				t.Fatalf("get Product: %v", err)
			}
			cond := cr.Status.Conditions.GetCondition(capabilitiesv1beta1.ProductFailedConditionType)
			return cond != nil && cond.Status == corev1.ConditionTrue
		},
	)
}
