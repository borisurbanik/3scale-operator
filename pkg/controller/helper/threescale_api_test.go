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

func TestPortaClientInvalidURL(t *testing.T) {
	providerAccount := &ProviderAccount{AdminURLStr: ":foo", Token: "some token"}
	_, err := PortaClient(providerAccount, false)
	assert(t, err != nil, "error should not be nil")
}

func TestPortaClient(t *testing.T) {
	providerAccount := &ProviderAccount{AdminURLStr: "http://somedomain.example.com", Token: "some token"}
	_, err := PortaClient(providerAccount, false)
	ok(t, err)
}

func TestPortaClientFromURLStringInvalidURL(t *testing.T) {
	_, err := PortaClientFromURLString(":foo", "some token", false)
	assert(t, err != nil, "error should not be nil")
}

func TestPortaClientFromURLString(t *testing.T) {
	_, err := PortaClientFromURLString("http://somedomain.example.com", "some token", false)
	ok(t, err)
}

func TestPortaClientFromURL(t *testing.T) {
	url := &url.URL{}
	_, err := PortaClientFromURL(url, "some token", false)
	assert(t, err != nil, "error should not be nil")
}

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

// TestPortaClientFromURLWithTLSConfig_NilFallsBackToInsecureSkipVerify: nil tlsConfig with
// insecureSkipVerify=false must reject an untrusted server certificate.
func TestPortaClientFromURLWithTLSConfig_NilFallsBackToInsecureSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(backendListHandler(t))
	defer srv.Close()

	srvURL, err := url.Parse(srv.URL)
	ok(t, err)

	client, err := PortaClientFromURLWithTLSConfig(srvURL, "token", nil, false)
	ok(t, err)

	_, reqErr := client.ListBackendApis()
	assert(t, reqErr != nil, "expected TLS certificate error, got nil")
	assert(t, strings.Contains(reqErr.Error(), "certificate"), "expected certificate error, got: %v", reqErr)
}

// TestPortaClientFromURLWithTLSConfig_TLSServerWithMatchingCA: a non-nil tlsConfig containing
// the server's CA must be accepted and insecureSkipVerify must be ignored.
func TestPortaClientFromURLWithTLSConfig_TLSServerWithMatchingCA(t *testing.T) {
	srv := httptest.NewTLSServer(backendListHandler(t))
	defer srv.Close()

	// srv.Client() is pre-configured to trust the server's self-signed cert; borrow its TLS config.
	tlsConfig := srv.Client().Transport.(*http.Transport).TLSClientConfig

	srvURL, err := url.Parse(srv.URL)
	ok(t, err)

	client, err := PortaClientFromURLWithTLSConfig(srvURL, "token", tlsConfig, false)
	ok(t, err)

	// ListBackendApis triggers a real HTTPS request through the client the function built.
	_, reqErr := client.ListBackendApis()
	ok(t, reqErr)
}

// TestPortaClientFromURLWithTLSConfig_NilTLSConfigInsecureSkipVerify: nil tlsConfig with
// insecureSkipVerify=true must accept an untrusted server certificate.
func TestPortaClientFromURLWithTLSConfig_NilTLSConfigInsecureSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(backendListHandler(t))
	defer srv.Close()

	srvURL, err := url.Parse(srv.URL)
	ok(t, err)

	client, err := PortaClientFromURLWithTLSConfig(srvURL, "token", nil, true)
	ok(t, err)

	_, reqErr := client.ListBackendApis()
	ok(t, reqErr)
}
