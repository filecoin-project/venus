#!/usr/bin/env bash

set -Eeo pipefail

generate_params() {
  # The `--test-only` flag will cause paramcache to generate Groth parameters
  # and verifying keys for 1KiB sectors. If Groth parameters or verifying keys
  # for other sector sizes are needed, remove the `--test-only` flag.
  RUST_LOG=info ./vendors/go-sectorbuilder/paramcache --test-only
}

generate_params
