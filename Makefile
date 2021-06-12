export CGO_CFLAGS_ALLOW=-D__BLST_PORTABLE__
export CGO_CFLAGS=-D__BLST_PORTABLE__

all:
	go run ./build/*.go build

deps:
	git submodule update --init
	go run ./build/*.go smartdeps

# WARNING THIS BUILDS A GO PLUGIN AND PLUGINS *DO NOT* WORK ON WINDOWS SYSTEMS
iptb:
	make -C tools/iptb-plugins all

clean:
	rm ./venus

	rm -rf ./extern/filecoin-ffi
	rm -rf ./extern/test-vectors

gen:
	go run ./tools/gen/api/proxygen.go
	gofmt -s -l -w ./app/client/client_gen.go
	goimports -l -w ./app/client/client_gen.go

lint:
	go run ./build/*.go lint

test:
	go run ./build/*.go test --timeout=30m