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
ExecStart=/usr/bin/dockerd -H unix:// -H tcp://0.0.0.0:2376 --data-root /mnt/storage/docker
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
docker run -d \
       --network host --name filebeat -e LOGSTASH_HOSTS=${logstash_hosts} \
       -v /mnt/storage/docker/containers:/usr/share/dockerlogs:ro \
       -v /var/run/docker.sock:/var/run/docker.sock \
       $${FILEBEAT_DOCKER_IMAGE}

${cadvisor_install}
${node_exporter_install}

# start block explorer
docker run -d \
       --name block-exporter -p 8000:8000 -e PORT=8000 \
       -e REACT_APP_API_PORT=34530 -e REACT_APP_API_URL="${instance_name}.kittyhawk.wtf" \
       657871693752.dkr.ecr.us-east-1.amazonaws.com/blockexplorer

docker network create --subnet 172.19.0.0/24 filecoin
# DASHBOARD
docker run -d --name=dashboard-aggregator \
       --network=filecoin --hostname=dashboard-aggregator -p 9080:9080 -p 19000:9000 \
       --ip 172.19.0.250 \
       657871693752.dkr.ecr.us-east-1.amazonaws.com/dashboard-aggregator:latest

docker run -d --name=dashboard-visualizer \
       -p 8010:3000 \
       -e DANGEROUSLY_DISABLE_HOST_CHECK=true \
       -e REACT_APP_FEED_URL="ws://${instance_name}.kittyhawk.wtf:9080/feed" \
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

# decrypt node keys
KEYS_PASS=$$(aws ssm get-parameter  --region us-east-1 --name kittyhawk-node-keys-pass --with-decryption | jq -r '.Parameter .Value')
echo $$KEYS_PASS | gpg --batch --yes --passphrase-fd 0 --output /home/ubuntu/node_keys.zip /home/ubuntu/node_keys.zip.gpg

# unzip node keys
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
  docker run -d  --restart always \
         --name "filecoin-$$i" --hostname "filecoin-$$i" \
         --network=filecoin -p "900$$i:9000" -p "3453$$i:3453" \
         --ip "172.19.0.1$${i}" \
         --entrypoint=/usr/local/bin/cluster_start \
         -v /home/ubuntu/car:/var/filecoin/car \
         -v "$${FILECOIN_STORAGE}"/$$i:/var/local/filecoin \
         -e IPFS_LOGGING_FMT=nocolor -e FILECOIN_PATH="/var/local/filecoin" \
         -e GO_FILECOIN_LOG_LEVEL=5 \
         --log-driver json-file --log-opt max-size=10m \
         $${FILECOIN_DOCKER_IMAGE} daemon --elstdout \
         --repodir="/var/local/filecoin" --swarmlisten="/ip4/0.0.0.0/tcp/9000" --block-time="30s"
done

filecoin_exec="go-filecoin --repodir=/var/local/filecoin"

# store peer addrs for joining/rejoining on restart
for i in {0..4}
do
  docker exec "filecoin-$i" $filecoin_exec id --format=\<addrs\> | uniq | grep 172\.19 >> $CAR_DIR/peers.txt
done

# configure mining on node 0
minerAddr=$$(cat $${CAR_DIR}/gen.json | grep -v Fixture | jq -r '.Miners[] | select(.Owner == 0).Address')

docker exec "filecoin-0" $$filecoin_exec \
       config mining.minerAddress "\"$${minerAddr}\""

# import miner owner
minerOwner=$$(docker exec "filecoin-0" $$filecoin_exec wallet import "/var/filecoin/car/0.key" | sed -e 's/^"//' -e 's/"$$//')

# update the peerID of the miner to the correct value
peerID=$$(docker exec "filecoin-0" $$filecoin_exec id | grep -v Fixture | jq ".ID" -r)
docker exec "filecoin-0" $$filecoin_exec \
       miner update-peerid --from=$$minerOwner "$$minerAddr" "$$peerID"
# start mining
docker exec "filecoin-0" $$filecoin_exec \
       mining start

