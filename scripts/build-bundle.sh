#!/usr/bin/env bash

SHORT_GIT_SHA=${CIRCLE_SHA1:0:6}
RELEASE_TAG="${CIRCLE_TAG:-$SHORT_GIT_SHA}"

mkdir bundle
pushd bundle
mkdir -p filecoin

# binary
cp ../venus filecoin/
chmod +x filecoin/venus

# proof params data
cp ../extern/go-sectorbuilder/paramcache filecoin/
chmod +x filecoin/paramcache

tar -zcvf "filecoin-$RELEASE_TAG-`uname`.tar.gz" filecoin
rm -rf filecoin

popd
