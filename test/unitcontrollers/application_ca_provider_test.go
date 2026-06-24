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

// TestApplicationReconciler_InvalidCABundle verifies that ApplicationReconciler
// surfaces a CA validation error as an ApplicationReadyConditionType=False status
// condition without attempting the 3scale API call.
//
// ApplicationReconciler calls CAProvider after the metadata guard (finalizer +
// ownerRef). The CR is pre-seeded with the finalizer and the DeveloperAccount
// ownerRef so that reconcileMetadata() returns false and the first Reconcile call
// proceeds directly to LookupProviderAccount → CAProvider.
func TestApplicationReconciler_InvalidCABundle(t *testing.T) {
	objects := []runtime.Object{
		func() *capabilitiesv1beta1.Application {
			a := &capabilitiesv1beta1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: caTestNamespace,
				},
				Spec: capabilitiesv1beta1.ApplicationSpec{
					AccountCR:           &corev1.LocalObjectReference{Name: "test-account"},
					ProductCR:           &corev1.LocalObjectReference{Name: "test-product"},
					ApplicationPlanName: "basic",
					Name:                "test",
					Description:         "test",
				},
			}
			controllerutil.AddFinalizer(a, "application.capabilities.3scale.net/finalizer")
			// Seed the ownerRef so reconcileMetadata returns false.
			a.OwnerReferences = []metav1.OwnerReference{
				{
					APIVersion: capabilitiesv1beta1.GroupVersion.String(),
					Kind:       "DeveloperAccount",
					Name:       "test-account",
					UID:        "test-account-uid",
				},
			}
			return a
		}(),
		&capabilitiesv1beta1.DeveloperAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-account",
				Namespace: caTestNamespace,
				UID:       "test-account-uid",
			},
			Spec: capabilitiesv1beta1.DeveloperAccountSpec{OrgName: "testorg"},
		},
		&capabilitiesv1beta1.Product{
			ObjectMeta: metav1.ObjectMeta{Name: "test-product", Namespace: caTestNamespace},
			Spec:       capabilitiesv1beta1.ProductSpec{Name: "test", SystemName: "test"},
		},
		providerAccountSecret(),
		caBundle(),
	}

	runCAProviderTest(t, objects,
		func(base *reconcilers.BaseReconciler, ca *controllerhelper.CAProvider) bool {
			r := &capabilitiescontrollers.ApplicationReconciler{BaseReconciler: base, CAProvider: ca}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-app"))
			return err != nil
		},
		func(cl client.Client) bool {
			cr := &capabilitiesv1beta1.Application{}
			if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-app"}, cr); err != nil {
				t.Fatalf("get Application: %v", err)
			}
			cond := cr.Status.Conditions.GetCondition(capabilitiesv1beta1.ApplicationReadyConditionType)
			return cond != nil && cond.Status == corev1.ConditionFalse && cond.Message != ""
		},
	)
}
