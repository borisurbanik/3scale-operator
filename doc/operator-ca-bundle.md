# Configuring a custom CA bundle

The 3scale operator supports injecting a custom CA certificate bundle into the TLS configuration used when communicating with external HTTPS services. This is done via a well-known `ConfigMap` that the operator watches in the same namespace.

When the operator finds the `threescale-ca-bundle` ConfigMap, it reads CA certificates from the `ca-bundle.crt` key and uses them as trusted root CAs for all outbound TLS connections. If the ConfigMap is absent or the key is missing, the operator falls back to the system default CA pool.

## Creating the ConfigMap

### Single CA certificate

```bash
kubectl create configmap threescale-ca-bundle \
  --from-file=ca-bundle.crt=my-ca.crt \
  -n <operator-namespace>
```

### Multiple CA certificates (bundle)

Concatenate all PEM root certificates into a single file before creating the ConfigMap:

```bash
cat root-ca-1.crt root-ca-2.crt > ca-bundle.crt

kubectl create configmap threescale-ca-bundle \
  --from-file=ca-bundle.crt=ca-bundle.crt \
  -n <operator-namespace>
```

### OpenShift cluster CA injection

On OpenShift, label the ConfigMap to have the Cluster Network Operator (CNO) automatically populate it with the cluster's merged CA bundle (user-defined proxy CA + built-in CAs):

```bash
kubectl create configmap threescale-ca-bundle -n <operator-namespace>
kubectl label configmap threescale-ca-bundle \
  config.openshift.io/inject-trusted-cabundle=true \
  -n <operator-namespace>
```

The CNO populates the `ca-bundle.crt` key within a few seconds. This covers the common case where the 3scale Admin API is served behind an OpenShift Route signed by a CA that is part of the cluster proxy trust bundle.

> **Note:** Once the `inject-trusted-cabundle=true` label is present, the CNO takes full ownership of the `ca-bundle.crt` key and overwrites any manual edits. To combine the cluster CA with an additional custom CA, see [Combining CA sources with trust-manager](operator-ca-bundle-trust-manager.md).

### Example ConfigMap manifest

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: threescale-ca-bundle
  namespace: <operator-namespace>
data:
  ca-bundle.crt: |
    -----BEGIN CERTIFICATE-----
    <base64-encoded certificate data>
    -----END CERTIFICATE-----
    -----BEGIN CERTIFICATE-----
    <base64-encoded certificate data>
    -----END CERTIFICATE-----
```

## Validation

The operator validates the contents of the `ca-bundle.crt` key when the ConfigMap is created or updated.

| Condition | Operator behaviour |
|---|---|
| ConfigMap absent | TLS uses system default CA pool; no error |
| `ca-bundle.crt` key absent | TLS uses system default CA pool; no error |
| Key present but empty | Error — `InvalidCAFormat`; `Warning` event emitted on the ConfigMap |
| Key contains no valid PEM CERTIFICATE blocks | Error — `InvalidCAFormat`; `Warning` event emitted on the ConfigMap |
| Key contains valid CERTIFICATE blocks plus other PEM block types (e.g. `PRIVATE KEY`) | Non-certificate blocks are silently skipped; certificates are loaded normally |
| Key contains one or more valid CERTIFICATE blocks | Success; TLS config updated immediately |

When validation fails, the operator emits a Kubernetes `Warning` event on the ConfigMap (reason: `InvalidCABundle`) and keeps the last known-good TLS configuration active. Inspect these events with:

```bash
kubectl describe configmap threescale-ca-bundle -n <operator-namespace>
```

## Reloading

The operator watches the `threescale-ca-bundle` ConfigMap for create, update, and delete events. Any change is picked up automatically — **no operator restart or annotation change is required**.

When the ConfigMap is deleted, the TLS configuration reverts to the system default CA pool immediately.

## Verifying certificates with openssl

### Inspect a single certificate

```bash
openssl x509 -in my-ca.crt -text -noout
```

This prints the subject, issuer, validity dates, and extensions of the certificate.

### Inspect all certificates in a bundle

```bash
awk '/-----BEGIN CERTIFICATE-----/,/-----END CERTIFICATE-----/' ca-bundle.crt \
  | awk 'BEGIN{n=0} /-----BEGIN CERTIFICATE-----/{n++; f="cert-"n".pem"} {print > f}'

for f in cert-*.pem; do
  echo "=== $f ==="
  openssl x509 -in "$f" -subject -issuer -dates -noout
done

rm -f cert-*.pem
```

### Verify the chain of trust

Check that a server certificate is signed by one of the CAs in the bundle:

```bash
openssl verify -CAfile ca-bundle.crt server.crt
```

Expected output:

```
server.crt: OK
```

### Test a live TLS connection using the bundle

```bash
openssl s_client -connect <host>:<port> -CAfile ca-bundle.crt -verify_return_error
```

A successful handshake shows `Verify return code: 0 (ok)` near the end of the output.

### Check certificate expiry dates across a bundle

```bash
openssl crl2pkcs7 -nocrl -certfile ca-bundle.crt \
  | openssl pkcs7 -print_certs -noout \
  | grep -A2 "subject"
```

Or using a loop:

```bash
while openssl x509 -noout -subject -dates 2>/dev/null; do :; done < ca-bundle.crt
```

### Generate a self-signed CA certificate for testing

```bash
openssl genrsa -out ca.key 2048

openssl req -x509 -new -nodes \
  -key ca.key \
  -sha256 \
  -days 365 \
  -out ca.crt \
  -subj "/CN=My Test CA"
```

The resulting `ca.crt` can be placed in the `ca-bundle.crt` key of the ConfigMap.
