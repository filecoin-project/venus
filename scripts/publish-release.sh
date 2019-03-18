#!/usr/bin/env bash

pushd bundle

RELEASE_TAG="${CIRCLE_TAG}"

# make sure we have a token set, api requests won't work otherwise
if [ -z $GITHUB_TOKEN ]; then
  echo "\$GITHUB_TOKEN not set, publish failed"
  exit 1
fi

#see if the release already exists by tag
RELEASE_RESPONSE=`
  curl \
    --header "Authorization: token $GITHUB_TOKEN" \
    "https://api.github.com/repos/$CIRCLE_PROJECT_USERNAME/$CIRCLE_PROJECT_REPONAME/releases/tags/$RELEASE_TAG"
`
RELEASE_ID=`echo $RELEASE_RESPONSE | jq '.id'`

if [ "$RELEASE_ID" = "null" ]; then
  echo "creating release"

  RELEASE_DATA="{
    \"tag_name\": \"$RELEASE_TAG\",
    \"target_commitish\": \"$CIRCLE_SHA1\",
    \"name\": \"$RELEASE_TAG\",
    \"body\": \"\"
  }"

  # create it if it doesn't exist yet
  RELEASE_RESPONSE=`
    curl \
        --request POST \
        --header "Authorization: token $GITHUB_TOKEN" \
        --header "Content-Type: application/json" \
        --data "$RELEASE_DATA" \
        "https://api.github.com/repos/$CIRCLE_PROJECT_USERNAME/$CIRCLE_PROJECT_REPONAME/releases"
  `
else
  echo "release already exists"
fi

RELEASE_UPLOAD_URL=`echo $RELEASE_RESPONSE | jq -r '.upload_url' | cut -d'{' -f1`

bundles=("filecoin-$RELEASE_TAG-Linux.tar.gz" "filecoin-$RELEASE_TAG-Darwin.tar.gz")
for RELEASE_FILE in "${bundles[@]}"
do
  echo "Uploading release bundle: $RELEASE_FILE"
  curl \
    --request POST \
    --header "Authorization: token $GITHUB_TOKEN" \
    --header "Content-Type: application/octet-stream" \
    --data-binary "@$RELEASE_FILE" \
    "$RELEASE_UPLOAD_URL?name=$(basename $RELEASE_FILE)"

  echo "Release bundle uploaded: $RELEASE_FILE "
done
