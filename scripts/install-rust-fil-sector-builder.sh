#!/usr/bin/env bash

set -Eeo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/install-shared.bash"

subm_dir="proofs/rust-fil-sector-builder"

git submodule update --init --recursive $subm_dir

if download_release_tarball tarball_path "${subm_dir}"; then
    tmp_dir=$(mktemp -d)
    tar -C $tmp_dir -xzf $tarball_path

    cp -R "${tmp_dir}/include" proofs
    cp -R "${tmp_dir}/lib" proofs
else
    echo "failed to find or obtain precompiled assets for ${subm_dir}, falling back to local build"
    build_from_source "${subm_dir}"

    mkdir -p proofs/include
    mkdir -p proofs/lib/pkgconfig

    find "${subm_dir}" -type f -name sector_builder_ffi.h -exec mv -- "{}" ./proofs/include/ \;
    find "${subm_dir}" -type f -name libsector_builder_ffi.a -exec cp -- "{}" ./proofs/lib/ \;
    find "${subm_dir}" -type f -name sector_builder_ffi.pc -exec cp -- "{}" ./proofs/lib/pkgconfig/ \;
fi
