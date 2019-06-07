#!/usr/bin/env bash

RELEASE_SHA1=$(git rev-parse @:./proofs/rust-fil-sector-builder)

install_precompiled() {
  RELEASE_NAME="rust-fil-sector-builder-$(uname)"
  RELEASE_TAG="${RELEASE_SHA1:0:16}"

  RELEASE_RESPONSE=$(curl \
    --retry 3 \
    --location \
    "https://api.github.com/repos/filecoin-project/rust-fil-sector-builder/releases/tags/$RELEASE_TAG")

  RELEASE_URL=$(echo $RELEASE_RESPONSE | jq -r ".assets[] | select(.name | contains(\"$RELEASE_NAME\")) | .url")

  ASSET_URL=$(curl \
    --head \
    --retry 3 \
    --header "Accept:application/octet-stream" \
    --location \
    --output /dev/null \
    -w %{url_effective} \
    "$RELEASE_URL")

  ASSET_ID=$(basename ${RELEASE_URL})

  TAR_NAME="${RELEASE_NAME}_${ASSET_ID}"
  if [ ! -f "/tmp/${TAR_NAME}.tar.gz" ]; then
      curl --retry 3 --output "/tmp/${TAR_NAME}.tar.gz" "$ASSET_URL"
      if [ $? -ne "0" ]; then
          echo "asset failed to be downloaded"
          return 1
      fi
  fi

  tmp_dir=$(mktemp -d)
  tar -C $tmp_dir -xzf /tmp/${TAR_NAME}.tar.gz

  cp -R "${tmp_dir}/include" proofs
  cp -R "${tmp_dir}/lib" proofs
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

  pushd proofs/rust-fil-sector-builder

  cargo --version
  cargo update
  cargo build --release --all

  popd

  mkdir -p proofs/include
  mkdir -p proofs/lib/pkgconfig

  cp proofs/rust-fil-sector-builder/target/release/libsector_builder_ffi.h ./proofs/include/libsector_builder_ffi.h
  cp proofs/rust-fil-sector-builder/target/release/libsector_builder_ffi.a ./proofs/lib/libsector_builder_ffi.a
  cp proofs/rust-fil-sector-builder/target/release/libsector_builder_ffi.pc ./proofs/lib/pkgconfig/libsector_builder_ffi.pc
}

git submodule update --init --recursive proofs/rust-fil-sector-builder

echo "using precompiled rust-fil-sector-builder @ ${RELEASE_SHA1}"
install_precompiled

if [ $? -ne "0" ]; then
  echo "failed to find or obtain precompiled rust-fil-sector-builder, falling back to local"
  install_local
fi
