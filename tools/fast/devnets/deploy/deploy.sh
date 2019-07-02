#!/usr/bin/env bash

set -xe

docker stop faucet genesis bootstrap1 bootstrap2 bootstrap3 miner1  || true
docker rm faucet genesis bootstrap1 bootstrap2 bootstrap3 miner1    || true

docker network rm filecoin || true

sudo rm -rf $(pwd)/devnet-repo
mkdir -p $(pwd)/devnet-repo/{0,1,2,3,4}

docker network create --subnet 172.19.0.0/24 filecoin

# Bootstrap1 container
docker run -d --name bootstrap1                                       \
  --net filecoin --ip 172.19.0.20                                     \
  -v $(pwd)/devnet-data:/opt/filecoin:ro                              \
  -v $(pwd)/devnet-repo/1:/var/filecoin                               \
  -v $FILECOIN_PARAMETER_CACHE:/var/filecoin-proof-parameters:ro      \
  devnet-base:latest                                                  \
  -profile bootstrap -profile-config /opt/filecoin/bootstrap1.json

# Bootstrap2 container
docker run -d --name bootstrap2                                       \
  --net filecoin --ip 172.19.0.30                                     \
  -v $(pwd)/devnet-data:/opt/filecoin:ro                              \
  -v $(pwd)/devnet-repo/2:/var/filecoin                               \
  -v $FILECOIN_PARAMETER_CACHE:/var/filecoin-proof-parameters:ro      \
  devnet-base:latest                                                  \
  -profile bootstrap -profile-config /opt/filecoin/bootstrap2.json

# Bootstrap3 container
docker run -d --name bootstrap3                                       \
  --net filecoin --ip 172.19.0.40                                     \
  -v $(pwd)/devnet-data:/opt/filecoin:ro                              \
  -v $(pwd)/devnet-repo/3:/var/filecoin                               \
  -v $FILECOIN_PARAMETER_CACHE:/var/filecoin-proof-parameters:ro      \
  devnet-base:latest                                                  \
  -profile bootstrap -profile-config /opt/filecoin/bootstrap3.json

# Faucet
docker run -d --name faucet                                           \
  --net filecoin --ip 172.19.0.5                                      \
  devnet-faucet:latest                                                \
  -fil-api genesis:3453 -fil-wallet t1ld7pd7uszknlrlhpwpd2337uqx6jwy2m5ux6e5y -faucet-val 1000

# Genesis container
docker run -d --name genesis                                          \
  --net filecoin --ip 172.19.0.10                                     \
  -v $(pwd)/devnet-data:/opt/filecoin:ro                              \
  -v $(pwd)/devnet-repo/0:/var/filecoin                               \
  -v $FILECOIN_PARAMETER_CACHE:/var/filecoin-proof-parameters:ro      \
  devnet-base:latest                                                  \
  -profile genesis -profile-config /opt/filecoin/genesis.json

# Miner1 container
docker run -d --name miner1                                           \
  --net filecoin --ip 172.19.0.50                                     \
  -v $(pwd)/devnet-data:/opt/filecoin:ro                              \
  -v $(pwd)/devnet-repo/4:/var/filecoin                               \
  -v $FILECOIN_PARAMETER_CACHE:/var/filecoin-proof-parameters:ro      \
  devnet-base:latest                                                  \
  -profile miner -profile-config /opt/filecoin/miner1.json

docker exec genesis    deploy -profile genesis   -profile-config /opt/filecoin/genesis.json    -step post
docker exec bootstrap1 deploy -profile bootstrap -profile-config /opt/filecoin/bootstrap1.json -step post
docker exec bootstrap2 deploy -profile bootstrap -profile-config /opt/filecoin/bootstrap2.json -step post
docker exec bootstrap3 deploy -profile bootstrap -profile-config /opt/filecoin/bootstrap3.json -step post
docker exec miner1     deploy -profile miner     -profile-config /opt/filecoin/miner1.json     -step post
