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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const appFinalizer = "application.capabilities.3scale.net/finalizer"

// TestApplicationReconciler_Reconcile is a table-driven test suite that verifies
// ApplicationReconciler behaviour under an invalid CA bundle.
func TestApplicationReconciler_Reconcile(t *testing.T) {
	tests := []struct {
		name           string
		objects        []runtime.Object
		conditionCheck func(err error, cl client.Client) bool
	}{
		{
			// ApplicationReconciler calls CAProvider after the metadata guard
			// (finalizer + ownerRef). The CR is pre-seeded with the finalizer and
			// the DeveloperAccount ownerRef so that reconcileMetadata() returns
			// false and the first Reconcile call proceeds directly to
			// LookupProviderAccount → CAProvider.
			name: "InvalidCABundle",
			objects: []runtime.Object{
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
					controllerutil.AddFinalizer(a, appFinalizer)
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
			},
			conditionCheck: func(_ error, cl client.Client) bool {
				cr := &capabilitiesv1beta1.Application{}
				if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-app"}, cr); err != nil {
					t.Fatalf("get Application: %v", err)
				}
				cond := cr.Status.Conditions.GetCondition(capabilitiesv1beta1.ApplicationReadyConditionType)
				return cond != nil && cond.Status == corev1.ConditionFalse && cond.Message != ""
			},
		},
		{
			// During deletion, ApplicationReconciler must return an error and
			// leave the finalizer in place when the CA bundle is invalid.
			name: "DeletionPath_InvalidCABundle",
			objects: []runtime.Object{
				func() *capabilitiesv1beta1.Application {
					now := metav1.Now()
					return &capabilitiesv1beta1.Application{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "test-app",
							Namespace:         caTestNamespace,
							Finalizers:        []string{appFinalizer},
							DeletionTimestamp: &now,
						},
						Spec: capabilitiesv1beta1.ApplicationSpec{
							AccountCR: &corev1.LocalObjectReference{Name: "test-account"},
						},
						Status: capabilitiesv1beta1.ApplicationStatus{
							ID: ptr.To(int64(1)),
						},
					}
				}(),
				&capabilitiesv1beta1.DeveloperAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-account",
						Namespace: caTestNamespace,
					},
					Status: capabilitiesv1beta1.DeveloperAccountStatus{
						ID: ptr.To(int64(1)),
					},
				},
				providerAccountSecret(),
				caBundle(),
			},
			conditionCheck: func(err error, cl client.Client) bool {
				if err == nil {
					t.Error("expected reconcile to return an error for invalid CA bundle during deletion")
					return false
				}
				cr := &capabilitiesv1beta1.Application{}
				if getErr := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-app"}, cr); getErr != nil {
					t.Fatalf("get Application: %v", getErr)
				}
				return controllerutil.ContainsFinalizer(cr, appFinalizer)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base, caProvider := setupCATestReconciler(t, tc.objects...)
			r := &capabilitiescontrollers.ApplicationReconciler{BaseReconciler: base, CAProvider: caProvider}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-app"))

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
