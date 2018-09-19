#!/bin/bash
iptb init -- --genesisfile=/data/genesis.car
iptb start -- --block-time=5s

for i in $(eval echo {0..$1})
do
  minerAddr=$(iptb run "$i" cat /data/minerAddr$i | tail -n 2 | head -n 1)
  iptb run "$i" -- go-filecoin config mining.minerAddresses \"[\\\"$minerAddr\\\"]\"
  iptb run "$i" -- go-filecoin wallet import /data/walletKey$i
done

