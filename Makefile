.PHONY: test test-contract lint fmt vet generate manifests install

CONTROLLER_GEN ?= $(shell which controller-gen)
ENVTEST ?= $(shell which setup-envtest)

ENVTEST_ASSETS_DIR ?= $(shell $(ENVTEST) use -p path 2>/dev/null)

test:
	KUBEBUILDER_ASSETS="$(ENVTEST_ASSETS_DIR)" go test ./... -v -count=1

test-contract:
	go test ./... -v -count=1 -tags=contract

lint: fmt vet
	@echo "Lint passed"

fmt:
	gofmt -s -w .

vet:
	go vet ./...

generate:
	$(CONTROLLER_GEN) object paths="./api/..."

manifests:
	$(CONTROLLER_GEN) crd rbac:roleName=manager-role paths="./..." output:crd:dir=config/crd/bases output:rbac:dir=config/rbac

install: manifests
	kubectl apply -f config/crd/bases/
