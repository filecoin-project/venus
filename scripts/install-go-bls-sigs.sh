#!/usr/bin/env bash

set -Eeo pipefail

subm_dir="vendors/go-bls-sigs"

git submodule update --init --recursive $subm_dir

(cd ${subm_dir} ; make ;)