# Combining CA sources with trust-manager

Use this guide when you need the operator to trust both a cluster-managed CA (via OpenShift CA injection) and a separately managed custom CA. Common cases include multiple 3scale instances signed by different CAs, or a remote OpenAPI spec endpoint that uses a CA not in the cluster bundle.

Once the `config.openshift.io/inject-trusted-cabundle=true` label is set, the Cluster Network Operator (CNO) owns the ConfigMap and overwrites any manual edits, so you cannot add extra certificates directly. [trust-manager](https://cert-manager.io/docs/trust/trust-manager/) solves this by assembling a merged bundle from multiple sources and writing it to the target ConfigMap.

## Prerequisites

Install cert-manager and trust-manager by following the [trust-manager installation guide](https://cert-manager.io/docs/trust/trust-manager/installation/).

## Step 1 — Create a ConfigMap for the cluster CA source

Label a ConfigMap in the `cert-manager` namespace (the trust-manager trust namespace) for OpenShift CA injection. trust-manager will read this as one of its sources:

```bash
kubectl create configmap openshift-ca -n cert-manager
kubectl label configmap openshift-ca \
  config.openshift.io/inject-trusted-cabundle=true \
  -n cert-manager
```

## Step 2 — Create a ConfigMap or Secret for your custom CA

Place your custom CA in a separate ConfigMap (or Secret) also in the `cert-manager` namespace:

```bash
kubectl create configmap my-custom-ca \
  --from-file=ca.crt=my-custom-ca.crt \
  -n cert-manager
```

If your custom CA is already in a cert-manager-managed Secret, you can reference it directly as a `secret` source in the next step — no copy needed.

## Step 3 — Create a Bundle

Create a `Bundle` that merges both sources and writes the result to `threescale-ca-bundle` in the operator namespace:

```yaml
apiVersion: trust.cert-manager.io/v1alpha1
kind: Bundle
metadata:
  name: threescale-ca-bundle
spec:
  sources:
    - configMap:
        name: openshift-ca        # cluster CA injected by CNO (in cert-manager namespace)
        key: ca-bundle.crt
    - configMap:
        name: my-custom-ca        # your custom CA
        key: ca.crt
  target:
    configMap:
      key: ca-bundle.crt          # must match the key the operator reads
    namespaceSelector:
      matchLabels:
        kubernetes.io/metadata.name: <operator-namespace>
```

For a cert-manager-managed Secret source, use:

```yaml
    - secret:
        name: my-ca-secret        # cert-manager Secret in the cert-manager namespace
        key: tls.crt              # prefer tls.crt over ca.crt — see note below
```

> **Which key to use — `tls.crt` or `ca.crt`?**
>
> Use `tls.crt`. cert-manager's `ca.crt` field is populated on a best-effort basis and may be incomplete or missing for some issuer types. `tls.crt` contains the full certificate chain. Where possible, include only root certificates in your trust bundle — trusting intermediates turns them into de facto roots, which makes CA rotation unsafe. See the trust-manager docs on [Bundling Intermediates](https://cert-manager.io/docs/trust/trust-manager/#bundling-intermediates) if you need to trim a chain.

Apply it:

```bash
kubectl apply -f threescale-bundle.yaml
```

Verify it was written to the operator namespace:

```bash
kubectl get configmap threescale-ca-bundle -n <operator-namespace> \
  -o jsonpath='{.data.ca-bundle\.crt}' | openssl x509 -noout -subject -issuer
```

## Verifying the operator picked up the bundle

Check that the operator emitted no validation errors on the ConfigMap:

```bash
kubectl describe configmap threescale-ca-bundle -n <operator-namespace>
```

A successful load produces no `Warning` events. An invalid bundle shows an event similar to:

```
Warning  InvalidCABundle  <timestamp>  CABundleWatcher  InvalidCAFormat: No valid PEM-encoded certificates found in CA bundle
```

If you see this, verify the ConfigMap key contains well-formed PEM data (see the `openssl` verification commands in [operator-ca-bundle.md](operator-ca-bundle.md#verifying-certificates-with-openssl)).
