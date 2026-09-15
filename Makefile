GO        ?= go
PKGS      := $(shell $(GO) list ./... | grep -v /examples/)
COVER_OUT := coverage.out
ARTIFACTS := .artifacts

.PHONY: all build lint vuln test cover fuzz testca redaction-scan bench tools

all: lint vuln test cover

build:
	$(GO) build ./...

tools:
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	$(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	$(GO) install honnef.co/go/tools/cmd/staticcheck@latest
	$(GO) install github.com/securego/gosec/v2/cmd/gosec@latest

lint:
	$(GO) vet ./...
	golangci-lint run ./...

vuln:
	./scripts/vulncheck.sh

test:
	$(GO) test -race -count=1 ./...

fuzz:
	for f in FuzzParseSPIFFEID FuzzAuditEventUnmarshal FuzzPolicyLoad; do \
	  $(GO) test -run xxx -fuzz=$$f -fuzztime=30s ./tests/fuzz/ || exit 1; done

cover:
	$(GO) test -count=1 -coverprofile=$(COVER_OUT) -coverpkg=./... $(PKGS)
	./scripts/coverage-gate.sh $(COVER_OUT)

testca:
	$(GO) run ./cmd/freya-devca -out .dev/ca -trust-domain example.org -services orders,inventory,billing

redaction-scan:
	./scripts/redaction-scan.sh

bench:
	mkdir -p $(ARTIFACTS)
	$(GO) test -tags bench -run xxx -bench BenchmarkCall -benchtime 5s ./tests/integration/ | tee $(ARTIFACTS)/bench.txt
	./scripts/bench-gate.sh $(ARTIFACTS)/bench.txt
