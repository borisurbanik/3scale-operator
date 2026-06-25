package helper

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// validCAPEM is a real certificate generated with openssl
const validCAPEM = `-----BEGIN CERTIFICATE-----
MIIDBTCCAe2gAwIBAgIUTis/OFTnk7aWS8pnXi2XFg5CmOowDQYJKoZIhvcNAQEL
BQAwEjEQMA4GA1UEAwwHVGVzdCBDQTAeFw0yNjA1MzEyMDI2MDhaFw0yNzA1MzEy
MDI2MDhaMBIxEDAOBgNVBAMMB1Rlc3QgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IB
DwAwggEKAoIBAQCm1D8xoFobfFRdwx++dNXES4lfuBkAS1cEFq+DG439L6HSOJ9s
bunZh9MHb6Lz8EWZ1B86H0NqAC+npDbxEYEEe6DDhR5Gg4yEvb+yTjHiNQln8Y+R
/VUCbn7ChyRAHTq9ybDD51wkaGxcfo5agj55cX7qNHnHUyMYUFzAwAOIWLWAoubu
K4JPFcSKjg7vxYntN0/5nYm6r88hKTp0IByRhiEwStB7Ui7a3NlkDZW4lVXy7U9h
7bd4em3hJtt+HRvGl4BhcXZ9dQ+zUZxZ2zEYYTeuZS/z9tRhp1IFdXJRW9YyXpQQ
SMP6YHM1gcAit2R7walLkjDSk+dJ4mGXmL01AgMBAAGjUzBRMB0GA1UdDgQWBBSa
rDHNx4p+LHSLFtYqOjVzNcm8pTAfBgNVHSMEGDAWgBSarDHNx4p+LHSLFtYqOjVz
Ncm8pTAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQBPVUtJA0to
BLUCMPFBTtApVmVyNX5+zqPscLYwazgJBBmZARfTmN3f+oaqh2WDurM98bNOk664
sWBXFVyhHnhvEoy2m+q2ERnhEcxXFFXmYOi2eDGiQ9s1qoeeF8/ioaq/pWAWqJ6s
0eTxer9bNaQXxQHWPyzZNh5l4VyoVBgfB9EsOptWLlos6qxEFz4auN8kLZS9CUaw
4IgB/a4NOPHMTf+CbeqcKObI4UbY95IkAJAARV3zgi8TnVXOkqa6Nmd1QH4pplGw
XOBXITbO8TOJxIbopUep5sJJSkk/aIpmRxLw2B26yyFd01BaMu62VBIComrlOkwX
uCY76vgFQ92B
-----END CERTIFICATE-----`

const validCA2PEM = `-----BEGIN CERTIFICATE-----
MIIDCTCCAfGgAwIBAgIUUbl96SbTF1Whp8UMacHvPz3RkPMwDQYJKoZIhvcNAQEL
BQAwFDESMBAGA1UEAwwJVGVzdCBDQSAyMB4XDTI2MDUzMTIwMzEyOVoXDTI3MDUz
MTIwMzEyOVowFDESMBAGA1UEAwwJVGVzdCBDQSAyMIIBIjANBgkqhkiG9w0BAQEF
AAOCAQ8AMIIBCgKCAQEAlIh6ezvFyAdLD2HJTfrio+hLkkV3hxEN61fmJxnY4oDK
HgqW5VjMoTwGCokafGkctcwpFlbWIWqgFwNulUF7kJ3i3ramVOsO1YM66zj4Wzl5
zZF1pSYNa3iEcX6nXER/1VATaiyBztL6Mcbc3N+TUjeVvt70uSWYWvlocZHqKVxV
hN/GEq97A89UZSXTf/YBX1CHhPFo8NXUk5ul9D5lwpj+Mp3MoBzZIhkuoQL8/GGF
+MZnomvJlb3wshtt21i1qMZpzk2uiLAyIpozwPaQgPviRHFEHMKUA//ONEBnolcu
tsMmrEf78OMO4LHPDXlL2UJW2hNKM6Ur90iyv4fjRQIDAQABo1MwUTAdBgNVHQ4E
FgQUE/zk/QH/1dISPMjW8q590bw05H0wHwYDVR0jBBgwFoAUE/zk/QH/1dISPMjW
8q590bw05H0wDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAXNQ9
2qzrRlDS/hFSmfuGXGvGp82arVCWX4QEiI7fRLBvCNzSS48rvtUh91c9aTayaQ5N
u6EqblImhDJ+dGlBUcSKHa3D5orArcWB4vvr470QDdqhzsqP0ctEE0z9WB+OUfAE
5W4OZgXr/1VH68TpO6AM2VaLzDSHPwkIRgRgy5EkiKIKuHEpv7GvslS3i8Be8Tj2
/w2dIzHQ05D/CE+tsdWUbGUwPkktPy5a3UE6JeurQ1KRNzHAWstuDUo5fjfWCL5o
D9hxDwhqDmzxDwBP1qqQxaQTRLDfMXlSvDfwIpCk7OIfHsE2gXTqZt7jBsQOLChW
ad4dva2DS9/WrTQq/g==
-----END CERTIFICATE-----`

