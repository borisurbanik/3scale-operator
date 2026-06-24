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
)

// TestApplicationAuthReconciler_InvalidCABundle verifies that
// ApplicationAuthReconciler surfaces a CA validation error as a returned error
// without attempting the 3scale API call.
//
// ApplicationAuthReconciler has no finalizer guard. All dependencies (Application,
// DeveloperAccount, Product, provider-account secret) are pre-seeded so the
// reconciler reaches CAProvider on the first call.
func TestApplicationAuthReconciler_InvalidCABundle(t *testing.T) {
	objects := []runtime.Object{
		&capabilitiesv1beta1.ApplicationAuth{
			ObjectMeta: metav1.ObjectMeta{Name: "test-appauth", Namespace: caTestNamespace},
			Spec: capabilitiesv1beta1.ApplicationAuthSpec{
				ApplicationCRName: "test-app",
				AuthSecretRef:     &corev1.LocalObjectReference{Name: "auth-secret"},
			},
		},
		&capabilitiesv1beta1.Application{
			ObjectMeta: metav1.ObjectMeta{Name: "test-app", Namespace: caTestNamespace},
			Spec: capabilitiesv1beta1.ApplicationSpec{
				AccountCR:           &corev1.LocalObjectReference{Name: "test-account"},
				ProductCR:           &corev1.LocalObjectReference{Name: "test-product"},
				ApplicationPlanName: "basic",
				Name:                "test",
				Description:         "test",
			},
		},
		&capabilitiesv1beta1.DeveloperAccount{
			ObjectMeta: metav1.ObjectMeta{Name: "test-account", Namespace: caTestNamespace},
			Spec:       capabilitiesv1beta1.DeveloperAccountSpec{OrgName: "testorg"},
		},
		&capabilitiesv1beta1.Product{
			ObjectMeta: metav1.ObjectMeta{Name: "test-product", Namespace: caTestNamespace},
			Spec:       capabilitiesv1beta1.ProductSpec{Name: "test", SystemName: "test"},
		},
		providerAccountSecret(),
		caBundle(),
	}

	// ApplicationAuthReconciler returns the CA error directly; no conditionCheck needed.
	runCAProviderTest(t, objects,
		func(base *reconcilers.BaseReconciler, ca *controllerhelper.CAProvider) bool {
			r := &capabilitiescontrollers.ApplicationAuthReconciler{BaseReconciler: base, CAProvider: ca}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-appauth"))
			return err != nil
		},
		nil,
	)
}
