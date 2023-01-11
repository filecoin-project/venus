# Bundles

This directory includes the actors bundles for each release. Each actor bundle is a zstd compressed
tarfile containing one bundle per network type. These tarfiles are subsequently embedded in the
lotus binary.

## Updating

1. copy all files ending in `.tar.zst` from `https://github.com/filecoin-project/lotus/tree/master/build/actors`
2. `make bundle-gen`
