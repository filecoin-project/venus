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
./venus daemon $Args
