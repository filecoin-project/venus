#!/usr/bin/env bash

download_release_tarball() {
    __resultvar=$1
    __submodule_path=$2
    __repo_name=$(echo $2 | cut -d '/' -f 2)
    __release_name="${__repo_name}-$(uname)"
    __release_sha1=$(git rev-parse @:"${__submodule_path}")
    __release_tag="${__release_sha1:0:16}"
    __release_url="https://api.github.com/repos/filecoin-project/${__repo_name}/releases/tags/${__release_tag}"

    __release_response=$(curl \
 --retry 3 \
 --location $__release_url)

    __release_url=$(echo $__release_response | jq -r ".assets[] | select(.name | contains(\"${__release_name}\")) | .url")

    __asset_url=$(curl \
 --head \
 --retry 3 \
 --header "Accept:application/octet-stream" \
 --location \
 --output /dev/null \
 -w %{url_effective} \
 "$__release_url")

    __tar_path="/tmp/${__release_name}_$(basename ${__release_url}).tar.gz"

    if [[ ! -f "${__tar_path}" ]]; then
        curl --retry 3 --output "${__tar_path}" "$__asset_url"
        if [[ $? -ne "0" ]]; then
            echo "failed to download release asset (url: ${__release_url}, response: ${__release_response})"
            return 1
        fi
    fi

    eval $__resultvar="'$__tar_path'"
}

build_from_source() {
    __submodule_path=$1

    if ! [ -x "$(command -v cargo)" ]; then
        echo 'Error: cargo is not installed.'
        echo 'Install Rust toolchain to resolve this problem.'
        exit 1
    fi

    if ! [ -x "$(command -v rustup)" ]; then
        echo 'Error: rustup is not installed.'
        echo 'Install Rust toolchain installer to resolve this problem.'
        exit 1
    fi

    pushd $__submodule_path

    cargo --version
    cargo update
    cargo build --release --all

    popd
}
