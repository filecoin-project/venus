#!/bin/sh

mkdir -p filecoin
cd filecoin

# library files
mkdir lib
cp ../proofs/rust-proofs/target/release/deps/libfilecoin_proofs.dylib lib/
cp ../proofs/rust-proofs/target/release/deps/libsector_base.dylib lib/

# binary
cp ../go-filecoin .
chmod +x go-filecoin

# update library links in the binary
install_name_tool -change $GOPATH/src/github.com/filecoin-project/go-filecoin/proofs/rust-proofs/target/release/deps/libfilecoin_proofs.dylib  @executable_path/lib/libfilecoin_proofs.dylib go-filecoin
install_name_tool -change $GOPATH/src/github.com/filecoin-project/go-filecoin/proofs/rust-proofs/target/release/deps/libsector_base.dylib  @executable_path/lib/libsector_base.dylib go-filecoin

# fixture data
mkdir fixtures
cp ../fixtures/*.key ./fixtures/
cp ../fixtures/*.car ./fixtures/
cp ../fixtures/*.json ./fixtures/
cp ../fixtures/*.toml ./fixtures/

cd ..
