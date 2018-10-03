#!/bin/bash

# ONLY TESTED ON LINUX, OSX UNTESTED (probably missing timeout command)
# This script can be used to created asks on the filecoin network,
# it accepts 2 required arguments, a nodes command api multiaddr and miner address to create asks with,
# it accepts 3 optional arguments, an ask size in bytes (default 1GB) an ask price,
# if a price is not provided a random value between 1 and 11 will be picked, and a
# timeout value for ask message propogation, if no timeout is given then will default to 1min.

function Usage() {
  echo "USAGE:"
  echo
  echo "makeAsk <CMD_API> <MINER_ADDRESS> [ASK_SIZE] [ASK_PRICE] [MSG_TIMEOUT]"
}

# Sanity check things are installed
if ! [ -x "$(command -v jq)" ]; then
  echo 'Error: jq is not in path.' >&2
  exit 1
fi

if ! [ -x "$(command -v timeout)" ]; then
  echo 'Error: timeout is not in path.' >&2
  exit 1
fi

if ! [ -x "$(command -v go-filecoin)" ]; then
  echo 'Error: go-filecoin is not in path.' >&2
  exit 1
fi

# Required
CMD_API="$1"
MINER_ADDRESS="$2"

# Optional
ASK_SIZE="$3"
ASK_PRICE="$4"
# ask message propogation timeout in seconds
MSG_TIMEOUT="$5"

if test -z "$CMD_API"
then
  echo "ERROR Missing Arg 1: you must specify a command api multiaddress"
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

MINER_OWNER=$(go-filecoin --cmdapiaddr="$CMD_API" miner owner "$MINER_ADDRESS")
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

if test -z "$MSG_TIMEOUT"
then
  MSG_TIMEOUT=60
  printf "No timout given, will use default value: %s second\n" "$MSG_TIMEOUT"
  echo
fi

printf "Creating ask on node: %s, miner: %s, owner: %s, size: %s, price: %s\n" "$CMD_API" "$MINER_ADDRESS" "$MINER_OWNER" "$ASK_SIZE" "$ASK_PRICE"
echo

MSG_CID=$(go-filecoin --cmdapiaddr="$CMD_API" miner add-ask "$MINER_ADDRESS" "$ASK_SIZE" "$ASK_PRICE" --from="$MINER_OWNER")

MSG_RESULT=$(timeout $MSG_TIMEOUT go-filecoin --cmdapiaddr="$CMD_API" message wait $MSG_CID)

EXIT_CODE=$(echo $MSG_RESULT | jq ".exitCode" | grep -v "null")

if [ "$EXIT_CODE" -eq "0" ]; then
   ASK_BOOK=$(go-filecoin --cmdapiaddr="$CMD_API" orderbook asks | jq '.')
   printf "Ask create success. Orderbook Asks:\n%s" "$ASK_BOOK"
   exit 0
else
  printf "Ask create failed. Message:\n%s" "$MSG_RESULT"
  exit 1
fi
