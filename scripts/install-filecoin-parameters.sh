#!/usr/bin/env bash

set -Eeo pipefail

fetch_params() {
  ./proofs/bin/paramfetch -z 1024 --verbose --json=./proofs/misc/parameters.json || true
}

generate_params() {
  RUST_LOG=info ./proofs/bin/paramcache --test-only
}

# Disabling paramfetch until optimizations can be completed
# Fetching parameters, even small ones, has a long history of being slow and unreliable.
# It's disabled now in favour of local generation of the small parameter files needed
# for tests.
# On a 2015 MacBook pro, generation takes about a minute, one-time.
#
# fetch_params
generate_params
