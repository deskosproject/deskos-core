# Developer convenience only. Every target runs a documented command that
# can be called directly.

GO       ?= go
GOFMT    ?= gofmt
BIN      := bin/deskosctl
CORE     := ./resources
EXAMPLE   := ./examples/baseline-and-role

.PHONY: build test fmt fmt-check vet check validate plan-centos plan-example render-centos render-example sbom-example golden clean

build:
	$(GO) build -o $(BIN) ./cmd/deskosctl

test:
	$(GO) test ./...

fmt:
	$(GOFMT) -w .

# Fails when gofmt itself cannot run, not only when files need formatting.
fmt-check:
	@out=$$($(GOFMT) -l .) || { echo "$(GOFMT) failed"; exit 1; }; \
	test -z "$$out" || { echo "$$out"; echo "run make fmt"; exit 1; }

vet:
	$(GO) vet ./...

check: fmt-check vet test validate

validate: build
	$(BIN) validate $(CORE) $(EXAMPLE)
	$(BIN) validate $(CORE) ./examples/core-centos-stream-10
	$(BIN) validate $(CORE) ./examples/core-almalinux-10
	$(BIN) validate $(CORE) ./examples/core-rhel-10

plan-centos: build
	$(BIN) plan $(CORE) --workstation deskos-core-centos10

plan-example: build
	$(BIN) plan $(CORE) $(EXAMPLE) --workstation example-devops-rhel10

render-centos: build
	$(BIN) render $(CORE) --workstation deskos-core-centos10 --backend containerfile --output dist/deskos-core-centos10

render-example: build
	$(BIN) render $(CORE) $(EXAMPLE) --workstation example-devops-rhel10 --backend containerfile --output dist/example-devops-rhel10

sbom-example: build
	mkdir -p dist
	$(BIN) sbom $(CORE) $(EXAMPLE) --workstation example-devops-rhel10 --output dist/example-devops-rhel10.sbom.cdx.json

golden:
	$(GO) test ./internal/compiler -update

clean:
	rm -rf bin dist
