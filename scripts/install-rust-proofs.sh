#!/usr/bin/env bash

install_precompiled() {
  RELEASE_SHA1=`git rev-parse @:./proofs/rust-proofs`
  RELEASE_NAME="rust-proofs-`uname`"
  RELEASE_TAG="${RELEASE_SHA1:0:16}"

  RELEASE_RESPONSE=`curl "https://api.github.com/repos/filecoin-project/rust-proofs/releases/tags/$RELEASE_TAG"`

  RELEASE_ID=`echo $RELEASE_RESPONSE | jq '.id'`

  if [ "$RELEASE_ID" == "null" ]; then
    echo "release ${RELEASE_TAG} does not exist, GitHub said ${RELEASE_RESPONSE}"
    echo "ensure your access token has full repo scope"
    return 1
  fi

  RELEASE_URL=`echo $RELEASE_RESPONSE | jq -r ".assets[] | select(.name | contains(\"$RELEASE_NAME\")) | .url"`


  ASSET_URL=`curl \
      --head \
      --header "Accept:application/octet-stream" \
      --location \
      --output /dev/null \
      -w %{url_effective} \
      "$RELEASE_URL"
  `
  ASSET_ID=`basename ${RELEASE_URL}`

  TAR_NAME="${RELEASE_NAME}_${ASSET_ID}"
  if [ ! -f "/tmp/${TAR_NAME}.tar.gz" ]; then
      curl --output "/tmp/${TAR_NAME}.tar.gz" "$ASSET_URL"
      if [ $? -ne "0" ]; then
          echo "asset failed to be downloaded"
          return 1
      fi
  fi

  mkdir -p proofs/bin
  mkdir -p proofs/include
  mkdir -p proofs/lib/pkgconfig
  mkdir -p proofs/misc

  tar -C proofs -xzf /tmp/${TAR_NAME}.tar.gz
}

install_local() {
  if ! [ -x "$(command -v cargo)" ] ; then
    echo 'Error: cargo is not installed.'
    echo 'Install Rust toolchain to resolve this problem.'
    exit 1
  fi

  git submodule update --init --recursive

  pushd proofs/rust-proofs

  cargo --version
  cargo update
  cargo build --release --all

  popd

  mkdir -p proofs/bin
  mkdir -p proofs/include
  mkdir -p proofs/lib/pkgconfig
  mkdir -p proofs/misc

  cp proofs/rust-proofs/parameters.json ./proofs/misc/
  cp proofs/rust-proofs/target/release/paramcache ./proofs/bin/
  cp proofs/rust-proofs/target/release/paramfetch ./proofs/bin/
  cp proofs/rust-proofs/target/release/libfilecoin_proofs.h ./proofs/include/
  cp proofs/rust-proofs/target/release/libfilecoin_proofs.a ./proofs/lib/
  cp proofs/rust-proofs/target/release/libfilecoin_proofs.pc ./proofs/lib/pkgconfig/
}

if [ -z "$FILECOIN_USE_PRECOMPILED_RUST_PROOFS" ]; then
  echo "using local rust-proofs"
  install_local
else
  echo "using precompiled rust-proofs"
  install_precompiled

  if [ $? -ne "0" ]; then
    echo "failed to find or obtain precompiled rust-proofs, falling back to local"
    install_local
  fi
fi
