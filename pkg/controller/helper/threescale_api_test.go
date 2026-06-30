package helper

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	configuration "github.com/3scale/3scale-operator/controllers/configuration"
	threescaleapi "github.com/3scale/3scale-porta-go-client/client"
)

// backendListHandler serves a minimal valid /admin/api/backend_apis.json response.
func backendListHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(threescaleapi.BackendApiList{}); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	})
}

// setTLSConfigForTest sets the package-level TLS config and restores the
// previous value when the test completes.  Must NOT be called from parallel
// tests — t.Setenv is used as a guard.
func setTLSConfigForTest(t *testing.T, cfg *tls.Config) {
	t.Helper()
	t.Setenv("_TEST_TLS_GUARD", "1")
	prev := configuration.GetTLSConfig()
	configuration.SetTLSConfig(cfg)
	t.Cleanup(func() { configuration.SetTLSConfig(prev) })
}

// TestPortaClientFromAccount_InvalidURL verifies that an unparseable admin URL
// is rejected before any network I/O.
func TestPortaClientFromAccount_InvalidURL(t *testing.T) {
	providerAccount := &ProviderAccount{AdminURLStr: ":foo", Token: "some token"}
	_, err := PortaClient(providerAccount, false)
	assert(t, err != nil, "error should not be nil")
}

// TestPortaClientFromAccount_Valid verifies that a valid admin URL produces a
// usable client.
func TestPortaClientFromAccount_Valid(t *testing.T) {
	providerAccount := &ProviderAccount{AdminURLStr: "http://somedomain.example.com", Token: "some token"}
	_, err := PortaClient(providerAccount, false)
	ok(t, err)
}

// TestPortaClientFromURL_InvalidURL verifies that an empty URL (no
// scheme, no host) is rejected.
func TestPortaClientFromURL_InvalidURL(t *testing.T) {
	_, err := PortaClientFromURL(&url.URL{}, "some token", false)
	assert(t, err != nil, "error should not be nil")
}

// TestPortaClientFromURL_TLSRejectUntrusted: with no TLS config set,
// a self-signed httptest TLS server must be rejected.
func TestPortaClientFromURL_TLSRejectUntrusted(t *testing.T) {
	srv := httptest.NewTLSServer(backendListHandler(t))
	defer srv.Close()

	srvURL, err := url.Parse(srv.URL)
	ok(t, err)

	setTLSConfigForTest(t, nil)

	c, err := PortaClientFromURL(srvURL, "token", false)
	ok(t, err)

	_, reqErr := c.ListBackendApis()
	assert(t, reqErr != nil, "expected TLS certificate error, got nil")
	assert(t, strings.Contains(reqErr.Error(), "certificate"), "expected certificate error, got: %v", reqErr)
}

// TestPortaClientFromURL_TLSMatchingCA: setting the package-level TLS
// config to trust the server's own CA must allow a successful request.
func TestPortaClientFromURL_TLSMatchingCA(t *testing.T) {
	srv := httptest.NewTLSServer(backendListHandler(t))
	defer srv.Close()

	certPool := x509.NewCertPool()
	for _, cert := range srv.TLS.Certificates {
		for _, c := range cert.Certificate {
			parsedCert, err := x509.ParseCertificate(c)
			ok(t, err)
			certPool.AddCert(parsedCert)
		}
	}
	setTLSConfigForTest(t, &tls.Config{RootCAs: certPool})

	srvURL, err := url.Parse(srv.URL)
	ok(t, err)

	c, err := PortaClientFromURL(srvURL, "token", false)
	ok(t, err)

	_, reqErr := c.ListBackendApis()
	ok(t, reqErr)
}

// TestPortaClientFromURL_InsecureSkipVerify: insecureSkipVerify=true
// must accept an untrusted server certificate regardless of the TLS config.
func TestPortaClientFromURL_InsecureSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(backendListHandler(t))
	defer srv.Close()

	srvURL, err := url.Parse(srv.URL)
	ok(t, err)

	setTLSConfigForTest(t, nil)

	c, err := PortaClientFromURL(srvURL, "token", true)
	ok(t, err)

	_, reqErr := c.ListBackendApis()
	ok(t, reqErr)
}
