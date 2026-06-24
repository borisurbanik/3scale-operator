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

// TestDeveloperAccountReconciler_InvalidCABundle verifies that
// DeveloperAccountReconciler surfaces a CA validation error as a
// DeveloperAccountFailedConditionType=True status condition without attempting
// the 3scale API call.
//
// DeveloperAccountReconciler.reconcileSpec() calls CAProvider after the metadata
// guard (finalizer). The CR is pre-seeded with the finalizer already set.
func TestDeveloperAccountReconciler_InvalidCABundle(t *testing.T) {
	objects := []runtime.Object{
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
	}

	runCAProviderTest(t, objects,
		func(base *reconcilers.BaseReconciler, ca *controllerhelper.CAProvider) bool {
			r := &capabilitiescontrollers.DeveloperAccountReconciler{BaseReconciler: base, CAProvider: ca}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-account"))
			return err != nil
		},
		func(cl client.Client) bool {
			cr := &capabilitiesv1beta1.DeveloperAccount{}
			if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-account"}, cr); err != nil {
				t.Fatalf("get DeveloperAccount: %v", err)
			}
			cond := cr.Status.Conditions.GetCondition(capabilitiesv1beta1.DeveloperAccountFailedConditionType)
			return cond != nil && cond.Status == corev1.ConditionTrue
		},
	)
}
