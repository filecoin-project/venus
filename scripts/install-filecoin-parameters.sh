#!/usr/bin/env bash

set -Eeo pipefail

fetch_params() {
  ./proofs/bin/paramfetch -z 1024 --verbose --json=./proofs/misc/parameters.json || true
}

generate_params() {
  RUST_LOG=info ./proofs/bin/paramcache --test-only
}

fetch_params
generate_params
