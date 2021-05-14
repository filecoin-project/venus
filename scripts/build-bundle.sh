#!/usr/bin/env bash

SHORT_GIT_SHA=${CIRCLE_SHA1:0:6}
RELEASE_TAG="${CIRCLE_TAG:-$SHORT_GIT_SHA}"

mkdir bundle
pushd bundle
mkdir -p filecoin

# binary
cp ../venus filecoin/
chmod +x filecoin/venus

popd
