#!/bin/bash

set -ex
SETUPFILE=$$(cat <<-END
{
  "keys": ["0", "1", "2", "3", "4", "5", "6", "7", "8", "9"],
  "preAlloc": {
    "0": "10000",
    "1": "10000",
    "2": "10000",
    "3": "10000",
    "4": "10000",
    "5": "10000",
    "6": "10000",
    "7": "10000",
    "8": "10000",
    "9": "10000"
  },
  "miners": [
    {
      "owner":"0",
      "power": 100
    },
    {
      "owner":"1",
      "power": 100
    },
    {
      "owner":"2",
      "power": 100
    },
    {
      "owner":"3",
      "power": 100
    },
    {
      "owner":"4",
      "power": 100
    },
    {
      "owner":"5",
      "power": 100
    },
    {
      "owner":"6",
      "power": 100
    },
    {
      "owner":"7",
      "power": 100
    },
    {
      "owner":"8",
      "power": 100
    },
    {
      "owner": "9",
      "power": 100
    }
  ]
}
END
         )

FILECOIN_CONFIG=$$(cat  <<-END
[api]
  address = "/ip4/0.0.0.0/tcp/3453"
  accessControlAllowOrigin = ["http://${instance_name}.kittyhawk.wtf:8000"]
  accessControlAllowCredentials = false
  accessControlAllowMethods = ["GET", "POST", "PUT", "OPTIONS", "HEAD"]
END
               )

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
eval $(aws --region us-east-1 ecr --no-include-email get-login)
docker pull $${FILECOIN_DOCKER_IMAGE}
docker pull $${FILEBEAT_DOCKER_IMAGE}

# start filebeat
docker run -d -v /var/lib/docker/containers:/usr/share/dockerlogs:ro -v /var/run/docker.sock:/var/run/docker.sock --network host --name filebeat -e LOGSTASH_HOSTS=${logstash_hosts} $${FILEBEAT_DOCKER_IMAGE}

${cadvisor_install}
${node_exporter_install}

docker network create --subnet 172.19.0.0/16 filecoin
# DASHBOARD
docker run -d --name=dashboard-aggregator \
       --network=filecoin --hostname=dashboard-aggregator -p 9080:9080 --ip 172.19.0.250 \
       657871693752.dkr.ecr.us-east-1.amazonaws.com/dashboard-aggregator:latest

docker run -d --name=dashboard-visualizer \
       -p 8010:3000 \
       -e DANGEROUSLY_DISABLE_HOST_CHECK=true \
       -e REACT_APP_FEED_URL="ws://${instance_name}.kittyhawk.wtf:9080" \
       -e REACT_APP_EXPLORER_URL="http://${instance_name}.kittyhawk.wtf:8000" \
       657871693752.dkr.ecr.us-east-1.amazonaws.com/dashboard-visualizer:latest

# generate genesis files
CAR_DIR=/home/ubuntu/car
mkdir -p $$CAR_DIR
echo $$SETUPFILE > "$$CAR_DIR/"setup.json
docker run \
       -v "$$CAR_DIR":/var/filecoin/car \
       --entrypoint='/bin/sh'\
       --workdir=/var/filecoin/car/ \
       $${FILECOIN_DOCKER_IMAGE} \
       -c 'cat setup.json | /usr/local/bin/gengen --json > genesis.car 2> gen.json'

FILECOIN_STORAGE=/mnt/storage/filecoins
# initialize filecoin nodes
for i in {0..9}; do mkdir -p "$${FILECOIN_STORAGE}"/$$i; done

GOPATH=/home/ubuntu/go
mkdir -p $GOPATH
GOPATH=/home/ubuntu/go go get -u github.com/whyrusleeping/ipfs-key
for i in {0..9}
do
  # yes this one liner works
  /home/ubuntu/go/bin/ipfs-key 2>&1 >"$$CAR_DIR/keyFile$$i" | tail -n1 | awk '{ print $$NF }' > "$$CAR_DIR/ID$$i"
done

# so here we need to pass the keyFile$i to the node to get a known peerID
for i in {0..9}
do
  docker run \
         -v /home/ubuntu/car:/var/filecoin/car \
         -v "$${FILECOIN_STORAGE}":/var/local/filecoins \
         --entrypoint=/usr/local/bin/go-filecoin \
         $${FILECOIN_DOCKER_IMAGE} \
         init --genesisfile=/var/filecoin/car/genesis.car --repodir="/var/local/filecoins/$$i" --peerkeyfile="/var/filecoin/car/keyFile$$i"
  echo "$$FILECOIN_CONFIG" > "/mnt/storage/filecoins/$i/config.toml"
done
chmod -R 0777 "$${FILECOIN_STORAGE}"

for i in {0..9}
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
         --repodir="/var/local/filecoin" --swarmlisten="/ip4/0.0.0.0/tcp/9000" --block-time="5s"
done

# Here we add the miner address to the nodes config
# so Miner[0] is imported by node 0, Miner[1] -> impprted to node1 etc.
filcoin_exec="go-filecoin --repodir=/var/local/filecoin"
for i in {0..9}
do
  # how we get the miner address
  miner=$$(cat $${CAR_DIR}/gen.json | jq ".Miners[$${i}].Address")
  echo "Adding miner $${miner} to filecoin-$$i"
  docker exec "filecoin-$$i" $$filcoin_exec \
         config mining.minerAddress $${miner}
done

# next we need to import the coresponding keys used to generate the minerAddress, do this like:
for i in {0..9}
do
  # this code has no honor
  cat "$$CAR_DIR/gen.json" | jq -r ".Keys[\"$${i}\"]" > "$$CAR_DIR/wallet$$i"
  docker exec "filecoin-$$i" sh -c "cat /var/filecoin/car/wallet$$i | /usr/local/bin/go-filecoin --repodir=/var/local/filecoin wallet import"
done

for i in {0..9}
do
  for node_addr in $$(docker exec "filecoin-$$i" $$filcoin_exec id --format=\<addrs\>)
  do
    if [[ $$node_addr = *"ip4/172"* ]]; then
      node_docker_addr=$$node_addr
    fi
  done

  echo "$$i: $$node_docker_addr"
  for j in {0..9}
  do
    echo "joining $${j} with peer at: $${node_docker_addr}"
    docker exec "filecoin-$$j" $$filcoin_exec swarm connect "$${node_docker_addr}" || true
  done
done

for i in {0..9}
do
  docker exec -d "filecoin-$i" $filcoin_exec \
         log streamto /ip4/172.19.0.250/tcp/9000
  docker exec "filecoin-$$i" $$filcoin_exec \
         mining start
done

# start block explorer
docker run -d \
       --name block-exporter -p 8000:8000 -e PORT=8000 \
       -e REACT_APP_API_PORT=34530 -e REACT_APP_API_URL="${instance_name}.kittyhawk.wtf" \
       657871693752.dkr.ecr.us-east-1.amazonaws.com/blockexplorer
