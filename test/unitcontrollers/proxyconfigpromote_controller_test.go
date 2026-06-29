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

// TestProxyConfigPromoteReconciler_Reconcile is a table-driven test suite that
// verifies ProxyConfigPromoteReconciler behaviour under an invalid CA bundle.
func TestProxyConfigPromoteReconciler_Reconcile(t *testing.T) {
	tests := []struct {
		name           string
		objects        []runtime.Object
		conditionCheck func(err error, cl client.Client) bool
	}{
		{
			// ProxyConfigPromoteReconciler calls LookupProviderAccount and then PortaClientFromAccount after fetching the
			// Product and resolving the provider account. No finalizer guard. The
			// Product and provider-account secret are pre-seeded so the reconciler
			// reaches PortaClientFromAccount on the first call.
			name: "InvalidCABundle",
			objects: []runtime.Object{
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
			},
			conditionCheck: func(_ error, cl client.Client) bool {
				cr := &capabilitiesv1beta1.ProxyConfigPromote{}
				if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-pcp"}, cr); err != nil {
					t.Fatalf("get ProxyConfigPromote: %v", err)
				}
				cond := cr.Status.Conditions.GetCondition(capabilitiesv1beta1.ProxyPromoteConfigFailedConditionType)
				return cond != nil && cond.Status == corev1.ConditionTrue && cond.Message != ""
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := setupCATestReconciler(t, tc.objects...)
			setupCAWithFailingTLS(t)
			r := &capabilitiescontrollers.ProxyConfigPromoteReconciler{BaseReconciler: base}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-pcp"))

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
