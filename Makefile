APP_NAME ?= nctl

# try getconf (linux / macos), getconf (BSD), nproc, then fallback to 1
NPROCS := $(shell getconf _NPROCESSORS_ONLN 2>/dev/null || getconf NPROCESSORS_ONLN 2>/dev/null || nproc 2>/dev/null || echo 1)
MAKEFLAGS += --jobs=$(NPROCS)

.PHONY: all build completions test clean lint update help

# Shells for which completion scripts are generated into completions/.
COMPLETION_SHELLS ?= bash zsh fish

all: build

build:
	GITHUB_REPOSITORY=ninech/nctl goreleaser build --clean --snapshot --single-target

# The generated init code embeds the absolute path of the binary. Rewrite it to the
# bare binary name so the completion resolves $(APP_NAME) via PATH instead.
completions:
	@tmp="$$(mktemp -d)"; trap 'rm -rf "$$tmp"' EXIT; \
	go build -o "$$tmp/$(APP_NAME)" . && \
	for shell in $(COMPLETION_SHELLS); do \
		"$$tmp/$(APP_NAME)" completions -c "$$shell" \
			| sed "s|$$tmp/$(APP_NAME)|$(APP_NAME)|g" \
			> "completions/$(APP_NAME).$$shell"; \
	done

test:
	go test -race ./...

lint: mod-tidy vet staticcheck golangci-lint modernize govulncheck

lint-fix:
	go mod tidy
	golangci-lint run --fix
	go fix ./...
	$(MAKE) lint

mod-tidy:
	go mod tidy -diff

vet:
	go vet ./...

golangci-lint:
	golangci-lint run

staticcheck:
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

modernize:
	go fix -diff ./... | awk '{print} /\S/ {found=1} END {if (found) exit 1}'

govulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

update:
	go get -v -u ./... && go mod tidy

clean:
	rm -rf dist/

help:
	@echo "make             # Build $(APP_NAME)"
	@echo "make completions # Regenerate the shell completions in completions/"
	@echo "make test        # Run tests"
	@echo "make lint-fix    # Run linters and try fix issues"
	@echo "make lint        # Run linters"
	@echo "make update      # Update dependencies"
	@echo "make clean       # Remove built app"
