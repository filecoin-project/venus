#!/bin/bash

set -ex
FILECOIN_CONFIG=$$(cat  <<-END
[api]
  address = "/ip4/0.0.0.0/tcp/3453"
  accessControlAllowOrigin = ["http://${instance_name}.kittyhawk.wtf:8000"]
  accessControlAllowCredentials = false
  accessControlAllowMethods = ["GET", "POST", "PUT", "OPTIONS", "HEAD"]
END
               )

hostnamectl --static set-hostname ${instance_name}.kittyhawk.wtf
${setup_instance_storage}

# expose Docker daemon on TCP 2376
DOCKER_OVERRIDE=$$(cat <<-END
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd -H fd:// -H tcp://0.0.0.0:2376
END
               )
DOCKER_OVERRIDE_PATH=/etc/systemd/system/docker.service.d/
mkdir -p "$$DOCKER_OVERRIDE_PATH"
echo "$$DOCKER_OVERRIDE" > "$$DOCKER_OVERRIDE_PATH/"override.conf
/bin/systemctl daemon-reload

${docker_install}
# pull images
FILECOIN_DOCKER_IMAGE=${docker_uri}:${docker_tag}
FILEBEAT_DOCKER_IMAGE=${filebeat_docker_uri}:${filebeat_docker_tag}
eval $$(aws --region us-east-1 ecr --no-include-email get-login)
docker pull $${FILECOIN_DOCKER_IMAGE}
docker pull $${FILEBEAT_DOCKER_IMAGE}

# start filebeat
docker run -d -v /var/lib/docker/containers:/usr/share/dockerlogs:ro -v /var/run/docker.sock:/var/run/docker.sock --network host --name filebeat -e LOGSTASH_HOSTS=${logstash_hosts} $${FILEBEAT_DOCKER_IMAGE}

${cadvisor_install}
${node_exporter_install}

# start block explorer
docker run -d \
       --name block-exporter -p 8000:8000 -e PORT=8000 \
       -e REACT_APP_API_PORT=34530 -e REACT_APP_API_URL="${instance_name}.kittyhawk.wtf" \
       657871693752.dkr.ecr.us-east-1.amazonaws.com/blockexplorer

docker network create --subnet 172.19.0.0/16 filecoin
# DASHBOARD
docker run -d --name=dashboard-aggregator \
       --network=filecoin --hostname=dashboard-aggregator -p 9080:9080 -p 19000:9000 \
       --ip 172.19.0.250 \
       657871693752.dkr.ecr.us-east-1.amazonaws.com/dashboard-aggregator:latest

docker run -d --name=dashboard-visualizer \
       -p 8010:3000 \
       -e DANGEROUSLY_DISABLE_HOST_CHECK=true \
       -e REACT_APP_FEED_URL="ws://${instance_name}.kittyhawk.wtf:9080" \
       -e REACT_APP_EXPLORER_URL="http://${instance_name}.kittyhawk.wtf:8000" \
       657871693752.dkr.ecr.us-east-1.amazonaws.com/dashboard-visualizer:latest

# copy genesis files
CAR_DIR=/home/ubuntu/car
mkdir -p $$CAR_DIR
docker run \
       -v "$$CAR_DIR":/var/filecoin/car \
       --entrypoint='/bin/sh'\
       --workdir=/var/filecoin/car/ \
       $${FILECOIN_DOCKER_IMAGE} \
       -c 'cp /data/genesis.car . && cp /data/*.key . && cp /data/gen.json . && cp /data/setup.json .'

FILECOIN_STORAGE=/mnt/storage/filecoins
# initialize filecoin nodes
for i in {0..4}; do mkdir -p "$${FILECOIN_STORAGE}"/$$i; done

# unzip keys
apt install -y unzip
unzip -q /home/ubuntu/node_keys.zip -d /home/ubuntu/car/

# so here we need to pass the keyFile$i to the node to get a known peerID
for i in {0..4}
do
  docker run \
         -v /home/ubuntu/car:/var/filecoin/car \
         -v "$${FILECOIN_STORAGE}":/var/local/filecoins \
         --entrypoint=/usr/local/bin/go-filecoin \
         $${FILECOIN_DOCKER_IMAGE} \
         init --genesisfile=/var/filecoin/car/genesis.car --repodir="/var/local/filecoins/$$i" --peerkeyfile="/var/filecoin/car/keyFile$$i"
  echo "$$FILECOIN_CONFIG" > "/mnt/storage/filecoins/$$i/config.toml"
