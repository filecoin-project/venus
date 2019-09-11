#!/usr/bin/env bash

set -Eeo pipefail

subm_dir="go-sectorbuilder"

git submodule update --init --recursive $subm_dir

(cd ${subm_dir} ; make clean ; make)
