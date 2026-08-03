BINARY   ?= phi
MAIN_SRC  = ./cmd/main.go

GOBIN    ?= $(shell go env GOBIN)
GOPATH   ?= $(shell go env GOPATH)
ifeq ($(GOBIN),)
GOBIN     = $(GOPATH)/bin
endif

GO       ?= go
GOFLAGS  ?= -ldflags="-s -w"
CGO      ?= 0

.PHONY: all build install run clean test lint help

all: build

build:
	CGO_ENABLED=$(CGO) $(GO) build $(GOFLAGS) -o $(BINARY) $(MAIN_SRC)

install: build
	@mkdir -p $(GOBIN)
	mv $(BINARY) $(GOBIN)/$(BINARY)
	@echo "installed $(BINARY) -> $(GOBIN)/$(BINARY)"

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)
	$(GO) clean

test:
	$(GO) test ./...

lint:
	@which golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

help:
	@echo "Usage:"
	@echo "  make          - build binary ($(BINARY))"
	@echo "  make install  - build & install to \$$GOBIN ($(GOBIN))"
	@echo "  make run      - build & run"
	@echo "  make clean    - remove binary & cache"
	@echo "  make test     - run all tests"
	@echo "  make lint     - run golangci-lint"
