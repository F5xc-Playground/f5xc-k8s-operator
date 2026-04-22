.PHONY: test test-contract lint fmt vet generate manifests helm-sync install

CONTROLLER_GEN ?= $(shell which controller-gen)
ENVTEST ?= $(shell which setup-envtest)

ENVTEST_ASSETS_DIR ?= $(shell $(ENVTEST) use -p path 2>/dev/null)

CHART_DIR := charts/f5xc-k8s-operator

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

manifests: helm-sync
	$(CONTROLLER_GEN) crd rbac:roleName=manager-role paths="./..." output:crd:dir=config/crd/bases output:rbac:dir=config/rbac
	@cp config/crd/bases/*.yaml $(CHART_DIR)/crds/

helm-sync:
	@cp config/crd/bases/*.yaml $(CHART_DIR)/crds/ 2>/dev/null || true
	@cd $(CHART_DIR) && \
		CURRENT=$$(grep '^version:' Chart.yaml | awk '{print $$2}') && \
		MAJOR=$$(echo $$CURRENT | cut -d. -f1) && \
		MINOR=$$(echo $$CURRENT | cut -d. -f2) && \
		PATCH=$$(echo $$CURRENT | cut -d. -f3) && \
		NEW="$$MAJOR.$$MINOR.$$((PATCH + 1))" && \
		sed -i '' "s/^version: .*/version: $$NEW/" Chart.yaml && \
		sed -i '' "s/^appVersion: .*/appVersion: \"$$NEW\"/" Chart.yaml && \
		echo "Helm chart bumped $$CURRENT -> $$NEW"

install: manifests
	kubectl apply -f config/crd/bases/
