#!/bin/sh

go get -d github.com/ipfs/go-ipfs
cd "$GOPATH/src/github.com/ipfs/go-ipfs" && git checkout 8b383da27afc0a5ab14473a70a6b6a44e2dc2b72 && gx install
