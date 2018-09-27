#!/bin/bash

# This script is used to create an IPTB testbed, initialize the testbed nodes with a genesis file,
# start the testbed nodes, configure the testbed nodes wallet addresses and miner address
# from the addresses in the aforementioned genesis file s.t. the nodes can mine, and lastly connect the
# testbed nodes together.
#
# This script is useful when you want to setup local filecoin processes that can mine.
# This script can be ran like any other bash script.

# TODO add tests to verify this always works.


if test -z "$1"
then
  echo "ERROR: you must pass value for number of nodes you wish to init, e.g.: 10"
  exit 1
fi

# create a testbed for the iptb nodes
iptb testbed create --count "$1" --type localfilecoin --force

# set common paths to find bins and confi files
GENDIR=$GOPATH/src/github.com/filecoin-project/go-filecoin/gengen
GENGEN=$GOPATH/src/github.com/filecoin-project/go-filecoin/gengen/gengen
GENSET=$GOPATH/src/github.com/filecoin-project/go-filecoin/gengen/gensetup

# generate N values in setup file where N is the passed value
$GENSET -count "$1" > $GENDIR/setup.json
# now generate a genesis car file from said setup script
cat $GENDIR/setup.json | $GENGEN --json > $GENDIR/genesis.car 2> $GENDIR/gen.json

# configure mining addresses on all the nodes
for i in $(eval echo {0..$1})
do
  cat $GENDIR/gen.json | jq ".Miners[$i].Address" > $GENDIR/minerAddr$i
done

# import the corresponding keys into the nodes wallet, this allows them
# to "own" the miner addresses from above
for i in $(eval echo {0..$1})
do
  cat $GENDIR/gen.json | jq ".Keys[\"$i\"]" > $GENDIR/walletKey$i
done

printf "Initializing %d nodes\n" "$1"
# init all the nodes using the generated genesis file
iptb init -- --genesisfile=$GENDIR/genesis.car

printf "Starting %d nodes\n" "$1"
# start their daemons with a block time of 5 seconds
iptb start -- --block-time=5s

printf "Configuring %d nodes\n" "$1"
for i in $(eval echo {0..$1})
do
  minerAddr=$(iptb run "$i" cat $GENDIR/minerAddr$i | tail -n 2 | head -n 1)
  iptb run "$i" -- go-filecoin config mining.minerAddress "$minerAddr"
  iptb run "$i" -- go-filecoin wallet import $GENDIR/walletKey$i
done

printf "Connecting %d nodes\n" "$1"
iptb connect

printf "Complete! %d nodes connected and ready to mine >.>" "$1"
