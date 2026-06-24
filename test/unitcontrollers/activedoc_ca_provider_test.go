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
)

// TestActiveDocReconciler_InvalidCABundle verifies that ActiveDocReconciler surfaces
// a CA validation error as an ActiveDocFailedConditionType=True status condition
// without attempting the 3scale API call.
//
// ActiveDocReconciler.reconcileSpec() calls CAProvider after checkExternalRefs.
// SystemName must be pre-set so that SetDefaults() returns false and no early
// requeue occurs.
func TestActiveDocReconciler_InvalidCABundle(t *testing.T) {
	systemName := "test"
	objects := []runtime.Object{
		&capabilitiesv1beta1.ActiveDoc{
			ObjectMeta: metav1.ObjectMeta{Name: "test-activedoc", Namespace: caTestNamespace},
			Spec: capabilitiesv1beta1.ActiveDocSpec{
				Name:       "test",
				SystemName: &systemName,
			},
		},
		providerAccountSecret(),
		caBundle(),
	}

	runCAProviderTest(t, objects,
		func(base *reconcilers.BaseReconciler, ca *controllerhelper.CAProvider) bool {
			r := &capabilitiescontrollers.ActiveDocReconciler{BaseReconciler: base, CAProvider: ca}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-activedoc"))
			return err != nil
		},
		func(cl client.Client) bool {
			cr := &capabilitiesv1beta1.ActiveDoc{}
			if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-activedoc"}, cr); err != nil {
				t.Fatalf("get ActiveDoc: %v", err)
			}
			cond := cr.Status.Conditions.GetCondition(capabilitiesv1beta1.ActiveDocFailedConditionType)
			return cond != nil && cond.Status == corev1.ConditionTrue
		},
	)
}