const testNamespace = "test-operator-namespace"

func createConfigMap(name, key, data string) *corev1.ConfigMap {
	var cmData map[string]string
	if key != "" {
		cmData = map[string]string{
			key: data,
		}
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
		},
		Data: cmData,
	}
}

func TestCAProvider_TLSConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name        string
		configMap   *corev1.ConfigMap
		expectNil   bool
		expectError bool
	}{
		{
			name:        "ConfigMap present with valid PEM",
			configMap:   createConfigMap(CABundleConfigMapName, CABundleConfigMapKey, validCAPEM),
			expectNil:   false,
			expectError: false,
		},
		{
			name:        "ConfigMap absent",
			configMap:   nil,
			expectNil:   true,
			expectError: false,
		},
		{
			name:        "ConfigMap present but key missing",
			configMap:   createConfigMap(CABundleConfigMapName, "", ""),
			expectNil:   true,
			expectError: false,
		},
		{
			name:        "ConfigMap present with empty key value",
			configMap:   createConfigMap(CABundleConfigMapName, CABundleConfigMapKey, ""),
			expectNil:   true,
			expectError: true,
		},
		{
			name:        "ConfigMap present with invalid PEM",
			configMap:   createConfigMap(CABundleConfigMapName, CABundleConfigMapKey, "not-a-valid-certificate-data"),
			expectNil:   true,
			expectError: true,
		},
		{
			name:        "ConfigMap present with multiple certificates",
			configMap:   createConfigMap(CABundleConfigMapName, CABundleConfigMapKey, validCAPEM+"\n"+validCA2PEM),
			expectNil:   false,
			expectError: false,
		},
		{
			name: "ConfigMap contains only non-CERTIFICATE blocks",
			configMap: createConfigMap(CABundleConfigMapName, CABundleConfigMapKey, `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAtest
-----END RSA PRIVATE KEY-----`),
			// AppendCertsFromPEM skips non-CERTIFICATE blocks; no certs appended → error
			expectNil:   true,
			expectError: true,
		},
		{
			name: "ConfigMap contains a mix of CERTIFICATE and non-CERTIFICATE blocks",
			configMap: createConfigMap(CABundleConfigMapName, CABundleConfigMapKey, validCAPEM+"\n"+`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAtest
-----END RSA PRIVATE KEY-----`),
			// AppendCertsFromPEM skips non-CERTIFICATE blocks; the valid cert is still appended → no error
			expectNil:   false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cl client.Client
			if tt.configMap != nil {
				cl = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.configMap).Build()
			} else {
				cl = fake.NewClientBuilder().WithScheme(scheme).Build()
			}

			p := NewCAProvider(cl, testNamespace)
			cfg, err := p.TLSConfig(context.TODO())

			if tt.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if cfg != nil {
					t.Fatal("expected tls.Config to be nil on error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectNil {
				if cfg != nil {
					t.Fatalf("expected nil tls.Config, got %v", cfg)
				}
			} else {
				if cfg == nil {
					t.Fatal("expected non-nil tls.Config")
				}
				if cfg.RootCAs == nil {
					t.Fatal("expected RootCAs to be non-nil")
				}
			}
		})
	}
}

