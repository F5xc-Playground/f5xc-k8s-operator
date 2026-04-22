# APIDefinition

An APIDefinition describes the API endpoints of your application, including swagger specs, inclusion/exclusion lists, and schema origin strategy. It is referenced from an HTTPLoadBalancer via the `apiSpecification` field.

**Short name:** `apidef`

## F5 XC Documentation

- [API Definition (How-To)](https://docs.cloud.f5.com/docs/how-to/app-security/api-definition)
- [API Definition API Reference](https://docs.cloud.f5.com/docs-v2/api/api-definition)

## Examples

### Basic with swagger spec

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: APIDefinition
metadata:
  name: petstore-api
spec:
  xcNamespace: my-namespace
  swaggerSpecs:
    - /api/object_store/namespaces/my-namespace/stored_objects/swagger/petstore/v1
  mixedSchemaOrigin: {}
```

### With inventory lists

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: APIDefinition
metadata:
  name: custom-api
spec:
  xcNamespace: my-namespace
  apiInventoryInclusionList:
    - method: GET
      path: /api/users
    - method: POST
      path: /api/users
  apiInventoryExclusionList:
    - method: DELETE
      path: /api/admin
  nonAPIEndpoints:
    - method: GET
      path: /health
  strictSchemaOrigin: {}
```

### Reference from an HTTPLoadBalancer

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: HTTPLoadBalancer
metadata:
  name: my-app
spec:
  xcNamespace: my-namespace
  # ...
  apiSpecification:
    apiDefinition:
      name: petstore-api
    validationDisabled: {}
```

## Spec Reference

> **Full field reference:** [API Definition API Documentation](https://docs.cloud.f5.com/docs-v2/api/api-definition)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `xcNamespace` | string | Yes | F5 XC namespace |
| `swaggerSpecs` | []string | No | List of swagger spec URLs from the XC object store |
| `apiInventoryInclusionList` | []APIOperation | No | Endpoints to include in API inventory |
| `apiInventoryExclusionList` | []APIOperation | No | Endpoints to exclude from API inventory |
| `nonAPIEndpoints` | []APIOperation | No | Endpoints that are not API endpoints |

### APIOperation

| Field | Type | Description |
|-------|------|-------------|
| `method` | string | HTTP method (GET, POST, etc.) |
| `path` | string | URL path |

### Schema Origin Strategy (choose one)

| Field | Description |
|-------|-------------|
| `mixedSchemaOrigin` | `{}` to allow schemas from both swagger and learned sources |
| `strictSchemaOrigin` | `{}` to only use schemas from swagger specs |
