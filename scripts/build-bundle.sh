#!/usr/bin/env bash

SHORT_GIT_SHA=${CIRCLE_SHA1:0:6}
RELEASE_TAG="${CIRCLE_TAG:-$SHORT_GIT_SHA}"

mkdir bundle
pushd bundle
mkdir -p filecoin

# binary
cp ../go-filecoin filecoin/
chmod +x filecoin/go-filecoin

# proof params data
cp ../vendors/go-sectorbuilder/paramcache filecoin/
chmod +x filecoin/paramcache

tar -zcvf "filecoin-$RELEASE_TAG-`uname`.tar.gz" filecoin
rm -rf filecoin

popd
