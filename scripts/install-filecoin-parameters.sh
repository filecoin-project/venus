#!/usr/bin/env bash

fetch_params() {
  ./proofs/bin/paramfetch --all --verbose --json=./proofs/misc/parameters.json
}

generate_params() {
  ./proofs/bin/paramcache
}

if ! fetch_params; then
  generate_params
fi
