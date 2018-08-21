#!/bin/bash

set -ex

while fuser /var/lib/dpkg/lock >/dev/null 2>&1 ; do
  sleep 0.5
done

passwd -d root

DD_API_KEY=${datadog_api_key} bash -c "$(curl -L https://raw.githubusercontent.com/DataDog/datadog-agent/master/cmd/agent/install_script.sh)"
echo "logs_enabled: true" >> /etc/datadog-agent/datadog.yaml
mkdir /etc/datadog-agent/conf.d/go.d
cat << EOF > /etc/datadog-agent/conf.d/go.d/conf.yaml
logs:
 - type: file
   path: /tmp/go-filecoin.events.*
   service: go-filecoin
   source: go
EOF
systemctl restart datadog-agent

DEBIAN_FRONTEND=noninteractive apt-get update -yq
DEBIAN_FRONTEND=noninteractive apt-get install -yq git golang-go jq wget htop

export HOME=/root
curl -sSf https://static.rust-lang.org/rustup.sh | sh

wget -r -np -nH -nd https://ipfs.io/ipfs/QmRqXeU37aNKxSXkCFmcSJ5GTDG5cZwSr1aWSH4J7Dppuz -P /home/ubuntu/car
mkdir -p /home/ubuntu/go/src/github.com/filecoin-project
mkdir /home/ubuntu/go/pkg
mkdir /home/ubuntu/go/bin
mkdir /home/ubuntu/filecoins

for i in {0..9}; do mkdir /home/ubuntu/filecoins/$i; done

echo "export GOPATH=/home/ubuntu/go" >> /home/ubuntu/.bash_profile
echo "export PATH=$PATH:/home/ubuntu/go/bin" >> /home/ubuntu/.bash_profile
source /home/ubuntu/.bash_profile

git clone https://${github_token}@github.com/filecoin-project/go-filecoin.git /home/ubuntu/go/src/github.com/filecoin-project/go-filecoin
cd /home/ubuntu/go/src/github.com/filecoin-project/go-filecoin
git checkout ${github_branch_name}

sed -i 's|https://github.com/bitcoin-core/secp256k1.git|https://${github_token}@github.com/bitcoin-core/secp256k1.git|g' .gitmodules
sed -i 's|git@github.com:filecoin-project/rust-proofs.git|https://${github_token}@github.com/filecoin-project/rust-proofs.git|g' .gitmodules
sed -i 's|git@github.com:filecoin-project/go-proofs.git|https://${github_token}@github.com/filecoin-project/go-proofs.git|g' .gitmodules

git submodule sync
git submodule update --init

go run ./build/*.go deps
go run ./build/*.go build
go run ./build/*.go install
# go install ./gengen
# cat /home/ubuntu/setup.json | gengen > genesis.car

miner_addresses=(fcq9d8najuh2fmah6wjcqctt0zuherg9exyfnnl0j fcqk8882zglteqk53wkesffm7vtl8sue0fayesspv fcqpnakl4v06mjevj0aa2e9egwfn24nd9q3c5glru fcqchmpkrjakgsez85d6h3sznmlgv6ukg7l4d53tz fcqaf4kwn3t6e8z4dlhdvcglu8jlwxdlh3t0vrf0c fcq8f29zfx82gt58pz5spz78apurn598pvllgjael fcqcnatvp3yljrm40lt999xvvucak9nh7pgsfegjp fcquwyyqzt23gyz3jxrgla4zx6ds7tkjpvtmg4w5m fcqf8pu3jeamvu0g7ytdnyewqk3rw3dlkx78ph0ne fcq22ke33d3xy05tt4echvl2r6rgq85pj4tvfkdrk)

for i in {0..9}; do go-filecoin init --genesisfile=/home/ubuntu/car/genesis.car --repodir=/home/ubuntu/filecoins/$i; done

for i in {0..9}
do
  go-filecoin daemon --cmdapiaddr=:800$i --repodir=/home/ubuntu/filecoins/$i --swarmlisten=/ip4/127.0.0.1/tcp/900$i --write-logfile=true &
done

for i in {0..9}
do
  go-filecoin --cmdapiaddr=:800$i --repodir=/home/ubuntu/filecoins/$i config mining.minerAddresses [\"$${miner_addresses[$i]}\"]
done

for i in {0..9}
do
  for j in {0..9}
  do
    test=$(go-filecoin --cmdapiaddr=:800$i --repodir=/home/ubuntu/filecoins/$i id --format=\<addrs\>)
    go-filecoin --cmdapiaddr=:800$j --repodir=/home/ubuntu/filecoins/$j swarm connect $test || true
  done
done

for i in {0..9}
do
  go-filecoin --cmdapiaddr=:800$i --repodir=/home/ubuntu/filecoins/$i mining start
done
