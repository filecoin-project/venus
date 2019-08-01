#!/usr/bin/env bash

set -Eeo pipefail

subm_dir="go-bls-sigs"

git submodule update --init --recursive $subm_dir

(cd ./go-bls-sigs && ./install-bls-signatures)