done
chmod -R 0777 "$${FILECOIN_STORAGE}"

for i in {0..4}
do
  echo "Starting filecoin-$$i"
  docker run -d \
         --name "filecoin-$$i" --hostname "filecoin-$$i" \
         --network=filecoin -p "900$$i:9000" -p "3453$$i:3453" \
         -v /home/ubuntu/car:/var/filecoin/car \
         -v "$${FILECOIN_STORAGE}"/$$i:/var/local/filecoin \
         -e IPFS_LOGGING_FMT=nocolor -e FILECOIN_PATH="/var/local/filecoin" \
         --log-driver json-file --log-opt max-size=10m \
         $${FILECOIN_DOCKER_IMAGE} daemon --elstdout \
         --repodir="/var/local/filecoin" --swarmlisten="/ip4/0.0.0.0/tcp/9000" --block-time="30s"
done

filecoin_exec="go-filecoin --repodir=/var/local/filecoin"

# configure mining on node 0
minerAddr=$$(cat $${CAR_DIR}/gen.json | grep -v Fixture | jq ".Miners[0].Address" -r)

docker exec "filecoin-0" $$filecoin_exec \
       config mining.minerAddress "\"$${minerAddr}\""

# import miner owner
minerOwner=$$(docker exec "filecoin-0" $$filecoin_exec wallet import "/var/filecoin/car/a.key" | tail -n +2 | sed -e 's/^"//' -e 's/"$$//')

# update the peerID of the miner to the correct value
peerID=$$(docker exec "filecoin-0" $$filecoin_exec id | grep -v Fixture | jq ".ID" -r)
docker exec "filecoin-0" $$filecoin_exec \
       miner update-peerid --from=$$minerOwner "$$minerAddr" "$$peerID"
# start mining
docker exec "filecoin-0" $$filecoin_exec \
       mining start

# connect nodes
for i in {0..4}
do
  for node_addr in $$(docker exec "filecoin-$$i" $$filecoin_exec id --format=\<addrs\>)
  do
    if [[ $$node_addr = *"ip4/172"* ]]; then
      node_docker_addr=$$node_addr
    fi
  done

  echo "$$i: $$node_docker_addr"
  for j in {0..4}
  do
    echo "joining $${j} with peer at: $${node_docker_addr}"
    docker exec "filecoin-$$j" $$filecoin_exec swarm connect "$${node_docker_addr}" || true
  done
done

# start streaming events to aggregator
for i in {0..4}
do
  docker exec -d "filecoin-$$i" $$filecoin_exec \
         log streamto /ip4/172.19.0.250/tcp/9000
done

  # send some tokens
for i in {1..4}
do
  nodeAddr=$$(docker exec "filecoin-$$i" $$filecoin_exec wallet addrs ls | tail -n +2)
  msgCidRaw=$$(docker exec "filecoin-0" $$filecoin_exec message send --from "$$minerOwner" --value 100 "$$nodeAddr" | tail -n +2)
  msgCid=$$(echo $$msgCidRaw | sed -e 's/^node\[0\] exit 0 //')
  docker exec "filecoin-$$i" $$filecoin_exec \
         message wait "$$msgCid"

  # create the actual miner
  newMinerAddr=$$(docker exec "filecoin-$$i" $$filecoin_exec miner create 10 10 | tail -n +2)

  # start mining
  docker exec "filecoin-$$i" $$filecoin_exec \
         mining start

  # add an ask
  # 1024*1024*1024*3
  docker exec "filecoin-$$i" $$filecoin_exec \
         miner add-ask "$$newMinerAddr" 3221225472 1

  # make a deal
  dd if=/dev/random of="$$CAR_DIR/fake.dat"  bs=1M  count=1 # small data file will be autosealed
  dataCid=$$(docker exec "filecoin-0" $$filecoin_exec client import "/var/filecoin/car/fake.dat" | tail -n +2)
  rm "$$CAR_DIR/fake.dat"

  docker exec "filecoin-0" $$filecoin_exec \
         client propose-storage-deal --price 1 "$$newMinerAddr" "$$dataCid" 10000
done

# start faucet
MONEY_BAGS_ADDR=$$(docker exec "filecoin-0" $$filecoin_exec wallet addrs ls | tail -n1)
docker run -d --name faucet \
       --network=filecoin -p 9797:9797 \
       657871693752.dkr.ecr.us-east-1.amazonaws.com/filecoin-faucet:76b219 \
       -fil-api filecoin-0:3453 -fil-wallet $${MONEY_BAGS_ADDR} -faucet-val 1000
