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

// PortaClient instantiates porta_client.ThreeScaleClient from ProviderAccount object
func PortaClient(providerAccount *ProviderAccount, insecureSkipVerify bool) (*threescaleapi.ThreeScaleClient, error) {
	return PortaClientFromURLString(providerAccount.AdminURLStr, providerAccount.Token, insecureSkipVerify)
}

// PortaClientWithTLSConfig instantiates porta_client.ThreeScaleClient from a ProviderAccount
// with an optional *tls.Config. When tlsConfig is non-nil it is used directly as the transport's
// TLS configuration and insecureSkipVerify is ignored. When tlsConfig is nil the function behaves
// identically to PortaClient.
func PortaClientWithTLSConfig(providerAccount *ProviderAccount, tlsConfig *tls.Config, insecureSkipVerify bool) (*threescaleapi.ThreeScaleClient, error) {
	adminURL, err := url.Parse(providerAccount.AdminURLStr)
	if err != nil {
		return nil, err
	}
	return PortaClientFromURLWithTLSConfig(adminURL, providerAccount.Token, tlsConfig, insecureSkipVerify)
}

// PortaClientFromURLString instantiates porta_client.ThreeScaleClient from url string
func PortaClientFromURLString(adminURLStr, token string, insecureSkipVerify bool) (*threescaleapi.ThreeScaleClient, error) {
	adminURL, err := url.Parse(adminURLStr)
	if err != nil {
		return nil, err
	}
	return PortaClientFromURL(adminURL, token, insecureSkipVerify)
}

// PortaClientFromURL instantiates porta_client.ThreeScaleClient from admin url object
func PortaClientFromURL(url *url.URL, token string, insecureSkipVerify bool) (*threescaleapi.ThreeScaleClient, error) {
	adminPortal, err := threescaleapi.NewAdminPortal(url.Scheme, url.Hostname(), helper.PortFromURL(url))
	if err != nil {
		return nil, err
	}

	// Activated by some env var or Spec param
	var transport http.RoundTripper = &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify},
	}

	if helper.GetEnvVar(HTTP_VERBOSE_ENVVAR, "0") == "1" {
		transport = &helper.Transport{Transport: transport}
	}

	return threescaleapi.NewThreeScale(adminPortal, token, &http.Client{Transport: transport}), nil
}

// PortaClientFromURLWithTLSConfig instantiates porta_client.ThreeScaleClient from admin url object
// with an optional *tls.Config. When tlsConfig is non-nil it is used directly as the transport's
// TLS configuration and insecureSkipVerify is ignored. When tlsConfig is nil the function behaves
// identically to PortaClientFromURL.
func PortaClientFromURLWithTLSConfig(url *url.URL, token string, tlsConfig *tls.Config, insecureSkipVerify bool) (*threescaleapi.ThreeScaleClient, error) {
	if tlsConfig == nil {
		return PortaClientFromURL(url, token, insecureSkipVerify)
	}

	adminPortal, err := threescaleapi.NewAdminPortal(url.Scheme, url.Hostname(), helper.PortFromURL(url))
	if err != nil {
		return nil, err
	}

	var transport http.RoundTripper = &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsConfig,
	}

	if helper.GetEnvVar(HTTP_VERBOSE_ENVVAR, "0") == "1" {
		transport = &helper.Transport{Transport: transport}
	}

	return threescaleapi.NewThreeScale(adminPortal, token, &http.Client{Transport: transport}), nil
}

// GetInsecureSkipVerifyAnnotation extracts the insecure_skip_verify annotation from an object
func GetInsecureSkipVerifyAnnotation(annotations map[string]string) bool {
	insecureSkipVerify, ok := annotations["insecure_skip_verify"]
	if ok && insecureSkipVerify == "true" {
		return true
	}
	return false
}
