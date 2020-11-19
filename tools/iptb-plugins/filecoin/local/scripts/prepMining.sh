#!/bin/bash

# This script is useful when you want to setup local filecoin processes that can mine.
# This script can be ran like any other bash script.

# This script is used to create an IPTB testbed, initialize the testbed nodes with a genesis file,
# start the testbed nodes, configure the testbed nodes wallet addresses and miner address
# from the addresses in the aforementioned genesis file s.t. the nodes can mine, and lastly connect the
# testbed nodes together.
#
# TODO add tests to verify this always works.

# Linux and OSX have different dd flags
DD_FILE_SIZE=1m
if [[ "$OSTYPE" == "linux-gnu" ]]; then
  # <3
  DD_FILE_SIZE=1M
fi

if test -z "$1"
then
  echo "ERROR: you must pass value for number of nodes you wish to init, e.g.: 10"
  exit 1
fi

if test -z "$GOPATH"; then
	GOPATH=$(go env GOPATH)
fi

# create a testbed for the iptb nodes
iptb testbed create --count "$1" --type localfilecoin --force

# set common paths to find bins and config files
GENDIR=$GOPATH/src/github.com/filecoin-project/venus/tools/gengen
FIXDIR=$GOPATH/src/github.com/filecoin-project/venus/fixtures

printf "Setting up initial boostrap node (0)\n"
# configure mining on node 0
minerAddr=$(cat $FIXDIR/gen.json | jq ".Miners[0].Address" -r)

iptb init 0 -- --genesisfile=$FIXDIR/genesis.car
iptb start 0
iptb run 0 -- venus config mining.minerAddress "\"$minerAddr\""

# import miner owner
ownerRaw=$(iptb run 0 -- venus wallet import "$FIXDIR/0.key")
# sad face, iptb makes me do all the jumps
minerOwner=$(echo $ownerRaw | sed -e 's/^node\[0\] exit 0 //' | jq -r ".")
# update the peerID to the correct value
peerID=$(iptb run 0 -- venus id | tail -n +3 | jq ".ID" -r)
iptb run 0 -- venus miner update-peerid --from="$minerOwner" --gas-price=0 --gas-limit=300 "$minerAddr" "$peerID"
# start mining
iptb run 0 -- venus mining start

# ranges are inclusive in bash, so subtract one
J=$(($1 - 1))

# init all other nodes
for i in `seq 1 $J`
do
    iptb init "$i" -- --genesisfile=$FIXDIR/genesis.car --auto-seal-interval-seconds 1 # autosealing every second
    iptb start "$i"
done

# connect nodes
printf "Connecting %d nodes\n" "$1"
iptb connect

printf "Creating miners\n"

# configure mining addresses on all the nodes
for i in `seq 1 $J`
do
    # send some tokens
    nodeAddr=$(iptb run "$i" -- venus wallet addrs ls | tail -n +3)
    msgCidRaw=$(iptb run 0 -- venus message send --from "$minerOwner" --value 100 "$nodeAddr")
    msgCid=$(echo $msgCidRaw | sed -e 's/^node\[0\] exit 0 //')
    echo "Waiting for $msgCid"
    iptb run "$i" -- venus message wait "$msgCid"

    # create the actual miner
    newMinerAddr=$(iptb run "$i" -- venus miner create 10 | tail -n +3)

    # start mining
    iptb run "$i" -- venus mining start  # I don't think these guys need to mine yet, wait until the deal is processed

    # add an ask
    printf "adding ask"
    iptb run "$i" -- venus miner set-price --miner="$newMinerAddr" 1 100000 --gas-price=0 --gas-limit=300 # price of one FIL/whatever, ask is valid for 100000 blocks

    # make a deal
    dd if=/dev/random of="$FIXDIR/fake.dat"  bs="$DD_FILE_SIZE"  count=1 # small data file will be autosealed
    dataCidRaw=$(iptb run 0 -- venus client import "$FIXDIR/fake.dat")
    rm "$FIXDIR/fake.dat"
    dataCid=$(echo $dataCidRaw | sed -e 's/^node\[0\] exit 0 //')
    printf "making deal"

    echo $newMinerAddr
    echo $dataCid
    iptb run 0 -- venus client propose-storage-deal "$newMinerAddr" "$dataCid" 1 10000  # I think this is where stuff fails right now??
done

printf "Complete! %d nodes connected and ready to mine >.>" "$1"
