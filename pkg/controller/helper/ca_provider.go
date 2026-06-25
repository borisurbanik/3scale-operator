package helper

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// CAProvider makes the operator's outbound TLS trust the CA bundle supplied by the
// platform team at runtime. The cache is keyed by ConfigMap ResourceVersion so that
// bundle rotations are picked up on the next reconcile without an operator restart.
type CAProvider struct {
	client              client.Client
	namespace           string
	mu                  sync.RWMutex
	config              *tls.Config
	cachedErr           error
	lastResourceVersion string
}

func NewCAProvider(cl client.Client, namespace string) *CAProvider {
	return &CAProvider{
		client:    cl,
		namespace: namespace,
	}
}

// TLSConfig returns a tls.Config that trusts the operator CA bundle, or nil if
// no bundle is configured. Safe for concurrent use. The PEM is only re-parsed
// when the ConfigMap ResourceVersion changes, keeping reconcile hot paths cheap.
func (p *CAProvider) TLSConfig(ctx context.Context) (*tls.Config, error) {
	cm := &corev1.ConfigMap{}
	err := p.client.Get(ctx, client.ObjectKey{Namespace: p.namespace, Name: CABundleConfigMapName}, cm)

	var currentRV string
	notFound := apierrors.IsNotFound(err)
	if err != nil && !notFound {
		// Prefer stale-but-valid config over surfacing a transient API error mid-reconcile.
		p.mu.RLock()
		config, cachedErr := p.config, p.cachedErr
		p.mu.RUnlock()
		if p.lastResourceVersion != "" || config != nil || cachedErr != nil {
			return config, cachedErr
		}
		return nil, err
	}

	if !notFound {
		currentRV = cm.ResourceVersion
	}

	p.mu.RLock()
	if p.lastResourceVersion == currentRV && (currentRV != "" || notFound) {
		config, cachedErr := p.config, p.cachedErr
		p.mu.RUnlock()
		return config, cachedErr
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Another goroutine may have reloaded while we waited for the write lock.
	if p.lastResourceVersion == currentRV && (currentRV != "" || notFound) {
		return p.config, p.cachedErr
	}

	if notFound {
		p.config = nil
		p.cachedErr = nil
		p.lastResourceVersion = ""
		return nil, nil
	}

	loadErr := p.loadFromConfigMap(cm)
	p.cachedErr = loadErr
	p.lastResourceVersion = currentRV
	return p.config, p.cachedErr
}

// Reload forces a synchronous re-parse regardless of ResourceVersion. Use when
// the caller needs a guarantee that the next TLSConfig returns fresh data (e.g.
// after a failed reconcile that may have cached a transient error).
func (p *CAProvider) Reload(ctx context.Context) error {
	cm := &corev1.ConfigMap{}
	err := p.client.Get(ctx, client.ObjectKey{Namespace: p.namespace, Name: CABundleConfigMapName}, cm)

	p.mu.Lock()
	defer p.mu.Unlock()

	if apierrors.IsNotFound(err) {
		p.config = nil
		p.cachedErr = nil
		p.lastResourceVersion = ""
		return nil
	}
	if err != nil {
		p.cachedErr = err
		p.lastResourceVersion = ""
		return err
	}

	loadErr := p.loadFromConfigMap(cm)
	p.cachedErr = loadErr
	p.lastResourceVersion = cm.ResourceVersion
	return loadErr
}

// loadFromConfigMap must be called with p.mu write-locked.
func (p *CAProvider) loadFromConfigMap(cm *corev1.ConfigMap) error {
	p.config = nil

	val, exists := cm.Data[CABundleConfigMapKey]
	if !exists {
		return nil
	}

	if len(val) == 0 {
		return &CAValidationError{
			Reason:  CAValidationReasonInvalidFormat,
			Message: fmt.Sprintf("Key %q in ConfigMap %s/%s is empty", CABundleConfigMapKey, cm.Namespace, cm.Name),
		}
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM([]byte(val)) {
		return &CAValidationError{
			Reason:  CAValidationReasonInvalidFormat,
			Message: "No valid PEM-encoded certificates found in CA bundle",
		}
	}

	p.config = &tls.Config{
		RootCAs: certPool,
	}
	return nil
}
