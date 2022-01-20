export CGO_CFLAGS_ALLOW=-D__BLST_PORTABLE__
export CGO_CFLAGS=-D__BLST_PORTABLE__

all:
	go run ./build/*.go build

deps:
	git submodule update --init
	go run ./build/*.go smartdeps

lint:
	go run ./build/*.go lint

test: test-venus-shared
	go run ./build/*.go test -timeout=30m

# WARNING THIS BUILDS A GO PLUGIN AND PLUGINS *DO NOT* WORK ON WINDOWS SYSTEMS
iptb:
	make -C tools/iptb-plugins all

clean:
	rm ./venus

	rm -rf ./extern/filecoin-ffi
	rm -rf ./extern/test-vectors

gen-asset:
	go-bindata -pkg=asset -o ./fixtures/asset/asset.go ./fixtures/_assets/car/ ./fixtures/_assets/proof-params/ ./fixtures/_assets/arch-diagram.monopic
	gofmt -s -l -w ./fixtures/asset/asset.go

### devtool ###
cborgen:
	cd venus-devtool && go run ./cborgen/*.go

gogen:
	cd venus-shared && go generate ./...

mock-api-gen:
	cd ./venus-shared/api/chain/v0 && go run github.com/golang/mock/mockgen -destination=./mock/full.go -package=mock . FullNode
	cd ./venus-shared/api/chain/v1 && go run github.com/golang/mock/mockgen -destination=./mock/full.go -package=mock . FullNode

inline-gen:
	cd venus-devtool && go run ./inline-gen/main.go ../ ./inline-gen/inlinegen-data.json

test-venus-shared:
	cd venus-shared && go test -covermode=set ./...

gen-api:
	cd ./venus-devtool/ && go run ./api-gen/
	gofmt -s -l -w ./venus-shared/api/chain/v0/proxy_gen.go;
	gofmt -s -l -w ./venus-shared/api/chain/v1/proxy_gen.go

v0APIDoc = ../venus-shared/api/v0-api-document.md
v1APIDoc = ../venus-shared/api/v1-api-document.md
api-docs:
	cd ./venus-devtool/ && go run ./api-docs-gen/cmd ../venus-shared/api/chain/v0/fullnode.go FullNode v0 ../venus-shared/api/chain/v0 $(v0APIDoc)
	cd ./venus-devtool/ && go run ./api-docs-gen/cmd ../venus-shared/api/chain/v1/fullnode.go FullNode v1 ../venus-shared/api/chain/v1 $(v1APIDoc)

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
