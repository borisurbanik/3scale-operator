package test

import (
	"context"
	"testing"

	capabilitiesv1alpha1 "github.com/3scale/3scale-operator/apis/capabilities/v1alpha1"
	capabilitiesv1beta1 "github.com/3scale/3scale-operator/apis/capabilities/v1beta1"
	controllerhelper "github.com/3scale/3scale-operator/pkg/controller/helper"
	"github.com/3scale/3scale-operator/pkg/reconcilers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakectrlclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const caTestNamespace = "operator-unittest"

// invalidCABundlePEM is a PEM block whose type is not "CERTIFICATE".
// CAProvider rejects it with a CAValidationError, which is the error
// this test suite is verifying is surfaced by each reconciler.
const invalidCABundlePEM = `-----BEGIN PRIVATE KEY-----
dGVzdA==
-----END PRIVATE KEY-----
`

// caBundle returns the threescale-ca-bundle ConfigMap with an invalid PEM value.
func caBundle() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerhelper.CABundleConfigMapName,
			Namespace: caTestNamespace,
		},
		Data: map[string]string{
			controllerhelper.CABundleConfigMapKey: invalidCABundlePEM,
		},
	}
}

// providerAccountSecret returns the well-known secret that LookupProviderAccount
// reads when no ProviderAccountRef is set on a CR.
func providerAccountSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "threescale-provider-account",
			Namespace: caTestNamespace,
		},
		Data: map[string][]byte{
			"adminURL": []byte("https://3scale-admin.example.com"),
			"token":    []byte("test-token"),
		},
		Type: corev1.SecretTypeOpaque,
	}
}

// setupCATestReconciler builds a BaseReconciler backed by a fake client pre-seeded
// with the supplied objects, and a CAProvider pointed at the same client.
func setupCATestReconciler(t *testing.T, objects ...runtime.Object) (*reconcilers.BaseReconciler, *controllerhelper.CAProvider) {
	t.Helper()
	s := scheme.Scheme
	if err := capabilitiesv1beta1.AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme capabilitiesv1beta1: %v", err)
	}
	if err := capabilitiesv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme capabilitiesv1alpha1: %v", err)
	}

	var clientObjs []client.Object
	for _, o := range objects {
		if co, ok := o.(client.Object); ok {
			clientObjs = append(clientObjs, co)
		}
	}

	cl := fakectrlclient.NewClientBuilder().
		WithScheme(s).
		WithRuntimeObjects(objects...).
		WithStatusSubresource(clientObjs...).
		Build()

	clientset := fakeclientset.NewSimpleClientset()
	recorder := record.NewFakeRecorder(100)
	base := reconcilers.NewBaseReconciler(
		context.Background(), cl, s, cl,
		ctrl.Log.WithName("ca-provider-test"),
		clientset.Discovery(), recorder,
	)
	caProvider := controllerhelper.NewCAProvider(cl, caTestNamespace)
	return base, caProvider
}

func reqFor(ns, name string) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}}
}
