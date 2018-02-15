#!/bin/sh

go get -d github.com/ipfs/go-ipfs
cd "$GOPATH/src/github.com/ipfs/go-ipfs" && gx install
