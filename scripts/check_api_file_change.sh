#!/bin/bash

change=$(git status | grep full.go);
change2=$(git status | grep stable_method_info.json);

echo "$change"
echo "$change2"
echo ""

if [[ "$change" = "" ]]
then
  if [[ "$change2" == "" ]]
    then
      # api file not change, eixt
      echo "full.go not change"
      echo "stable_method_info.json not change"
      exit 0
    else
      echo "$change2"
      exit 1
  fi
else
  echo "$change"
  exit 1
fi
