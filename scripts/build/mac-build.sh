#!/usr/bin/env bash
set -Eeou pipefail

export CC=gcc

if [ -z "$GOPATH" ]; then
  echo 'no GOPATH set using ~'
  export GOPATH=~
fi

base_build_path=${GOPATH}/src/github.com/filecoin-project
build_path=${base_build_path}/venus

mkdir -p ${base_build_path}
git clone https://github.com/filecoin-project/venus.git ${build_path}

if which brew; then
  echo 'HomeBrew found'
else
  echo "Homebrew not found. Installing Homebrew"
  ruby -e "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/master/install)"
  echo 'HomeBrew found'
fi

echo 'installing HomeBrew dependencies'
brew bundle --file=${build_path}/Brewfile --verbose

cd ${build_path} || exit 1
echo 'pulling down submodules'
git submodule update --init --recursive

echo 'build venus dependencies'
FILECOIN_USE_PRECOMPILED_RUST_PROOFS=true go run ${build_path}/build deps

go run ${build_path}/build build
