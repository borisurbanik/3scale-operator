package helper

import (
	"crypto/tls"
	"net/http"
	"net/url"

	"github.com/3scale/3scale-operator/pkg/helper"

	threescaleapi "github.com/3scale/3scale-porta-go-client/client"
)

const (
	HTTP_VERBOSE_ENVVAR = "THREESCALE_DEBUG"
)

type ProviderAccount struct {
	AdminURLStr string
	Token       string
}

// PortaClientFromAccount instantiates a ThreeScaleClient from a ProviderAccount
// using the supplied *http.Client.  The httpClient is expected to have been
// obtained from an HTTPClientSource for the current reconcile invocation.
// When insecureSkipVerify is true, httpClient is ignored and a fresh client
// with InsecureSkipVerify=true is used instead (per-CR annotation behaviour).
func PortaClientFromAccount(account *ProviderAccount, httpClient *http.Client, insecureSkipVerify bool) (*threescaleapi.ThreeScaleClient, error) {
	adminURL, err := url.Parse(account.AdminURLStr)
	if err != nil {
		return nil, err
	}
	return PortaClientFromURLWithClient(adminURL, account.Token, httpClient, insecureSkipVerify)
}

// PortaClientFromURLWithClient instantiates a ThreeScaleClient from an admin URL
// and access token using the supplied *http.Client.  Used by TenantReconciler
// which works with a raw *url.URL rather than a ProviderAccount.
// When insecureSkipVerify is true, httpClient is ignored and a fresh client
// with InsecureSkipVerify=true is used instead (per-CR annotation behaviour).
func PortaClientFromURLWithClient(adminURL *url.URL, token string, httpClient *http.Client, insecureSkipVerify bool) (*threescaleapi.ThreeScaleClient, error) {
	if insecureSkipVerify {
		var transport http.RoundTripper = &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
		if helper.GetEnvVar(HTTP_VERBOSE_ENVVAR, "0") == "1" {
			transport = &helper.Transport{Transport: transport}
		}
		httpClient = &http.Client{Transport: transport}
	}
	adminPortal, err := threescaleapi.NewAdminPortal(adminURL.Scheme, adminURL.Hostname(), helper.PortFromURL(adminURL))
	if err != nil {
		return nil, err
	}
	return threescaleapi.NewThreeScale(adminPortal, token, httpClient), nil
}

// GetInsecureSkipVerifyAnnotation extracts the insecure_skip_verify annotation from an object
func GetInsecureSkipVerifyAnnotation(annotations map[string]string) bool {
	insecureSkipVerify, ok := annotations["insecure_skip_verify"]
	if ok && insecureSkipVerify == "true" {
		return true
	}
	return false
}
