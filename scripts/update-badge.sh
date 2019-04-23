#!/usr/bin/env bash
# this script updates go-filecoin status badges in https://github.com/filecoin-project/go-filecoin-badges
# with latest binary release version
set -e
FILENAME="${1}"
if [[ -z "${FILENAME}" ]]; then
	echo "filename argument must be provided"
	exit 1
fi
git config --global user.email dev-helper@filecoin.io
git config --global user.name filecoin-helper
git clone "https://${GITHUB_TOKEN}@github.com/filecoin-project/go-filecoin-badges.git"
cd go-filecoin-badges
jq --arg FILECOIN_BINARY_VERSION "${FILECOIN_BINARY_VERSION}" -r '.message = $FILECOIN_BINARY_VERSION' < "${FILENAME}" | tee "${FILENAME}.new"
if [[ ! -s ${FILENAME}.new ]]; then
	echo "badge update was not successful. file is empty"
	exit 1
fi
mv "${FILENAME}.new" "${FILENAME}"
git add "${FILENAME}"
git commit -m "badge update bot: update ${FILENAME} to ${FILECOIN_BINARY_VERSION}"
git push "https://${GITHUB_TOKEN}@github.com/filecoin-project/go-filecoin-badges.git"
