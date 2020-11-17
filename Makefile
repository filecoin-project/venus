all:
	go run ./build/*.go build
	go run github.com/GeertJohan/go.rice/rice append --exec venus -i ./fixtures

deps:
	go run ./build/*.go smartdeps

# WARNING THIS BUILDS A GO PLUGIN AND PLUGINS *DO NOT* WORK ON WINDOWS SYSTEMS
iptb:
	make -C tools/iptb-plugins all
