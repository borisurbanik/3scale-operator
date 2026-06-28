package helper

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

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

// TestPortaClientFromAccount_InvalidURL verifies that an unparseable admin URL
// is rejected before any network I/O.
func TestPortaClientFromAccount_InvalidURL(t *testing.T) {
	providerAccount := &ProviderAccount{AdminURLStr: ":foo", Token: "some token"}
	_, err := PortaClientFromAccount(providerAccount, &http.Client{}, false)
	assert(t, err != nil, "error should not be nil")
}

// TestPortaClientFromAccount_Valid verifies that a valid admin URL produces a
// usable client.
func TestPortaClientFromAccount_Valid(t *testing.T) {
	providerAccount := &ProviderAccount{AdminURLStr: "http://somedomain.example.com", Token: "some token"}
	_, err := PortaClientFromAccount(providerAccount, &http.Client{}, false)
	ok(t, err)
}

// TestPortaClientFromURLWithClient_InvalidURL verifies that an empty URL (no
// scheme, no host) is rejected.
func TestPortaClientFromURLWithClient_InvalidURL(t *testing.T) {
	_, err := PortaClientFromURLWithClient(&url.URL{}, "some token", &http.Client{}, false)
	assert(t, err != nil, "error should not be nil")
}

// TestPortaClientFromURLWithClient_TLSRejectUntrusted: a nil RootCAs client
// (system roots, no custom CA) must reject an httptest TLS server whose
// self-signed cert is not in the system pool.
func TestPortaClientFromURLWithClient_TLSRejectUntrusted(t *testing.T) {
	srv := httptest.NewTLSServer(backendListHandler(t))
	defer srv.Close()

	srvURL, err := url.Parse(srv.URL)
	ok(t, err)

	// Use a plain http.Client — system roots don't trust the test server.
	c, err := PortaClientFromURLWithClient(srvURL, "token", &http.Client{}, false)
	ok(t, err)

	_, reqErr := c.ListBackendApis()
	assert(t, reqErr != nil, "expected TLS certificate error, got nil")
	assert(t, strings.Contains(reqErr.Error(), "certificate"), "expected certificate error, got: %v", reqErr)
}

// TestPortaClientFromURLWithClient_TLSMatchingCA: providing the server's own CA
// must allow a successful request.
func TestPortaClientFromURLWithClient_TLSMatchingCA(t *testing.T) {
	srv := httptest.NewTLSServer(backendListHandler(t))
	defer srv.Close()

	// srv.Client() is pre-configured to trust the server's self-signed cert.
	trustedClient := srv.Client()

	srvURL, err := url.Parse(srv.URL)
	ok(t, err)

	c, err := PortaClientFromURLWithClient(srvURL, "token", trustedClient, false)
	ok(t, err)

	_, reqErr := c.ListBackendApis()
	ok(t, reqErr)
}

// TestPortaClientFromURLWithClient_InsecureSkipVerify: insecureSkipVerify=true
// must accept an untrusted server certificate, ignoring the supplied httpClient.
func TestPortaClientFromURLWithClient_InsecureSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(backendListHandler(t))
	defer srv.Close()

	srvURL, err := url.Parse(srv.URL)
	ok(t, err)

	// Pass a plain (untrusted) client; the insecureSkipVerify flag overrides it.
	c, err := PortaClientFromURLWithClient(srvURL, "token", &http.Client{}, true)
	ok(t, err)

	_, reqErr := c.ListBackendApis()
	ok(t, reqErr)
}
