export CGO_CFLAGS_ALLOW=-D__BLST_PORTABLE__
export CGO_CFLAGS=-D__BLST_PORTABLE__

all:
	go run ./build/*.go build

deps:
	git submodule update --init
	go run ./build/*.go smartdeps

lint:
	go run ./build/*.go lint

test:
	go run ./build/*.go test -timeout=30m

# WARNING THIS BUILDS A GO PLUGIN AND PLUGINS *DO NOT* WORK ON WINDOWS SYSTEMS
iptb:
	make -C tools/iptb-plugins all

clean:
	rm ./venus

	rm -rf ./extern/filecoin-ffi
	rm -rf ./extern/test-vectors

gen-api:
	go run ./tools/gen/api/proxygen.go
	gofmt -s -l -w ./app/client/full.go
	gofmt -s -l -w ./app/client/v0api/full.go

v2-gen-api:
	cd ./venus-devtool/ && go run ./api-gen/
	gofmt -s -l -w ./venus-shared/api/chain/v0/proxy_gen.go;
	gofmt -s -l -w ./venus-shared/api/chain/v1/proxy_gen.go

docfile = ./venus-shared/api/api-document.md
api-docs:
	cd ./venus-devtool/ && \
		go run ./api-docs-gen/cmd \
		../venus-shared/api/chain/v1/fullnode.go \
		FullNode v1 \
		../venus-shared/api/chain/v1 > \
		.$(docfile)
	echo json-rpc-doc-file : $(docfile)

compare-api:
	go run ./tools/gen/api/proxygen.go compare

gen-asset:
	go-bindata -pkg=asset -o ./fixtures/asset/asset.go ./fixtures/_assets/car/ ./fixtures/_assets/proof-params/ ./fixtures/_assets/arch-diagram.monopic


### shared module ###
cborgen:
	cd venus-devtool && go run ./cborgen/*.go

gogen:
	cd venus-shared && go generate ./...

test-venus-shared:
	cd venus-shared && go test -covermode=set ./...

test: gogen test-venus-shared

compatible-all: compatible-api compatible-actor

compatible-api: api-checksum api-diff

api-checksum:
	cd venus-devtool && go run ./compatible/apis/*.go checksum > ../venus-shared/compatible-checks/api-checksum.txt

api-diff:
	cd venus-devtool && go run ./compatible/apis/*.go diff > ../venus-shared/compatible-checks/api-diff.txt

compatible-actor: actor-templates actor-sources actor-render

actor-templates:
	cd venus-devtool && go run ./compatible/actors/*.go templates --dst ../venus-shared/actors/ > ../venus-shared/compatible-checks/actor-templates.txt

actor-sources:
	cd venus-devtool && go run ./compatible/actors/*.go sources > ../venus-shared/compatible-checks/actor-sources.txt

actor-render:
	cd venus-devtool && go run ./compatible/actors/*.go render ../venus-shared/actors/

actor-replica:
	cd venus-devtool && go run ./compatible/actors/*.go replica --dst ../venus-shared/actors/
