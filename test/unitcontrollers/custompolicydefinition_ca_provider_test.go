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

// TestCustomPolicyDefinitionReconciler_InvalidCABundle verifies that
// CustomPolicyDefinitionReconciler surfaces a CA validation error as a
// CustomPolicyDefinitionFailedConditionType=True status condition without
// attempting the 3scale API call.
//
// CustomPolicyDefinitionReconciler.reconcileSpec() calls CAProvider directly
// with no finalizer guard.
func TestCustomPolicyDefinitionReconciler_InvalidCABundle(t *testing.T) {
	objects := []runtime.Object{
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
		caBundle(),
	}

	runCAProviderTest(t, objects,
		func(base *reconcilers.BaseReconciler, ca *controllerhelper.CAProvider) bool {
			r := &capabilitiescontrollers.CustomPolicyDefinitionReconciler{BaseReconciler: base, CAProvider: ca}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-cpd"))
			return err != nil
		},
		func(cl client.Client) bool {
			cr := &capabilitiesv1beta1.CustomPolicyDefinition{}
			if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-cpd"}, cr); err != nil {
				t.Fatalf("get CustomPolicyDefinition: %v", err)
			}
			cond := cr.Status.Conditions.GetCondition(capabilitiesv1beta1.CustomPolicyDefinitionFailedConditionType)
			return cond != nil && cond.Status == corev1.ConditionTrue
		},
	)
}
