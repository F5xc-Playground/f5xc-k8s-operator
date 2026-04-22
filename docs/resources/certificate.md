# Certificate

A Certificate reads a TLS certificate and private key from a Kubernetes `kubernetes.io/tls` Secret and syncs it to F5 XC. This lets you manage certificates in Kubernetes and reference them from HTTPLoadBalancers that use explicit HTTPS configuration.

**Short name:** `cert`

## F5 XC Documentation

- [Certificate API Reference](https://docs.cloud.f5.com/docs-v2/api/certificates-certificate)

## Examples

### Basic certificate from a K8s TLS Secret

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: Certificate
metadata:
  name: my-cert
spec:
  xcNamespace: my-namespace
  secretRef:
    name: my-tls-secret
```

### Certificate in the shared XC namespace

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: Certificate
metadata:
  name: shared-cert
spec:
  xcNamespace: shared
  secretRef:
    name: wildcard-tls
```

### Certificate with OCSP stapling disabled

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: Certificate
metadata:
  name: no-ocsp-cert
spec:
  xcNamespace: my-namespace
  secretRef:
    name: my-tls-secret
  disableOcspStapling: {}
```

### Certificate with system-default OCSP stapling

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: Certificate
metadata:
  name: ocsp-cert
spec:
  xcNamespace: my-namespace
  secretRef:
    name: my-tls-secret
  useSystemDefaults: {}
```

## Prerequisites

Create a `kubernetes.io/tls` Secret containing your certificate and private key:

```bash
kubectl create secret tls my-tls-secret \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key
```

The Secret must have type `kubernetes.io/tls` with `tls.crt` and `tls.key` data keys.

## Spec Reference

> **Full field reference:** [Certificate API Documentation](https://docs.cloud.f5.com/docs-v2/api/certificates-certificate)
>
> The operator reads the TLS certificate and private key from the referenced Kubernetes Secret, base64-encodes them, and sends them to the XC API using the `string:///` URL format.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `xcNamespace` | string | Yes | F5 XC namespace |
| `secretRef` | object | Yes | Reference to a `kubernetes.io/tls` Secret |
| `secretRef.name` | string | Yes | Name of the Secret |
| `secretRef.namespace` | string | No | Namespace of the Secret (defaults to the Certificate CR's namespace) |

### OCSP Stapling (choose one, all optional)

Omitting all OCSP fields disables OCSP stapling (API default).

| Field | Description |
|-------|-------------|
| `disableOcspStapling` | `{}` to explicitly disable OCSP stapling |
| `useSystemDefaults` | `{}` to use system default OCSP settings |
| `customHashAlgorithms` | Custom hash algorithm configuration |

All OneOf fields accept JSON objects passed through to the XC API.
