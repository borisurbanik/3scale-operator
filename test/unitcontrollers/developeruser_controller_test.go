package test

import (
	"context"
	"testing"

	capabilitiesv1beta1 "github.com/3scale/3scale-operator/apis/capabilities/v1beta1"
	capabilitiescontrollers "github.com/3scale/3scale-operator/controllers/capabilities"
	"github.com/3scale/3scale-operator/pkg/apispkg/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// TestDeveloperUserReconciler_Reconcile is a table-driven test suite that
// verifies DeveloperUserReconciler behaviour under an invalid CA bundle.
func TestDeveloperUserReconciler_Reconcile(t *testing.T) {
	tests := []struct {
		name           string
		objects        []runtime.Object
		conditionCheck func(err error, cl client.Client) bool
	}{
		{
			// DeveloperUserReconciler.reconcileSpec() calls LookupProviderAccount and then PortaClientFromAccount after:
			//   1. the metadata guard (finalizer)
			//   2. the ownerRef guard (EnsureOwnerReference)
			//   3. findParentAccount — the parent DeveloperAccount must be IsReady()
			//
			// The DeveloperUser is pre-seeded with finalizer + ownerRef already
			// present, and the DeveloperAccount is pre-seeded with
			// DeveloperAccountReadyConditionType=True.
			name: "InvalidCABundle",
			objects: []runtime.Object{
				func() *capabilitiesv1beta1.DeveloperUser {
					u := &capabilitiesv1beta1.DeveloperUser{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-user",
							Namespace: caTestNamespace,
						},
						Spec: capabilitiesv1beta1.DeveloperUserSpec{
							Username: "testuser",
							Email:    "testuser@example.com",
							PasswordCredentialsRef: corev1.SecretReference{
								Name:      "user-password",
								Namespace: caTestNamespace,
							},
							DeveloperAccountRef: corev1.LocalObjectReference{Name: "test-account"},
						},
					}
					controllerutil.AddFinalizer(u, "developeruser.capabilities.3scale.net/finalizer")
					u.OwnerReferences = []metav1.OwnerReference{
						{
							APIVersion: capabilitiesv1beta1.GroupVersion.String(),
							Kind:       "DeveloperAccount",
							Name:       "test-account",
							UID:        "test-account-uid",
						},
					}
					return u
				}(),
				func() *capabilitiesv1beta1.DeveloperAccount {
					return &capabilitiesv1beta1.DeveloperAccount{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-account",
							Namespace: caTestNamespace,
							UID:       "test-account-uid",
						},
						Spec: capabilitiesv1beta1.DeveloperAccountSpec{OrgName: "testorg"},
						Status: capabilitiesv1beta1.DeveloperAccountStatus{
							// ID non-nil so findDevUserByUsernameAndEmail can dereference it
							// without a nil pointer panic; the HTTP call then hits the
							// failing transport — exercising the TLS error path.
							ID: ptr.To(int64(42)),
							Conditions: common.Conditions{
								{
									Type:   capabilitiesv1beta1.DeveloperAccountReadyConditionType,
									Status: corev1.ConditionTrue,
								},
							},
						},
					}
				}(),
				providerAccountSecret(),
			},
			conditionCheck: func(_ error, cl client.Client) bool {
				cr := &capabilitiesv1beta1.DeveloperUser{}
				if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-user"}, cr); err != nil {
					t.Fatalf("get DeveloperUser: %v", err)
				}
				cond := cr.Status.Conditions.GetCondition(capabilitiesv1beta1.DeveloperUserFailedConditionType)
				return cond != nil && cond.Status == corev1.ConditionTrue
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := setupCATestReconciler(t, tc.objects...)
			setupCAWithFailingTLS(t)
			r := &capabilitiescontrollers.DeveloperUserReconciler{BaseReconciler: base}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-user"))

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
