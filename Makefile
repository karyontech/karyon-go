GO := go
LINTER := golangci-lint
PKGS := ./...

.PHONY: all test lint clean

all: lint test

test:
	$(GO) test $(PKGS)

lint:
	$(LINTER) run $(PKGS)

clean:
	$(GO) clean

