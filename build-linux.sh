#!/bin/sh

mkdir -p filecoin

# binary
cp go-filecoin filecoin/
chmod +x filecoin/go-filecoin

# fixture data
mkdir filecoin/fixtures
cp fixtures/*.key filecoin/fixtures/
cp fixtures/*.car filecoin/fixtures/
cp fixtures/*.json filecoin/fixtures/