func TestCAProvider_Reload(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := createConfigMap(CABundleConfigMapName, CABundleConfigMapKey, validCAPEM)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(cm).Build()

	p := NewCAProvider(cl, testNamespace)

	// First load
	cfg, err := p.TLSConfig(context.TODO())
	if err != nil || cfg == nil {
		t.Fatalf("initial load failed: err=%v, cfg=%v", err, cfg)
	}

	// Update ConfigMap and Reload
	cm.Data[CABundleConfigMapKey] = validCA2PEM
	if err := cl.Update(context.TODO(), cm); err != nil {
		t.Fatalf("failed to update configmap in fake client: %v", err)
	}

	if err := p.Reload(context.TODO()); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	cfg2, err := p.TLSConfig(context.TODO())
	if err != nil || cfg2 == nil {
		t.Fatalf("subsequent load failed: err=%v, cfg=%v", err, cfg2)
	}

	// Delete ConfigMap and Reload
	if err := cl.Delete(context.TODO(), cm); err != nil {
		t.Fatalf("failed to delete configmap in fake client: %v", err)
	}

	if err := p.Reload(context.TODO()); err != nil {
		t.Fatalf("reload after delete failed: %v", err)
	}

	cfg3, err := p.TLSConfig(context.TODO())
	if err != nil {
		t.Fatalf("TLSConfig after reload-delete failed: %v", err)
	}
	if cfg3 != nil {
		t.Fatalf("expected nil tls.Config after deleting ConfigMap, got %v", cfg3)
	}

	// Re-create with invalid PEM and Reload
	cm2 := createConfigMap(CABundleConfigMapName, CABundleConfigMapKey, "invalid-pem")
	if err := cl.Create(context.TODO(), cm2); err != nil {
		t.Fatalf("failed to create invalid configmap: %v", err)
	}

	if err := p.Reload(context.TODO()); err == nil {
		t.Fatal("expected reload to fail with invalid PEM")
	}

	cfg4, err := p.TLSConfig(context.TODO())
	if err == nil {
		t.Fatal("expected TLSConfig to return error after failed reload")
	}
	if cfg4 != nil {
		t.Fatal("expected tls.Config to be nil after failed reload")
	}
}

// TestCAProvider_AutoReload verifies that a ConfigMap update is reflected on the
// next TLSConfig call without requiring an explicit Reload.
func TestCAProvider_AutoReload(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := createConfigMap(CABundleConfigMapName, CABundleConfigMapKey, validCAPEM)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(cm).Build()

	p := NewCAProvider(cl, testNamespace)

	cfg, err := p.TLSConfig(context.TODO())
	if err != nil || cfg == nil {
		t.Fatalf("initial TLSConfig failed: err=%v, cfg=%v", err, cfg)
	}

	cm.Data[CABundleConfigMapKey] = "invalid-pem"
	if err := cl.Update(context.TODO(), cm); err != nil {
		t.Fatalf("failed to update configmap: %v", err)
	}

	cfg2, err := p.TLSConfig(context.TODO())
	if err == nil {
		t.Fatal("expected CAValidationError after ConfigMap update to invalid PEM, got nil")
	}
	if cfg2 != nil {
		t.Fatalf("expected nil tls.Config on error, got %v", cfg2)
	}
}

// TestCAProvider_CacheHit verifies that the CA bundle is not re-parsed on every
// call — repeated calls without a ConfigMap change must return the same object.
func TestCAProvider_CacheHit(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := createConfigMap(CABundleConfigMapName, CABundleConfigMapKey, validCAPEM)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(cm).Build()

	p := NewCAProvider(cl, testNamespace)

	cfg1, err := p.TLSConfig(context.TODO())
	if err != nil || cfg1 == nil {
		t.Fatalf("first TLSConfig failed: err=%v, cfg=%v", err, cfg1)
	}

	cfg2, err := p.TLSConfig(context.TODO())
	if err != nil || cfg2 == nil {
		t.Fatalf("second TLSConfig failed: err=%v, cfg=%v", err, cfg2)
	}

	if cfg1 != cfg2 {
		t.Fatal("expected the same *tls.Config pointer on cache hit, got different pointers")
	}
}
