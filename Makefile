export CGO_CFLAGS_ALLOW=-D__BLST_PORTABLE__
export CGO_CFLAGS=-D__BLST_PORTABLE__

GO?=go

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

gen-all: cborgen gogen inline-gen api-gen bundle-gen state-type-gen

### devtool ###
cborgen:
	cd venus-devtool && $(GO) run ./cborgen/*.go

gogen:
	cd venus-shared && $(GO) generate ./...

inline-gen:
	cd venus-devtool && $(GO) run ./inline-gen/main.go ../ ./inline-gen/inlinegen-data.json

test-venus-shared:
	cd venus-shared && $(GO) test -covermode=set ./...

bundle-gen:
	cd venus-devtool && $(GO) run ./bundle-gen/*.go  --dst ./../venus-shared/actors/builtin_actors_gen.go

state-type-gen:
	cd venus-devtool && $(GO) run ./state-type-gen/*.go --dst ./../venus-shared/types

api-gen:
	find ./venus-shared/api/ -name 'client_gen.go' -delete
	find ./venus-shared/api/ -name 'proxy_gen.go' -delete
	cd ./venus-devtool/ && $(GO) run ./api-gen/ proxy
	cd ./venus-devtool/ && $(GO) run ./api-gen/ client
	cd ./venus-devtool/ && $(GO) run ./api-gen/ doc
	cd ./venus-devtool/ && $(GO) run ./api-gen/ mock

compatible-all: compatible-api compatible-actor

compatible-api: api-checksum api-diff api-perm

api-checksum:
	cd venus-devtool && $(GO) run ./compatible/apis/*.go checksum > ../venus-shared/compatible-checks/api-checksum.txt

api-diff:
	cd venus-devtool && $(GO) run ./compatible/apis/*.go diff > ../venus-shared/compatible-checks/api-diff.txt

api-perm:
	cd venus-devtool && $(GO) run ./compatible/apis/*.go perm > ../venus-shared/compatible-checks/api-perm.txt

compatible-actor: actor-templates actor-sources actor-render actor-replica

actor-templates:
	cd venus-devtool && $(GO) run ./compatible/actors/*.go templates --dst ../venus-shared/actors/ > ../venus-shared/compatible-checks/actor-templates.txt

actor-sources:
	cd venus-devtool && $(GO) run ./compatible/actors/*.go sources > ../venus-shared/compatible-checks/actor-sources.txt

actor-render:
	cd venus-devtool && $(GO) run ./compatible/actors/*.go render ../venus-shared/actors/

actor-replica:
	cd venus-devtool && $(GO) run ./compatible/actors/*.go replica --dst ../venus-shared/actors/

test:test-venus-shared
	$(GO) build -o genesis-file-server ./tools/genesis-file-server
	$(GO) build -o gengen ./tools/gengen
	./gengen --keypath ./fixtures/live --out-car ./fixtures/live/genesis.car --out-json  ./fixtures/live/gen.json --config ./fixtures/setup.json
	./gengen --keypath ./fixtures/test --out-car ./fixtures/test/genesis.car --out-json  ./fixtures/test/gen.json --config ./fixtures/setup.json
	$(GO) test $$(go list ./... | grep -v /venus-shared/) -timeout=30m -v -integration=true -unit=false
	$(GO) test $$(go list ./... | grep -v /venus-shared/) -timeout=30m -v -integration=false -unit=true

lint: $(BUILD_DEPS)
	golangci-lint run

deps: $(BUILD_DEPS)

dist-clean:
	git clean -xdff
	git submodule deinit --all -f

build: $(BUILD_DEPS)
	rm -f venus
	$(GO) build -o ./venus $(GOFLAGS) .


.PHONY: docker
TAG:=test
docker: $(BUILD_DEPS)
ifdef DOCKERFILE
	cp $(DOCKERFILE) ./dockerfile
else
	curl -o dockerfile https://raw.githubusercontent.com/filecoin-project/venus-docs/master/script/docker/dockerfile
endif

	docker build --build-arg https_proxy=$(BUILD_DOCKER_PROXY) --build-arg BUILD_TARGET=venus -t venus  .
	docker tag venus:latest filvenus/venus:$(TAG)

ifdef PRIVATE_REGISTRY
	docker tag venus:latest $(PRIVATE_REGISTRY)/filvenus/venus:$(TAG)
endif


docker-push: docker
	docker push $(PRIVATE_REGISTRY)/filvenus/venus:$(TAG)
