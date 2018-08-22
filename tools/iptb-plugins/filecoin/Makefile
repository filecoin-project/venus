all: filecoinlocal

filecoinlocal:
	gx-go rw
	(cd local/localfilecoin; go build -buildmode=plugin -o ../../localfilecoin.so)
	gx-go uw
CLEAN += localfilecoin.so


.PHONY: all filecoinlocal
