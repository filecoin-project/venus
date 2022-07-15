#!/bin/sh

echo $@

Args=" --auth-url=http://127.0.0.1:8989 "

if [ $nettype ]
then 
    if [ $nettype = "calibnet" ]
    then
    nettype="cali"
    fi

    Args="$Args --network=$nettype"
fi

if [ $snapshot ]
then
    Args="$Args --import-snapshot=/snapshot.car"
fi

if [ $genesisfile ]
then
    Args="$Args --genesisfile=/genesis.gen"
fi


echo "EXEC: ./venus daemon $Args"
echo 
echo

./venus daemon $Args
