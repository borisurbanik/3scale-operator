package test

import (
	"context"
	"testing"

	capabilitiesv1beta1 "github.com/3scale/3scale-operator/apis/capabilities/v1beta1"
	capabilitiescontrollers "github.com/3scale/3scale-operator/controllers/capabilities"
	controllerhelper "github.com/3scale/3scale-operator/pkg/controller/helper"
	"github.com/3scale/3scale-operator/pkg/reconcilers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TestProxyConfigPromoteReconciler_InvalidCABundle verifies that
// ProxyConfigPromoteReconciler surfaces a CA validation error as a returned
// error without attempting the 3scale API call.
//
// ProxyConfigPromoteReconciler calls CAProvider after fetching the Product and
// resolving the provider account. No finalizer guard. The Product and
// provider-account secret are pre-seeded so the reconciler reaches CAProvider
// on the first call.
func TestProxyConfigPromoteReconciler_InvalidCABundle(t *testing.T) {
	objects := []runtime.Object{
		&capabilitiesv1beta1.ProxyConfigPromote{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pcp", Namespace: caTestNamespace},
			Spec: capabilitiesv1beta1.ProxyConfigPromoteSpec{
				ProductCRName: "test-product",
			},
		},
		&capabilitiesv1beta1.Product{
			ObjectMeta: metav1.ObjectMeta{Name: "test-product", Namespace: caTestNamespace},
			Spec:       capabilitiesv1beta1.ProductSpec{Name: "test", SystemName: "test"},
		},
		providerAccountSecret(),
		caBundle(),
	}

	// ProxyConfigPromoteReconciler returns the CA error directly; no conditionCheck needed.
	runCAProviderTest(t, objects,
		func(base *reconcilers.BaseReconciler, ca *controllerhelper.CAProvider) bool {
			r := &capabilitiescontrollers.ProxyConfigPromoteReconciler{BaseReconciler: base, CAProvider: ca}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-pcp"))
			return err != nil
		},
		nil,
	)
}
