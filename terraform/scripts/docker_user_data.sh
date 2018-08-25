#!/bin/bash

set -ex

while fuser /var/lib/dpkg/lock >/dev/null 2>&1 ; do
  sleep 0.5
done

passwd -d root

DEBIAN_FRONTEND=noninteractive apt-get purge -qy docker docker-engine docker.io || echo "Purged old versions of docker"
DEBIAN_FRONTEND=noninteractive apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y apt-transport-https ca-certificates curl software-properties-common awscli

# add Docker gpg key and repo
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
DEBIAN_FRONTEND=noninteractive \
               add-apt-repository \
               "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
                 $(lsb_release -cs) stable"

DEBIAN_FRONTEND=noninteractive apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y docker-ce

# get genesis.car
wget -r -np -nH -nd https://ipfs.io/ipfs/QmRqXeU37aNKxSXkCFmcSJ5GTDG5cZwSr1aWSH4J7Dppuz -P /home/ubuntu/car

# pull image
DOCKER_IMAGE=657871693752.dkr.ecr.us-east-1.amazonaws.com/filecoin:1d6357
eval $(aws --region us-east-1 ecr --no-include-email get-login)
docker pull ${DOCKER_IMAGE}

# initialize filecoin nodes
for i in {0..9}; do mkdir -p /var/local/filecoins/$i; done

# TODO mount /data/filecoin from the host ?
for i in {0..9}
do
  docker run -it \
         -v /home/ubuntu/car:/var/filecoin/car \
         -v /var/local/filecoins:/var/local/filecoins \
         --entrypoint=/usr/local/bin/go-filecoin \
         ${DOCKER_IMAGE} \
         init --genesisfile=/var/filecoin/car/genesis.car --repodir=/var/local/filecoins/$i
done

chmod -R 0777 /var/local/filecoins/

for i in {0..9}
do
  echo "Starting filecoin_$i"
  docker run -d --name "filecoin_$i" --expose "9000" \
         -v /var/local/filecoins/$i:/var/local/filecoin \
         ${DOCKER_IMAGE} daemon \
         --repodir="/var/local/filecoin" --swarmlisten="/ip4/0.0.0.0/tcp/9000"
done

filcoin_exec="go-filecoin --repodir=/var/local/filecoin"
miner_addresses=(fcq9d8najuh2fmah6wjcqctt0zuherg9exyfnnl0j fcqk8882zglteqk53wkesffm7vtl8sue0fayesspv fcqpnakl4v06mjevj0aa2e9egwfn24nd9q3c5glru fcqchmpkrjakgsez85d6h3sznmlgv6ukg7l4d53tz fcqaf4kwn3t6e8z4dlhdvcglu8jlwxdlh3t0vrf0c fcq8f29zfx82gt58pz5spz78apurn598pvllgjael fcqcnatvp3yljrm40lt999xvvucak9nh7pgsfegjp fcquwyyqzt23gyz3jxrgla4zx6ds7tkjpvtmg4w5m fcqf8pu3jeamvu0g7ytdnyewqk3rw3dlkx78ph0ne fcq22ke33d3xy05tt4echvl2r6rgq85pj4tvfkdrk)
for i in "${!miner_addresses[@]}"
do
  miner=${miner_addresses[$i]}
  echo "Adding miner ${miner} to filecoin_$i"
  docker exec "filecoin_$i" $filcoin_exec \
         config mining.minerAddresses [\"${miner}\"]
done

for i in {0..9}
do
  for node_addr in $(docker exec -it "filecoin_$i" $filcoin_exec id --format=\<addrs\>)
  do
    if [[ $node_addr = *"ip4/172"* ]]; then
      node_docker_addr=$node_addr
    fi
  done

  echo $node_docker_addr
  for j in {0..9}
  do
    echo "joining ${j} with peer at: ${node_docker_addr}"
    docker exec -it "filecoin_$j" $filcoin_exec swarm connect "${node_docker_addr}" || true
  done
done

for i in {0..9}
do
  docker exec "filecoin_$i" $filcoin_exec \
         mining start
done
