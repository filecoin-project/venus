#!/bin/bash

# This script can be used to created asks on the filecoin network,
# it accepts 2 required arguments, a nodes repo dir and miner address to create asks with,
# it accepts 2 optional arguments, an ask size in bytes (default 1GB) and ask price,
# if a price is not provided a random value between 1 and 11 will be picked.

function Usage() {
  echo "USAGE:"
  echo
  echo "makeAsk <REPO_DIR> <MINER_ADDRESS> [ASK_SIZE] [ASK_PRICE]"
}

# Required
REPO_DIR="$1"
MINER_ADDRESS="$2"

# Optional
ASK_SIZE="$3"
ASK_PRICE="$4"

if test -z "$REPO_DIR"
then
  echo "ERROR Missing Arg 1: you must specify a repo dir"
  Usage
  echo
  exit 1
fi

if test -z "$MINER_ADDRESS"
then
  echo "ERROR Missing Arg 2: you must specify a miner address to create ask from"
  Usage
  echo
  exit 1
fi

MINER_OWNER=$(go-filecoin --repodir="$REPO_DIR" miner owner "$MINER_ADDRESS")
if test -z "$MINER_OWNER"
then
  echo "ERROR failed to determine miner owner"
  exit 1
fi

if test -z "$ASK_SIZE"
then
  echo "No ask price given, will default to 1GB"
  echo
  ASK_SIZE="1000000000"
fi

if test -z "$ASK_PRICE"
then
  ASK_PRICE=$((1 + RANDOM % 11))
  printf "No ask price given, will use random value: %s\n" "$ASK_PRICE"
  echo
fi

printf "Creating ask on node: %s, miner: %s, owner: %s, size: %s, price: %s\n" "$REPO_DIR" "$MINER_ADDRESS" "$MINER_OWNER" "$ASK_SIZE" "$ASK_PRICE"
  echo

go-filecoin --repodir="$REPO_DIR" miner add-ask "$MINER_ADDRESS" "$ASK_SIZE" "$ASK_PRICE" --from="$MINER_OWNER"
