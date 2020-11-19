#!/bin/bash

# This script is useful when you want to setup dockerized filecoin instances that can mine.
# This script can be ran like any other bash script.

# This script is used to create an IPTB testbed, initialize the testbed nodes with a genesis file,
# start the testbed nodes, configure the testbed nodes wallet addresses and miner address
# from the addresses in the aforementioned genesis file s.t. the nodes can mine, and lastly connect the
# testbed nodes together.
#
# This script has a hard limit on the number of node called MAX_NODES, this is
#   due to our docker configuration, for more information on this limit ask @frrist.

# TODO add tests to verify this always works.

MAX_NODES=25

if test -z "$1"
then
  echo "ERROR: you must pass value for number of nodes you wish to init, e.g.: 10"
  exit 1
fi


if [ "$1" -gt "$MAX_NODES" ];
then
  printf "If you wish to run with a value larger that 25, you must edit the Dockerfile in the venus repo\n
  Where to edit:\n
  ENV GENSETUP_COUNT 25 #<--SET THIS VALUE\n
  After edit you must rebuild the docker file:\n
  $ docker build -t venus .
  "
  exit 1
fi

# create a testbed for the iptb nodes
iptb testbed create --count "$1" --type dockerfilecoin --force

printf "Initializing %d nodes\n" "$1"
iptb init -- --genesisfile=/data/genesis.car

printf "Starting %d nodes\n" "$1"
iptb start

printf "Configuring %d nodes\n" "$1"
for i in $(eval echo {0..$1})
do
  minerAddr=$(iptb run "$i" cat /data/minerAddr$i | tail -n 2 | head -n 1)
  iptb run "$i" -- venus config mining.minerAddress \"\\\"$minerAddr\\\"\"
  iptb run "$i" -- venus wallet import /data/walletKey$i
done

printf "Connecting %d nodes\n" "$1"
iptb connect

printf "Complete! %d nodes connected and ready to mine >.>" "$1"
