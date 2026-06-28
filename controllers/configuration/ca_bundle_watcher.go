/*
Copyright 2026 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package configuration

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"sync/atomic"

	pkghelper "github.com/3scale/3scale-operator/pkg/helper"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	CABundleConfigMapName = "threescale-ca-bundle"
	CABundleConfigMapKey  = "ca-bundle.crt"
)

// CAValidationError is returned when the CA bundle ConfigMap exists but its
// contents cannot be used to build a valid certificate pool.
type CAValidationError struct {
	Reason  string
	Message string
	Err     error
}

func (e *CAValidationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Reason, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Reason, e.Message)
}

func (e *CAValidationError) Unwrap() error {
	return e.Err
}

const (
	CAValidationReasonMissingSecret = "MissingCASecret"
	CAValidationReasonMissingKey    = "MissingCAKey"
	CAValidationReasonInvalidFormat = "InvalidCAFormat"
)

// CABundleWatcher watches the threescale-ca-bundle ConfigMap and maintains a
// single cached *http.Client that reflects the current CA bundle.
// It implements reconcilers.HTTPClientSource (structural; no explicit import
// of pkg/reconcilers needed).
// It runs on every operator replica independently (NeedLeaderElection: false).
type CABundleWatcher struct {
	client.Client
	Recorder  record.EventRecorder
	Namespace string

	// cachedClient holds the current *http.Client atomically.  Reads never block.
	cachedClient atomic.Pointer[http.Client]
}

// GetHTTPClient returns the cached *http.Client backed by the current CA bundle.
// The returned client must be used only for the duration of a single reconcile
// invocation and must never be stored on a struct field.
// If no valid CA bundle has been loaded yet, a client with system roots is
// returned (nil TLSClientConfig = system roots).
// The insecure_skip_verify annotation is a per-CR concern; callers that need
// InsecureSkipVerify=true should use PortaClientFromAccount or
// PortaClientFromURLWithClient which handle it independently.
func (r *CABundleWatcher) GetHTTPClient() *http.Client {
	c := r.cachedClient.Load()
	if c == nil {
		// No bundle reconciled yet — return a fresh client with system roots.
		return buildHTTPClient(nil)
	}
	return c
}

// buildHTTPClient constructs an *http.Client with the supplied TLS configuration.
// If THREESCALE_DEBUG=1 the verbose transport wrapper is applied.
// A nil tlsConfig means system roots (no custom CA).
func buildHTTPClient(tlsConfig *tls.Config) *http.Client {
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	}
	var transport http.RoundTripper = &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsConfig,
	}
	if pkghelper.GetEnvVar("THREESCALE_DEBUG", "0") == "1" {
		transport = &pkghelper.Transport{Transport: transport}
	}
	return &http.Client{Transport: transport}
}

// SetupWithManager registers the controller with the manager.
func (r *CABundleWatcher) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("cabundlewatcher").
		WithOptions(controller.Options{NeedLeaderElection: ptr.To(false)}).
		For(&corev1.ConfigMap{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return e.Object.GetName() == CABundleConfigMapName &&
					e.Object.GetNamespace() == r.Namespace
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return e.ObjectNew.GetName() == CABundleConfigMapName &&
					e.ObjectNew.GetNamespace() == r.Namespace
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return e.Object.GetName() == CABundleConfigMapName &&
					e.Object.GetNamespace() == r.Namespace
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return e.Object.GetName() == CABundleConfigMapName &&
					e.Object.GetNamespace() == r.Namespace
			},
		})).
		WithLogConstructor(func(_ *reconcile.Request) logr.Logger {
			return mgr.GetLogger().WithValues("controller", "cabundlewatcher")
		}).
		Complete(r)
}

// Reconcile fetches the CA bundle ConfigMap and rebuilds the cached client.
// On a missing ConfigMap, a system-roots client is stored.
// On an invalid bundle, the error is logged and recorded as a Warning event on
// the ConfigMap; the existing client is left unchanged so capability controllers
// continue to operate with the last known good CA.
func (r *CABundleWatcher) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	cm := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Namespace: r.Namespace, Name: CABundleConfigMapName}, cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// ConfigMap deleted — fall back to system roots.
			r.cachedClient.Store(buildHTTPClient(nil))
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	tlsConfig, parseErr := parseBundleFromConfigMap(cm)
	if parseErr != nil {
		logger.Error(parseErr, "CA bundle ConfigMap contains an invalid certificate bundle; keeping previous cached client")
		if r.Recorder != nil {
			r.Recorder.Eventf(cm, corev1.EventTypeWarning, "InvalidCABundle", "%v", parseErr)
		}
		// Leave the existing client in place — return nil to avoid requeue.
		return ctrl.Result{}, nil
	}

	// Atomically swap in the new client, closing idle connections on the old one.
	old := r.cachedClient.Swap(buildHTTPClient(tlsConfig))
	if old != nil {
		old.CloseIdleConnections()
	}
	return ctrl.Result{}, nil
}

// parseBundleFromConfigMap parses the CA bundle from a ConfigMap and returns a
// *tls.Config with the custom RootCAs pool set.  Returns nil and an error if
// the ConfigMap key is absent or the PEM data is invalid.
func parseBundleFromConfigMap(cm *corev1.ConfigMap) (*tls.Config, error) {
	val, exists := cm.Data[CABundleConfigMapKey]
	if !exists {
		return nil, nil // no bundle configured — use system roots
	}

	if len(val) == 0 {
		return nil, &CAValidationError{
			Reason:  CAValidationReasonInvalidFormat,
			Message: fmt.Sprintf("Key %q in ConfigMap %s/%s is empty", CABundleConfigMapKey, cm.Namespace, cm.Name),
		}
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM([]byte(val)) {
		return nil, &CAValidationError{
			Reason:  CAValidationReasonInvalidFormat,
			Message: "No valid PEM-encoded certificates found in CA bundle",
		}
	}

	return &tls.Config{RootCAs: certPool}, nil
}
