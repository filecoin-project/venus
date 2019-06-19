#!/usr/bin/env bash

set -Eeo pipefail

fetch_params() {
  ./proofs/bin/paramfetch --all --verbose --json=./proofs/misc/parameters.json || true
}

generate_params() {
  ./proofs/bin/paramcache
}

fetch_params
generate_params
