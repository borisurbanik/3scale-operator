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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// TestBackendReconciler_InvalidCABundle verifies that BackendReconciler surfaces
// a CA validation error as a BackendFailedConditionType=True status condition
// without attempting the 3scale API call.
//
// BackendReconciler.reconcile() calls CAProvider after the finalizer guard.
// The CR is pre-seeded with the finalizer and a complete Metrics map (including
// "hits") so that SetDefaults() returns false and no early requeue occurs.
func TestBackendReconciler_InvalidCABundle(t *testing.T) {
	objects := []runtime.Object{
		func() *capabilitiesv1beta1.Backend {
			b := &capabilitiesv1beta1.Backend{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-backend",
					Namespace: caTestNamespace,
				},
				Spec: capabilitiesv1beta1.BackendSpec{
					Name:           "test",
					SystemName:     "test",
					PrivateBaseURL: "https://backend.example.com",
					Metrics: map[string]capabilitiesv1beta1.MetricSpec{
						"hits": {Name: "Hits", Unit: "hit", Description: "Number of API hits"},
					},
				},
			}
			controllerutil.AddFinalizer(b, "backend.capabilities.3scale.net/finalizer")
			return b
		}(),
		providerAccountSecret(),
		caBundle(),
	}

	runCAProviderTest(t, objects,
		func(base *reconcilers.BaseReconciler, ca *controllerhelper.CAProvider) bool {
			r := &capabilitiescontrollers.BackendReconciler{BaseReconciler: base, CAProvider: ca}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-backend"))
			return err != nil
		},
		func(cl client.Client) bool {
			cr := &capabilitiesv1beta1.Backend{}
			if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-backend"}, cr); err != nil {
				t.Fatalf("get Backend: %v", err)
			}
			cond := cr.Status.Conditions.GetCondition(capabilitiesv1beta1.BackendFailedConditionType)
			return cond != nil && cond.Status == corev1.ConditionTrue
		},
	)
}
