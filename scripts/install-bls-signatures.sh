#!/usr/bin/env bash

install_precompiled() {
  echo "precompiled bls-signatures not supported yet"
  return 1
}

install_local() {
  if ! [ -x "$(command -v cargo)" ] ; then
    echo 'Error: cargo is not installed.'
    echo 'Install Rust toolchain to resolve this problem.'
    exit 1
  fi

  git submodule update --init --recursive bls-signatures/bls-signatures

  pushd bls-signatures/bls-signatures

  cargo --version
  cargo update
  cargo build --release --all

  popd

  mkdir -p bls-signatures/include
  mkdir -p bls-signatures/lib/pkgconfig

  cp bls-signatures/bls-signatures/target/release/libbls_signatures.h ./bls-signatures/include/
  cp bls-signatures/bls-signatures/target/release/libbls_signatures_ffi.a ./bls-signatures/lib/libbls_signatures.a
  cp bls-signatures/bls-signatures/target/release/libbls_signatures.pc ./bls-signatures/lib/pkgconfig/
}

if [ -z "$FILECOIN_USE_PRECOMPILED_BLS_SIGNATURES" ]; then
  echo "using local bls-signatures"
  install_local
else
  echo "using precompiled bls-signatures"
  install_precompiled

  if [ $? -ne "0" ]; then
    echo "failed to find or obtain precompiled bls-signatures, falling back to local"
    install_local
  fi
fi
