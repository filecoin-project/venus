all:
	go run ./build/*.go build

deps:
	go run ./build/*.go smartdeps
