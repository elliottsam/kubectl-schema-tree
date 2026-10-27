SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -c

BINARY   := kubectl-schema_tree
PKG      := github.com/elliottsam/kubectl-schema-tree
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COVER_MIN ?= 85
CONTEXT  ?= kind-schematree

.PHONY: build test test-integration cover golden-update lint fuzz bench fixtures kind-up kind-down tidy

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY) ./cmd/$(BINARY)

test:
	go test -race -shuffle=on -cover ./...

# Enforces the per-package coverage floor for every tested package under internal/
# (test-helper packages without tests, such as schematest, are not gated).
cover:
	go test -race -shuffle=on -cover ./internal/... | tee cover.txt
	@awk '/^ok/ && /coverage:/ { gsub("%","",$$5); if ($$5+0 < $(COVER_MIN)) { print "FAIL: " $$2 " coverage " $$5 "% < $(COVER_MIN)%"; bad=1 } } END { exit bad }' cover.txt

# Needs a reachable cluster; see kind-up.
test-integration:
	KUBECTL_SCHEMATREE_CONTEXT=$(CONTEXT) go test -race -tags integration -count=1 ./internal/integration/...

# Rewrites golden files. Review the diff before committing.
golden-update:
	go test ./internal/tui -update

lint:
	golangci-lint run

fuzz:
	go test ./internal/schema -run '^$$' -fuzz FuzzLoadDocument -fuzztime 30s

bench:
	go test ./internal/... -run '^$$' -bench . -benchmem

# Refreshes trimmed OpenAPI fixtures from the current cluster context.
fixtures:
	go run hack/fixtures/main.go --context $(CONTEXT)

kind-up:
	kind create cluster --name schematree --wait 120s

kind-down:
	kind delete cluster --name schematree

tidy:
	go mod tidy
