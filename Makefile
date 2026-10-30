# Developer convenience only. Every target runs a documented command that
# can be called directly.

GO       ?= go
GOFMT    ?= gofmt
BIN      := bin/deskosctl
# The runtime companion: it reads the plan an image carries, so it is built here too
# and cannot rot unnoticed (docs/adr/0018).
BIN_RUNTIME := bin/deskos
# DeskOS Core is embedded in the binary; only an organization's resources are
# passed on the command line.
EXAMPLE   := ./examples/organization-and-role

.PHONY: build test test-evidence fmt fmt-check vet check validate links style plan-centos plan-example render-centos render-example sbom-example golden clean

build:
	$(GO) build -o $(BIN) ./cmd/deskosctl
	$(GO) build -o $(BIN_RUNTIME) ./cmd/deskos

test:
	$(GO) test ./...

test-evidence:
	python3 -m unittest discover -s tests/evidence

fmt:
	$(GOFMT) -w .

# Fails when gofmt itself cannot run, not only when files need formatting.
fmt-check:
	@out=$$($(GOFMT) -l .) || { echo "$(GOFMT) failed"; exit 1; }; \
	test -z "$$out" || { echo "$$out"; echo "run make fmt"; exit 1; }

vet:
	$(GO) vet ./...

check: fmt-check vet test test-evidence validate links style

# Relative Markdown links and anchors, so a rewrite cannot leave a dead path.
links:
	python3 tools/check-links.py

# The mechanical part of docs/style.md: em dashes and filler words in prose.
style:
	python3 tools/check-style.py

validate: build
	$(BIN) validate $(EXAMPLE)
	$(BIN) validate ./examples/centos-stream-10
	$(BIN) validate ./examples/almalinux-10
	$(BIN) validate ./examples/rhel-10
# Fedora 45 is a shipped platform (resources/platforms/); the example root
# references it by name like the other per-platform examples.
	$(BIN) validate tests/identity/resources

plan-centos: build
	$(BIN) plan --workstation deskos-core-centos-stream-10

plan-example: build
	$(BIN) plan $(EXAMPLE) --workstation example-devops-rhel-10

render-centos: build
	$(BIN) render --workstation deskos-core-centos-stream-10 --backend containerfile --output dist/deskos-core-centos-stream-10

render-example: build
	$(BIN) render $(EXAMPLE) --workstation example-devops-rhel-10 --backend containerfile --output dist/example-devops-rhel-10

sbom-example: build
	mkdir -p dist
	$(BIN) sbom $(EXAMPLE) --workstation example-devops-rhel-10 --output dist/example-devops-rhel-10.sbom.cdx.json

golden:
	$(GO) test ./internal/compiler -update

clean:
	rm -rf bin dist
