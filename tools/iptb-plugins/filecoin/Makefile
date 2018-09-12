all: filecoinlocal filecoindocker

filecoinlocal:
	gx-go rw
	(cd local/localfilecoin; go build -buildmode=plugin -o ../../localfilecoin.so)
	gx-go uw
CLEAN += localfilecoin.so

filecoindocker:
	gx-go rw
	(cd docker/dockerfilecoin; go build -buildmode=plugin -o ../../dockerfilecoin.so)
	gx-go uw
CLEAN += dockerfilecoin.so

.PHONY: all filecoinlocal filecoindocker
