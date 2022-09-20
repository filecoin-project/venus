#!/bin/sh

echo $@

Args=" --auth-url=http://127.0.0.1:8989 "

if [ $nettype ]
then
    Args="$Args --network=$nettype"
fi

if [ $snapshot ]
then
    Args="$Args --import-snapshot=/snapshot.car"
fi

if [ $genesisfile ]
then
    Args="$Args --genesisfile=/root/genesis.car"
fi


echo "EXEC: ./venus daemon $Args \n\n"
./venus daemon $Args &

echo "bootstrapping..."
cat /env/bootstrap
sleep 10

# connect to bootstrap
if [ -f /env/bootstrap ];then
    while [ -z "$peerID" ];do
        sleep 5
        peerID=`/app/venus swarm id`
    done
    bootstrap=`cat /env/bootstrap`
    /app/venus swarm connect $bootstrap
    echo "EXEC: ./venus swarm connect $bootstrap \n"
fi

wait
