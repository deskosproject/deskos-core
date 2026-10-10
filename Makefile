# Developer convenience only. Every target runs a documented command that
# can be called directly.

GO       ?= go
GOFMT    ?= gofmt
BIN      := bin/deskosctl
# DeskOS Core is embedded in the binary; only an organization's resources are
# passed on the command line.
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
	$(BIN) validate $(EXAMPLE)
	$(BIN) validate ./examples/core-centos-stream-10
	$(BIN) validate ./examples/core-almalinux-10
	$(BIN) validate ./examples/core-rhel-10

plan-centos: build
	$(BIN) plan --workstation deskos-core-centos10

plan-example: build
	$(BIN) plan $(EXAMPLE) --workstation example-devops-rhel10

render-centos: build
	$(BIN) render --workstation deskos-core-centos10 --backend containerfile --output dist/deskos-core-centos10

render-example: build
	$(BIN) render $(EXAMPLE) --workstation example-devops-rhel10 --backend containerfile --output dist/example-devops-rhel10

sbom-example: build
	mkdir -p dist
	$(BIN) sbom $(EXAMPLE) --workstation example-devops-rhel10 --output dist/example-devops-rhel10.sbom.cdx.json

golden:
	$(GO) test ./internal/compiler -update

clean:
	rm -rf bin dist
