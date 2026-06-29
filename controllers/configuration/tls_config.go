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
	"crypto/tls"
	"sync"
)

// currentTLSConfig is the package-level TLS configuration derived from the
// most recently reconciled threescale-ca-bundle ConfigMap.
// nil means "no custom CA bundle" — callers should use system roots.
var currentTLSConfig *tls.Config

var tlsConfigMu sync.RWMutex

// SetTLSConfig replaces the current package-level TLS config.
// Pass nil to revert to system roots (e.g. when the ConfigMap is deleted).
func SetTLSConfig(cfg *tls.Config) {
	tlsConfigMu.Lock()
	currentTLSConfig = cfg
	tlsConfigMu.Unlock()
}

// GetTLSConfig returns the current package-level TLS config.
// Returns nil if no bundle has been reconciled yet, or after the ConfigMap
// has been deleted.  Callers must not mutate the returned *tls.Config.
func GetTLSConfig() *tls.Config {
	tlsConfigMu.RLock()
	cfg := currentTLSConfig
	tlsConfigMu.RUnlock()
	return cfg
}
