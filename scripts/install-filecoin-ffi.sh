#!/usr/bin/env bash

set -Eeo pipefail

subm_dir="extern/filecoin-ffi"

git submodule update --init --recursive $subm_dir

(cd ${subm_dir} ; ./install-filcrypto ;)