# start faucet
docker run -d --name faucet \
       --network=filecoin -p 9797:9797 \
       657871693752.dkr.ecr.us-east-1.amazonaws.com/filecoin-faucet:76b219 \
       -fil-api filecoin-0:3453 -fil-wallet $${minerOwner} -faucet-val 1000

# connect nodes
for i in {0..4}
do
  for node_addr in $$(cat $${CAR_DIR}/peers.txt)
  do
    echo "joining $${i} with peer at: $${node_addr}"
    docker exec "filecoin-$$i" $$filecoin_exec swarm connect "$${node_addr}" || true
  done
done

# start streaming events to aggregator
for i in {0..4}
do
  docker exec "filecoin-$$i" $$filecoin_exec \
         config heartbeat.nickname '"boot"'
  docker exec -d "filecoin-$$i" $$filecoin_exec \
         log streamto /ip4/172.19.0.250/tcp/9000
done

# send some tokens
for i in {1..4}
do
  nodeAddr=$$(docker exec "filecoin-$$i" $$filecoin_exec wallet addrs ls)
  msgCidRaw=$$(docker exec "filecoin-0" $$filecoin_exec message send --from "$$minerOwner" --value 100 "$$nodeAddr")
  msgCid=$$(echo $$msgCidRaw | sed -e 's/^node\[0\] exit 0 //')
  docker exec "filecoin-$$i" $$filecoin_exec \
         message wait "$$msgCid"

  # create the actual miner
  newMinerAddr=$$(docker exec "filecoin-$$i" $$filecoin_exec miner create 10 10)

  # start mining
  docker exec "filecoin-$$i" $$filecoin_exec \
         mining start

  # add an ask
  MINER_ADD_ASK_MSG_CID=$$(docker exec "filecoin-$$i" $$filecoin_exec miner add-ask $${newMinerAddr} 10 10000)

  # wait for ask to be mined
  docker exec "filecoin-$$i" $$filecoin_exec message wait $${MINER_ADD_ASK_MSG_CID}

  # make a deal
  dd if=/dev/random of="$$CAR_DIR/fake.dat"  bs=1M  count=1 # small data file will be autosealed
  dataCid=$$(docker exec "filecoin-0" $$filecoin_exec client import "/var/filecoin/car/fake.dat")
  rm "$$CAR_DIR/fake.dat"

  docker exec "filecoin-0" $$filecoin_exec \
         client propose-storage-deal $${newMinerAddr} $${dataCid} 0 8640
done


# Deployment checks
send_alert () {
  alertmanager_basic_auth=$$(aws ssm get-parameter  --region us-east-1 --name kittyhawk-node-alertmanager_basic_auth --with-decryption | jq -r '.Parameter .Value')
  alerts="[
    {
      \"labels\": {
         \"alertname\": \"$$1\"
       },
       \"annotations\": {
          \"description\": \"$$2\"
        }
    }
  ]"
  curl -XPOST -d"$$alerts" -u $${alertmanager_basic_auth} \
       ${alertmanager_url}
}

# Faucet Check
check_faucet () {
  local faucetIp faucetHost faucetTarget
  local -i count balance startBalance

  faucetIp=$$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' faucet)
  faucetHost="$$faucetIp:9797"
  faucetTarget=$$(docker exec "filecoin-4" $$filecoin_exec wallet addrs ls)
  startBalance=$$(docker exec "filecoin-4" $$filecoin_exec wallet balance $$faucetTarget)

  curl -sSL -D - -XPOST \
       --data "target=$$faucetTarget" \
       "http://$$faucetHost/tap" \
       -o /dev/null

  for (( count=3; count>0; count-- )); do
    balance=$$(docker exec "filecoin-4" $$filecoin_exec wallet balance $$faucetTarget)

    if [ $$balance -gt $$startBalance ]; then
      break
    fi

    # Sleep for blocktime
    sleep 30
  done

  if [ $$count -eq 0 ]; then
    send_alert "${instance_name} faucet" "Faucet did not transfer funds in required time during cluster setup"
    exit 1
  fi
}

# Run checks
check_faucet

