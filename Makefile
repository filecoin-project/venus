-include .env

SHELL := /bin/bash

COMMIT := $(shell git log -n 1 --format=%H)
TARGET := $(shell echo $${PWD\#\#*/})
SRC := $(shell find . -type f -name '*.go' -not -path "./vendor/*")

LDFLAGS=-ldflags "-X=github.com/filecoin-project/go-filecoin/flags.Commit=$(COMMIT)"
GOCMD=GO111MODULE=on go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean

deps:
	pkg-config --version
	git submodule update --init --recursive
	go mod download
	./scripts/install-rust-fil-proofs.sh
	./scripts/install-go-bls-sigs.sh
	./scripts/install-go-sectorbuilder.sh
	./scripts/install-filecoin-parameters.sh

all: deps build-all install

$(TARGET): $(SRC)
	$(GOBUILD) $(LDFLAGS) -o $(TARGET)

build: $(TARGET)
	true

install: $(TARGET)
	cp $< $(GOPATH)/bin/go-filecoin

lint: $(TARGET)
	$(GOCMD) run github.com/golangci/golangci-lint/cmd/golangci-lint run

clean:
	$(GOCLEAN)
	rm -f $(TARGET)

test: test-unit

test-unit:
	$(GOTEST) ./... -unit=true -functional=false -integration=false

test-integration: $(TARGET)
	$(GOTEST) ./... -integration=true -unit=false -functional=false

test-functional: $(TARGET)
	$(GOTEST) ./... -functional=true -unit=false -integration=false

test-all: $(TARGET)
	$(GOTEST) ./... -functional=true -unit=true -integration=true

daemon-init: $(TARGET)
	./$< init

daemon-start: $(TARGET)
	./$< daemon
	
run-localnet: $(TARGET) build-localnet
	./tools/fast/bin/localnet/localnet

build-all: $(TARGET) build-gengen build-faucet build-genesis-file-server build-migrations build-prerelease-tool build-localnet generate-genesis-live

build-gengen:
	$(GOBUILD) -o ./gengen/gengen ./gengen

build-faucet:
	$(GOBUILD) -o ./tools/faucet/faucet ./tools/faucet/

build-genesis-file-server:
	$(GOBUILD) -o ./tools/genesis-file-server/genesis-file-server ./tools/genesis-file-server/

build-migrations:
	$(GOBUILD) -o ./tools/migration/go-filecoin-migrate ./tools/migration/main.go

build-prerelease-tool:
	$(GOBUILD) -o ./tools/prerelease-tool/prerelease-tool ./tools/prerelease-tool/

build-localnet: generate-genesis-test
	$(GOBUILD) -o ./tools/fast/bin/localnet/localnet ./tools/fast/bin/localnet/

generate-genesis-live:
	./gengen/gengen --keypath=./fixtures/live --out-car=./fixtures/live/genesis.car --out-json=./fixtures/live/gen.json --config=./fixtures/setup.json

generate-genesis-test:
	./gengen/gengen --keypath=./fixtures/test --out-car=./fixtures/test/genesis.car --out-json=./fixtures/test/gen.json --config=./fixtures/setup.json --test-proofs-mode

