.PHONY: test test-contract lint fmt vet

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
