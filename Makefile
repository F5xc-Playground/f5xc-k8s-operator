.PHONY: test test-contract lint fmt vet generate manifests install

CONTROLLER_GEN ?= $(shell which controller-gen)
ENVTEST ?= $(shell which setup-envtest)

test:
	go test ./... -v -count=1

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
