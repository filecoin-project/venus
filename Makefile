all:
	go run ./build/*.go build

deps:
	go run ./build/*.go smartdeps

# WARNING THIS BUILDS A GO PLUGIN AND PLUGINS *DO NOT* WORK ON WINDOWS SYSTEMS
iptb:
	make -C tools/iptb-plugins all
