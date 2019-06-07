#!/usr/bin/env bash

source "$(dirname "${BASH_SOURCE[0]}")/install-shared.bash"

subm_dir="bls-signatures/bls-signatures"

git submodule update --init --recursive $subm_dir

if download_release_tarball tarball_path "${subm_dir}"; then
    tmp_dir=$(mktemp -d)
    tar -C $tmp_dir -xzf $tarball_path

    cp -R "${tmp_dir}/include" bls-signatures
    cp -R "${tmp_dir}/lib" bls-signatures
else
    (>&2 echo "failed to find or obtain precompiled assets for ${subm_dir}, falling back to local build")
    build_from_source "${subm_dir}"

    mkdir -p bls-signatures/include
    mkdir -p bls-signatures/lib/pkgconfig

    cp "${subm_dir}/target/release/libbls_signatures.h" ./bls-signatures/include/libbls_signatures.h
    cp "${subm_dir}/target/release/libbls_signatures_ffi.a" ./bls-signatures/lib/libbls_signatures.a
    cp "${subm_dir}/target/release/libbls_signatures.pc" ./bls-signatures/lib/pkgconfig/libbls_signatures.pc
fi
