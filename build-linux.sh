#!/bin/sh

mkdir -p filecoin/lib
cd filecoin
cp ../proofs/rust-proofs/target/release/deps/libfilecoin_proofs.so lib/
cp ../proofs/rust-proofs/target/release/deps/libsector_base.so lib/
cp ../go-filecoin .
chmod +x go-filecoin
cd ..
