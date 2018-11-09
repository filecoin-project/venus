#!/bin/sh

mkdir -p filecoin
cd filecoin

# library files
mkdir lib
cp ../proofs/rust-proofs/target/release/deps/libfilecoin_proofs.so lib/
cp ../proofs/rust-proofs/target/release/deps/libsector_base.so lib/

# binary
cp ../go-filecoin .
chmod +x go-filecoin

# fixture data
mkdir fixtures
cp ../fixtures/*.key ./fixtures/
cp ../fixtures/*.car ./fixtures/
cp ../fixtures/*.json ./fixtures/
cp ../fixtures/*.yaml ./fixtures/

cd ..
