export GO111MODULE=on

all: filecoinlocal filecoindocker

filecoinlocal:
	(cd local/localfilecoin; go build -buildmode=plugin -o ../../localfilecoin.so)
CLEAN += localfilecoin.so

filecoindocker:
	(cd docker/dockerfilecoin; go build -buildmode=plugin -o ../../dockerfilecoin.so)
CLEAN += dockerfilecoin.so

.PHONY: all filecoinlocal filecoindocker
