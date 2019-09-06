include mk/golang.mk

TARGET := $(shell echo $${PWD\#\#*/})
SRC := $(shell find . -type f -name '*.go' -not -path "./vendor/*")

LDFLAGS=-ldflags "-X=github.com/filecoin-project/go-filecoin/flags.Commit=$(COMMIT)"

$(TARGET): $(SRC)
	@$(GOBUILD) $(LDFLAGS) -o $(TARGET)

build: $(TARGET)
	@true

all: deps build-all

deps:
	pkg-config --version
	git submodule update --init --recursive
	go mod download
	./scripts/install-rust-fil-proofs.sh
	./scripts/install-go-bls-sigs.sh
	./scripts/install-go-sectorbuilder.sh
	./scripts/install-filecoin-parameters.sh


build-all: build gengen faucet genesis-file-server go-filecoin-migrate prerelease-tool localnet

install: $(TARGET)
	@cp $< $(GOPATH)/bin/go-filecoin

lint: $(TARGET)
	@$(GOCMD) run github.com/golangci/golangci-lint/cmd/golangci-lint run

clean:
	@$(GOCLEAN)
	@rm -f $(TARGET)

test: test-unit test-integration

test-unit:
	$(GOTEST) ./... -unit=true -functional=false -integration=false

test-integration: $(TARGET)
	$(GOTEST) ./... -integration=true -unit=false -functional=false

test-functional: $(TARGET)
	$(GOTEST) ./... -functional=true -unit=false -integration=false

test-all: $(TARGET)
	$(GOTEST) ./... -functional=true -unit=true -integration=true

gengen:
	@$(MAKE) -C ./gengen

faucet:
	@$(MAKE) -C ./tools/faucet

genesis-file-server:
	@$(MAKE) -C ./tools/genesis-file-server

go-filecoin-migrate:
	@$(MAKE) -C ./tools/migration

prerelease-tool:
	@$(MAKE) -C ./tools/prerelease-tool

localnet:
	@$(MAKE) -C ./tools/fast/bin/localnet

genesis-live: gengen
	./gengen/gengen --keypath=./fixtures/live --out-car=./fixtures/live/genesis.car --out-json=./fixtures/live/gen.json --config=./fixtures/setup.json

genesis-test: gengen
	./gengen/gengen --keypath=./fixtures/test --out-car=./fixtures/test/genesis.car --out-json=./fixtures/test/gen.json --config=./fixtures/setup.json --test-proofs-mode

.PHONY: deps build build-all gengen faucet genesis-file-server go-filecoin-migrate prerelease-tool localnet genesis-live genesis-test
