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
	// CABundleConfigMapName is the well-known ConfigMap name for the CA bundle
	CABundleConfigMapName = "threescale-ca-bundle"
	// CABundleConfigMapKey is the well-known key in the ConfigMap containing the CA bundle
	CABundleConfigMapKey = "ca-bundle.crt"
)

// CAValidationError represents errors that occur during CA validation
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

// CA validation error reasons
const (
	CAValidationReasonMissingSecret = "MissingCASecret"
	CAValidationReasonMissingKey    = "MissingCAKey"
	CAValidationReasonInvalidFormat = "InvalidCAFormat"
)

// CAProvider loads, validates, and caches the CA bundle from the well-known ConfigMap
type CAProvider struct {
	client    client.Client
	namespace string
	mu        sync.RWMutex
	config    *tls.Config
	err       error
	loaded    bool
}

// NewCAProvider constructs a new CAProvider
func NewCAProvider(cl client.Client, namespace string) *CAProvider {
	return &CAProvider{
		client:    cl,
		namespace: namespace,
	}
}

// TLSConfig returns the cached tls.Config
func (p *CAProvider) TLSConfig() (*tls.Config, error) {
	p.mu.RLock()
	if p.loaded {
		config, err := p.config, p.err
		p.mu.RUnlock()
		return config, err
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.loaded {
		return p.config, p.err
	}

	err := p.load(context.TODO())
	p.loaded = true
	p.err = err
	return p.config, p.err
}

// Reload re-reads the ConfigMap and updates the cached state
func (p *CAProvider) Reload(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	err := p.load(ctx)
	p.loaded = true
	p.err = err
	return err
}

func getConfigMapKey(ctx context.Context, cl client.Client, namespace, name, key string) ([]byte, error) {
	cm := &corev1.ConfigMap{}
	err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	val, exists := cm.Data[key]
	if !exists {
		return nil, nil
	}

	if len(val) == 0 {
		return nil, &CAValidationError{
			Reason:  CAValidationReasonInvalidFormat,
			Message: fmt.Sprintf("Key %q in ConfigMap %s/%s is empty", key, namespace, name),
		}
	}

	return []byte(val), nil
}

func (p *CAProvider) load(ctx context.Context) error {
	p.config = nil // default; overwritten only on success path

	caData, err := getConfigMapKey(ctx, p.client, p.namespace, CABundleConfigMapName, CABundleConfigMapKey)
	if err != nil {
		return err
	}
	if caData == nil {
		return nil
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caData) {
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
