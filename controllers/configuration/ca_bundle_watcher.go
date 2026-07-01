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

// CABundleWatcher watches the threescale-ca-bundle ConfigMap and writes the
// parsed *tls.Config to the package-level variable via SetTLSConfig on every
// successful reconcile.
type CABundleWatcher struct {
	client.Client
	Recorder  record.EventRecorder
	Namespace string
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

// Reconcile fetches the CA bundle ConfigMap and updates the package-level
// TLS config.  On an invalid bundle, the error is logged and recorded as a Warning event on
// the ConfigMap; the existing TLS config is left unchanged so capability
// controllers continue to operate with the last known good CA.
func (r *CABundleWatcher) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	cm := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Namespace: r.Namespace, Name: CABundleConfigMapName}, cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			SetTLSConfig(nil)
			logger.Info("CA bundle ConfigMap not found; using system default CAs", "namespace", r.Namespace, "configmap", CABundleConfigMapName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	tlsConfig, parseErr := parseBundleFromConfigMap(cm)
	if parseErr != nil {
		logger.Error(parseErr, "CA bundle ConfigMap contains an invalid certificate bundle; keeping previous TLS config")
		if r.Recorder != nil {
			r.Recorder.Eventf(cm, corev1.EventTypeWarning, "InvalidCABundle", "%v", parseErr)
		}
		return ctrl.Result{}, nil
	}

	if tlsConfig == nil {
		SetTLSConfig(nil)
		logger.Info("CA bundle ConfigMap key absent; using system default CAs", "namespace", r.Namespace, "configmap", CABundleConfigMapName, "key", CABundleConfigMapKey)
		return ctrl.Result{}, nil
	}

	SetTLSConfig(tlsConfig)
	logger.Info("CA bundle updated and applied", "namespace", r.Namespace, "configmap", CABundleConfigMapName)
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
