all: filecoinlocal filecoindocker

filecoinlocal:
	gx install
	(cd local/localfilecoin; go build -buildmode=plugin -o ../../localfilecoin.so)
CLEAN += localfilecoin.so

filecoindocker:
	gx install
	(cd docker/dockerfilecoin; go build -buildmode=plugin -o ../../dockerfilecoin.so)
CLEAN += dockerfilecoin.so

.PHONY: all filecoinlocal filecoindocker
