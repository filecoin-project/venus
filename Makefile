export CGO_CFLAGS_ALLOW=-D__BLST_PORTABLE__
export CGO_CFLAGS=-D__BLST_PORTABLE__

all: build
.PHONY: all

## variables

# git modules that need to be loaded
MODULES:=

ldflags=-X=github.com/filecoin-project/venus/pkg/constants.CurrentCommit=+git.$(subst -,.,$(shell git describe --always --match=NeVeRmAtCh --dirty 2>/dev/null || git rev-parse --short HEAD 2>/dev/null))
ifneq ($(strip $(LDFLAGS)),)
	    ldflags+=-extldflags=$(LDFLAGS)
	endif

GOFLAGS+=-ldflags="$(ldflags)"

## FFI

FFI_PATH:=extern/filecoin-ffi/
FFI_DEPS:=.install-filcrypto
FFI_DEPS:=$(addprefix $(FFI_PATH),$(FFI_DEPS))

$(FFI_DEPS): build-dep/.filecoin-install ;

build-dep/.filecoin-install: $(FFI_PATH)
	$(MAKE) -C $(FFI_PATH) $(FFI_DEPS:$(FFI_PATH)%=%)
	@touch $@

MODULES+=$(FFI_PATH)
BUILD_DEPS+=build-dep/.filecoin-install
CLEAN+=build-dep/.filecoin-install

## modules
build-dep:
	mkdir $@

$(MODULES): build-dep/.update-modules;
# dummy file that marks the last time modules were updated
build-dep/.update-modules: build-dep;
	git submodule update --init --recursive
	touch $@

gen-all: cborgen gogen inline-gen api-gen

gen-asset:
	go-bindata -pkg=asset -o ./fixtures/asset/asset.go ./fixtures/_assets/car/ ./fixtures/_assets/proof-params/ ./fixtures/_assets/arch-diagram.monopic
	gofmt -s -l -w ./fixtures/asset/asset.go

### devtool ###
cborgen:
	cd venus-devtool && go run ./cborgen/*.go

gogen:
	cd venus-shared && go generate ./...

inline-gen:
	cd venus-devtool && go run ./inline-gen/main.go ../ ./inline-gen/inlinegen-data.json

test-venus-shared:
	cd venus-shared && go test -covermode=set ./...

api-gen:
	cd ./venus-devtool/ && go run ./api-gen/ proxy
	cd ./venus-devtool/ && go run ./api-gen/ client
	cd ./venus-devtool/ && go run ./api-gen/ doc
	cd ./venus-devtool/ && go run ./api-gen/ mock

compatible-all: compatible-api compatible-actor

compatible-api: api-checksum api-diff api-perm

api-checksum:
	cd venus-devtool && go run ./compatible/apis/*.go checksum > ../venus-shared/compatible-checks/api-checksum.txt

api-diff:
	cd venus-devtool && go run ./compatible/apis/*.go diff > ../venus-shared/compatible-checks/api-diff.txt

api-perm:
	cd venus-devtool && go run ./compatible/apis/*.go perm > ../venus-shared/compatible-checks/api-perm.txt

compatible-actor: actor-templates actor-sources actor-render

actor-templates:
	cd venus-devtool && go run ./compatible/actors/*.go templates --dst ../venus-shared/actors/ > ../venus-shared/compatible-checks/actor-templates.txt

actor-sources:
	cd venus-devtool && go run ./compatible/actors/*.go sources > ../venus-shared/compatible-checks/actor-sources.txt

actor-render:
	cd venus-devtool && go run ./compatible/actors/*.go render ../venus-shared/actors/

actor-replica:
	cd venus-devtool && go run ./compatible/actors/*.go replica --dst ../venus-shared/actors/

test:
	go build -o genesis-file-server ./tools/genesis-file-server
	go build -o gengen ./tools/gengen
	./gengen --keypath ./fixtures/live --out-car ./fixtures/live/genesis.car --out-json  ./fixtures/live/gen.json --config ./fixtures/setup.json
	./gengen --keypath ./fixtures/test --out-car ./fixtures/test/genesis.car --out-json  ./fixtures/test/gen.json --config ./fixtures/setup.json
	 go test  -v ./... -integration=true -unit=false

lint: $(BUILD_DEPS)
	staticcheck ./...

deps: $(BUILD_DEPS)

dist-clean:
	git clean -xdff
	git submodule deinit --all -f

build: $(BUILD_DEPS)
	rm -f venus
	go build -o ./venus $(GOFLAGS) .