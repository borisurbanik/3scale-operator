package test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	capabilitiesv1alpha1 "github.com/3scale/3scale-operator/apis/capabilities/v1alpha1"
	capabilitiesv1beta1 "github.com/3scale/3scale-operator/apis/capabilities/v1beta1"
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

// failingTransport is an http.RoundTripper that always returns a TLS-like error,
// simulating the effect of a misconfigured or invalid CA bundle.
type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("tls: failed to verify certificate: x509: certificate signed by unknown authority")
}

// workingHTTPClientSource returns a real *http.Client with system roots.
// Use for tests that don't exercise CA errors.
type workingHTTPClientSource struct{}

func (workingHTTPClientSource) GetHTTPClient() *http.Client {
	return &http.Client{}
}

// failingHTTPClientSource returns a *http.Client whose transport always fails
// with a TLS-like error.  Use for CA error surfacing tests.
type failingHTTPClientSource struct{}

func (failingHTTPClientSource) GetHTTPClient() *http.Client {
	return &http.Client{Transport: failingTransport{}}
}

// Verify both types satisfy the interface at compile time.
var _ reconcilers.HTTPClientSource = workingHTTPClientSource{}
var _ reconcilers.HTTPClientSource = failingHTTPClientSource{}

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

// setupCATestReconciler builds a BaseReconciler backed by a fake client
// pre-seeded with the supplied objects, and a failingHTTPClientSource test
// double that simulates a TLS error on every outbound request.
func setupCATestReconciler(t *testing.T, objects ...runtime.Object) (*reconcilers.BaseReconciler, reconcilers.HTTPClientSource) {
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

	return base, failingHTTPClientSource{}
}

func reqFor(ns, name string) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}}
}
