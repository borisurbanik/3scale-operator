package test

import (
	"context"
	"testing"

	capabilitiesv1beta1 "github.com/3scale/3scale-operator/apis/capabilities/v1beta1"
	capabilitiescontrollers "github.com/3scale/3scale-operator/controllers/capabilities"
	"github.com/3scale/3scale-operator/pkg/apispkg/common"
	controllerhelper "github.com/3scale/3scale-operator/pkg/controller/helper"
	"github.com/3scale/3scale-operator/pkg/reconcilers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// TestDeveloperUserReconciler_InvalidCABundle verifies that DeveloperUserReconciler
// surfaces a CA validation error as a DeveloperUserFailedConditionType=True status
// condition without attempting the 3scale API call.
//
// DeveloperUserReconciler.reconcileSpec() calls CAProvider after:
//  1. the metadata guard (finalizer)
//  2. the ownerRef guard (EnsureOwnerReference)
//  3. findParentAccount — the parent DeveloperAccount must be IsReady()
//
// The DeveloperUser is pre-seeded with finalizer + ownerRef already present, and
// the DeveloperAccount is pre-seeded with DeveloperAccountReadyConditionType=True.
func TestDeveloperUserReconciler_InvalidCABundle(t *testing.T) {
	objects := []runtime.Object{
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
			a := &capabilitiesv1beta1.DeveloperAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-account",
					Namespace: caTestNamespace,
					UID:       "test-account-uid",
				},
				Spec: capabilitiesv1beta1.DeveloperAccountSpec{OrgName: "testorg"},
				Status: capabilitiesv1beta1.DeveloperAccountStatus{
					Conditions: common.Conditions{
						{
							Type:   capabilitiesv1beta1.DeveloperAccountReadyConditionType,
							Status: corev1.ConditionTrue,
						},
					},
				},
			}
			return a
		}(),
		providerAccountSecret(),
		caBundle(),
	}

	runCAProviderTest(t, objects,
		func(base *reconcilers.BaseReconciler, ca *controllerhelper.CAProvider) bool {
			r := &capabilitiescontrollers.DeveloperUserReconciler{BaseReconciler: base, CAProvider: ca}
			_, err := r.Reconcile(context.Background(), reqFor(caTestNamespace, "test-user"))
			return err != nil
		},
		func(cl client.Client) bool {
			cr := &capabilitiesv1beta1.DeveloperUser{}
			if err := cl.Get(context.Background(), types.NamespacedName{Namespace: caTestNamespace, Name: "test-user"}, cr); err != nil {
				t.Fatalf("get DeveloperUser: %v", err)
			}
			cond := cr.Status.Conditions.GetCondition(capabilitiesv1beta1.DeveloperUserFailedConditionType)
			return cond != nil && cond.Status == corev1.ConditionTrue
		},
	)
}
