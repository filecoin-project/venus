#!/usr/bin/env bash

go run ./build/*.go build

set -e
set -o pipefail

export GO_FILECOIN_LOG_LEVEL=3
export FILECOIN_PROOFS_FAST_DELAY_SECONDS=1

if [ -z $1 ]; then
    USE_SMALL_SECTORS="true"
else
    USE_SMALL_SECTORS=$1
fi

export FIL_USE_SMALL_SECTORS=${USE_SMALL_SECTORS}

if [ -z $2 ]; then
    AUTO_SEAL_INTERVAL_SECONDS="0"
else
    AUTO_SEAL_INTERVAL_SECONDS=$2
fi

# forward-declare stuff that we need to clean up
MN_PID=""
CL_PID=""
MN_REPO_DIR=""
CL_REPO_DIR=""
PIECE_1_PATH=$(mktemp)
PIECE_2_PATH=$(mktemp)
UNSEAL_PATH=$(mktemp)
BLOCK_TIME="5s"
HODL="HODL HODL HODL HODL HODL HODL HODL HODL HODL HODL HODL HODL HODL HODL HODL"

if [ "${USE_SMALL_SECTORS}" = true ] ; then
    echo ${HODL} > ${PIECE_1_PATH}
    echo "BLOCKCHAIN BLOCKCHAIN BLOCKCHAIN BLOCKCHAIN BLOCKCHAIN BLOCKCHAIN BLOCKCHAIN" > ${PIECE_2_PATH}
else
    # Keeping first piece block count at: 1040000 < 1040384 = 1065353216/1024
    # will prevent first piece from triggering seal.
    dd if=/dev/urandom of=${PIECE_1_PATH} bs=1024 count=1040000
    # A total block count of: (piece_1_count + piece_2_count) = 1040000 + 500 = 1040500 > 1040384
    # will cause the second piece to trigger a seal.
    dd if=/dev/urandom of=${PIECE_2_PATH} bs=1024 count=500
fi

function finish {
  echo ""
  echo "cleaning up..."
  kill $MN_PID || true
  kill $CL_PID || true
  rm -f ${PIECE_1_PATH}
  rm -f ${PIECE_2_PATH}
  rm -f ${UNSEAL_PATH}
  rm -rf ${CL_REPO_DIR}
  rm -rf ${MN_REPO_DIR}
}

trap finish EXIT

function free_port {
  python -c "import socket; s = socket.socket(); s.bind(('', 0)); print(s.getsockname()[1])"
}

function import_private_key {
  ./go-filecoin wallet import ./fixtures/$1.key \
    --repodir=$2 \
    | jq -r ""
}

function init_daemon {
  ./go-filecoin init \
    --auto-seal-interval-seconds=${AUTO_SEAL_INTERVAL_SECONDS} \
    --repodir=$1 \
    --cmdapiaddr=/ip4/127.0.0.1/tcp/$2 \
    --walletfile= \
    --walletaddr= \
    --testgenesis=false \
    --genesisfile=$3
}

function start_daemon {
  ./go-filecoin daemon \
    --repodir=$1 \
    --block-time=${BLOCK_TIME} \
    --cmdapiaddr=/ip4/127.0.0.1/tcp/$2 \
    --swarmlisten=/ip4/127.0.0.1/tcp/$3 &
}

function get_first_address {
  ./go-filecoin id \
    --repodir=$1 \
    | jq -r ".Addresses[0]"
}

function get_peer_id {
  ./go-filecoin id \
    --repodir=$1 \
    | jq -r ".ID"
}

function get_peers {
  ./go-filecoin swarm peers \
    --repodir=$1
}

function swarm_connect {
  ./go-filecoin swarm connect $1 \
    --repodir=$2

  local __peers=$(get_peers $2)
  until [[ "$__peers" = "$1" ]]; do
    __peers=$(get_peers $2)
    sleep 1
  done
}

function chain_ls {
  ./go-filecoin chain ls --enc=json \
    --repodir=$1
}

function wait_for_message_in_chain_by_method_and_sender {
  IFS=$'\n' # make newlines the only separator

  local __chain=""
  local __hodl=""

  while [ -z $__hodl ]; do
    __chain=$(chain_ls $3)

    __hodl=""
    for blk in $__chain
    do
        __hodl=$(echo "$blk" | jq ".[].messages[].message | select(.method == \"$1\").from | select(. == \"$2\")" 2>/dev/null | head -n 1)
        if [ ! -z $__hodl ]; then
          break
        fi
    done

    echo "$(date "+%T") - sleeping for 10 seconds"
    sleep 10
  done

  unset IFS
}

function set_wallet_default_address_in_config {
  ./go-filecoin config wallet.defaultAddress \"$1\" \
    --repodir=$2
}

function set_mining_address_in_config {
  ./go-filecoin config mining.minerAddress \"$1\" \
    --repodir=$2
}

function wait_mpool_size {
  ./go-filecoin mpool \
    --wait-for-count=$1 \
    --repodir="$2"
}

function add_ask {
  ./go-filecoin miner add-ask $1 $2 $3 \
    --repodir="$4"
}

function miner_update_pid {
  ./go-filecoin miner update-peerid $1 $2 \
    --repodir=$3
}

function mine_once {
  ./go-filecoin mining once \
    --repodir=$1
}

function fork_message_wait {
  eval "exec $1< <(./go-filecoin message wait $2 --repodir=$3)"
}

function join {
  echo $(eval "cat <&$1")
}

MN_REPO_DIR=$(mktemp -d)
MN_CMDAPI_PORT=$(free_port)
MN_SWARM_PORT=$(free_port)

CL_REPO_DIR=$(mktemp -d)
CL_CMDAPI_PORT=$(free_port)
CL_SWARM_PORT=$(free_port)

echo ""
echo "generating private keys..."

MN_MINER_FIL_ADDR=$(cat fixtures/gen.json | jq -r '.Miners[] | select(.Owner == 0).Address')

echo ""
echo "initializing daemons..."
init_daemon ${MN_REPO_DIR} ${MN_CMDAPI_PORT} ./fixtures/genesis.car
init_daemon ${CL_REPO_DIR} ${CL_CMDAPI_PORT} ./fixtures/genesis.car

echo ""
echo "start daemons..."
start_daemon ${MN_REPO_DIR} ${MN_CMDAPI_PORT} ${MN_SWARM_PORT}
MN_PID=$!
start_daemon ${CL_REPO_DIR} ${CL_CMDAPI_PORT} ${CL_SWARM_PORT}
CL_PID=$!

sleep 2

echo ""
echo "importing private keys..."
MN_MINER_OWNER_FIL_ADDR=$(import_private_key 0 ${MN_REPO_DIR})
CL_FIL_ADDRESS=$(import_private_key 1 ${CL_REPO_DIR})

echo ""
echo "ensure that miner address is set so that the miner-node can mine..."
set_mining_address_in_config ${MN_MINER_FIL_ADDR} ${MN_REPO_DIR}

echo ""
echo "node default address should match what's associated with imported SK..."
set_wallet_default_address_in_config ${CL_FIL_ADDRESS} ${CL_REPO_DIR}
set_wallet_default_address_in_config ${MN_MINER_OWNER_FIL_ADDR} ${MN_REPO_DIR}

echo ""
echo "get mining node's libp2p identity..."
MN_PEER_ID=$(get_peer_id ${MN_REPO_DIR})

echo ""
echo "get the client's peer id..."
CL_PEER_ID=$(get_peer_id ${CL_REPO_DIR})

echo ""
echo "update miner's libp2p identity to match its node's..."
MINER_UPDATE_PID_MSG_CID=$(miner_update_pid ${MN_MINER_FIL_ADDR} ${MN_PEER_ID} ${MN_REPO_DIR})
MINER_ADD_ASK_MSG_CID=$(add_ask ${MN_MINER_FIL_ADDR} 10 10000 ${MN_REPO_DIR})

echo ""
echo "connecting daemons..."
swarm_connect $(get_first_address ${CL_REPO_DIR}) ${MN_REPO_DIR}
swarm_connect $(get_first_address ${MN_REPO_DIR}) ${CL_REPO_DIR}

echo ""
echo ""
echo ""
echo "********************** BEGIN STORAGE PROTOCOL"
echo ""
echo ""
echo ""

echo ""
echo "miner mines a block (to include commitSector message)..."
mine_once ${MN_REPO_DIR}

echo ""
echo "miner node starts mining..."
./go-filecoin mining start \
  --repodir=$MN_REPO_DIR \

echo ""
echo "use process substitution, waiting until messages are in blockchain..."
fork_message_wait 104 ${MINER_UPDATE_PID_MSG_CID} ${MN_REPO_DIR}
fork_message_wait 105 ${MINER_UPDATE_PID_MSG_CID} ${CL_REPO_DIR}
fork_message_wait 106 ${MINER_ADD_ASK_MSG_CID} ${MN_REPO_DIR}
fork_message_wait 107 ${MINER_ADD_ASK_MSG_CID} ${CL_REPO_DIR}

echo ""
echo "block until miner peer id-update message appears in chains..."
join 104
join 105
join 106
join 107

echo ""
echo "client imports piece 1..."
PIECE_1_CID=$(cat ${PIECE_1_PATH} \
  | ./go-filecoin client import --repodir=${CL_REPO_DIR})

echo ""
echo "client proposes a storage deal, which transfers file 1..."
./go-filecoin client propose-storage-deal ${MN_MINER_FIL_ADDR} ${PIECE_1_CID} 0 5 \
  --repodir=$CL_REPO_DIR \

echo ""
echo "client imports piece 2..."
PIECE_2_CID=$(cat ${PIECE_2_PATH} \
  | ./go-filecoin client import --repodir=${CL_REPO_DIR})

echo ""
echo "client proposes a storage deal, which transfers piece 2 (triggers seal)..."
./go-filecoin client propose-storage-deal ${MN_MINER_FIL_ADDR} ${PIECE_2_CID} 0 5 \
  --repodir=$CL_REPO_DIR \

echo ""
echo "miner mines a block (to include commitSector message)..."
mine_once ${MN_REPO_DIR}

echo ""
echo "wait for commitSector sent by miner owner to be included in a block viewable by both nodes..."
wait_for_message_in_chain_by_method_and_sender commitSector ${MN_MINER_OWNER_FIL_ADDR} ${CL_REPO_DIR}

wait_for_message_in_chain_by_method_and_sender commitSector ${MN_MINER_OWNER_FIL_ADDR} ${MN_REPO_DIR}
echo ""
echo ""
echo ""
echo "********************** BEGIN RETRIEVAL PROTOCOL"
echo ""
echo ""
echo ""

./go-filecoin retrieval-client retrieve-piece ${MN_PEER_ID} ${PIECE_1_CID} \
  --repodir=${CL_REPO_DIR} > ${UNSEAL_PATH}



if [ "${USE_SMALL_SECTORS}" = true ] ; then
    GOT=$(cat ${UNSEAL_PATH})

    if [ "${GOT}" = "${HODL}" ]; then
        echo "Round trip passed!"
        exit 0
    else
        echo "Round trip Failed!, expected:"
        echo "${HODL}"
        echo "got:"
        echo "${GOT}"
        exit 1
    fi
else
    echo "big sector piece is too... big to cat"
    echo ""
    echo "report on unsealed file..."
    du -sh ${UNSEAL_PATH}
fi
