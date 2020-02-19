#!/usr/bin/env bash

set -Eeo pipefail

subm_dir="vendors/filecoin-ffi"

git submodule update --init --recursive $subm_dir

(cd ${subm_dir} ; ./install-filecoin ;)
