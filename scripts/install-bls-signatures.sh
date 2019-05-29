#!/usr/bin/env bash

RELEASE_SHA1=`git rev-parse @:./bls-signatures/bls-signatures`

install_precompiled() {
  RELEASE_NAME="bls-signatures-`uname`"
  RELEASE_TAG="${RELEASE_SHA1:0:16}"

  RELEASE_RESPONSE=`curl \
    --retry 3 \
    --location \
    "https://api.github.com/repos/filecoin-project/bls-signatures/releases/tags/$RELEASE_TAG"
  `

  RELEASE_URL=`echo $RELEASE_RESPONSE | jq -r ".assets[] | select(.name | contains(\"$RELEASE_NAME\")) | .url"`

  ASSET_URL=`curl \
    --head \
    --retry 3 \
    --header "Accept:application/octet-stream" \
    --location \
    --output /dev/null \
    -w %{url_effective} \
    "$RELEASE_URL"
  `
  ASSET_ID=`basename ${RELEASE_URL}`

  TAR_NAME="${RELEASE_NAME}_${ASSET_ID}"
  if [ ! -f "/tmp/${TAR_NAME}.tar.gz" ]; then
      curl --retry 3 --output "/tmp/${TAR_NAME}.tar.gz" "$ASSET_URL"
      if [ $? -ne "0" ]; then
          echo "asset failed to be downloaded"
          return 1
      fi
  fi

  mkdir -p bls-signatures/include
  mkdir -p bls-signatures/lib/pkgconfig

  tar -C bls-signatures -xzf /tmp/${TAR_NAME}.tar.gz
}

install_local() {
  if ! [ -x "$(command -v cargo)" ] ; then
    echo 'Error: cargo is not installed.'
    echo 'Install Rust toolchain to resolve this problem.'
    exit 1
  fi

  if ! [ -x "$(command -v rustup)" ] ; then
    echo 'Error: rustup is not installed.'
    echo 'Install Rust toolchain installer to resolve this problem.'
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
  echo "using local bls-signatures @ ${RELEASE_SHA1}"
  install_local
else
  echo "using precompiled bls-signatures @ ${RELEASE_SHA1}"
  install_precompiled

  if [ $? -ne "0" ]; then
    echo "failed to find or obtain precompiled bls-signatures, falling back to local"
    install_local
  fi
fi
