#!/bin/sh

mkdir -p filecoin/lib
cd filecoin
cp ../proofs/rust-proofs/target/release/deps/libfilecoin_proofs.dylib lib/
cp ../proofs/rust-proofs/target/release/deps/libsector_base.dylib lib/
cp ../go-filecoin .
chmod +x go-filecoin
install_name_tool -change $GOPATH/src/github.com/filecoin-project/go-filecoin/proofs/rust-proofs/target/release/deps/libfilecoin_proofs.dylib  @executable_path/lib/libfilecoin_proofs.dylib go-filecoin
install_name_tool -change $GOPATH/src/github.com/filecoin-project/go-filecoin/proofs/rust-proofs/target/release/deps/libsector_base.dylib  @executable_path/lib/libsector_base.dylib go-filecoin
cd ..
