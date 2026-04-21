# Development

## Prerequisites

- Go 1.26+
- [controller-gen](https://book.kubebuilder.io/reference/controller-gen) (for code generation)
- [setup-envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/tools/setup-envtest) (for integration tests)

Install both with:

```bash
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
```

## Make Targets

```bash
make test              # Unit and integration tests (requires envtest)
make test-contract     # Contract tests against a live XC tenant
make manifests         # Regenerate CRD and RBAC manifests
make generate          # Regenerate deepcopy methods
make fmt               # Format Go source files
make vet               # Run go vet
make lint              # Format + vet
```

## Running Tests

Unit and integration tests use [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest) to spin up a local API server and etcd. No external cluster needed.

```bash
make test
```

### Contract Tests

Contract tests run against a real F5 XC tenant. They create, verify, and delete resources to confirm the operator works end-to-end. Set the following environment variables:

```bash
export XC_TENANT_URL=https://your-tenant.console.ves.volterra.io
export XC_API_TOKEN=your-api-token
export XC_TEST_NAMESPACE=your-xc-namespace   # optional, defaults to "operator-test"

make test-contract
```

## Project Structure

```
api/v1alpha1/           CRD type definitions and shared types
cmd/                    Operator entrypoint
internal/
  controller/           Reconcilers, mappers, and controller tests
  credentials/          Credential loading and rotation
  xcclient/             F5 XC API client
  xcclientset/          Typed client wrapper for all resource types
config/
  crd/bases/            Generated CRD manifests
  rbac/                 Generated RBAC manifests
  samples/              Example CRs for each resource type
charts/
  f5xc-k8s-operator/    Helm chart (templates, values, CRDs)
```

## Adding or Modifying a CRD

1. Edit the type definition in `api/v1alpha1/`
2. Run `make generate` to update deepcopy methods
3. Run `make manifests` to regenerate CRD and RBAC YAML
4. Copy the updated CRD to the Helm chart: `cp config/crd/bases/*.yaml charts/f5xc-k8s-operator/crds/`
5. Update the mapper in `internal/controller/` if spec fields changed
6. Update tests and run `make test`

## Building the Container Image

```bash
docker build -t f5xc-k8s-operator:latest .
```

Or cross-compile without Docker:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o manager ./cmd/
```
