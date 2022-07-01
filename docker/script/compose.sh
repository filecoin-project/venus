#!/bin/sh

echo $@

if [ $nettype = "calibnet" ];then
    nettype="cali"
fi

echo "nettype:"
echo $nettype

if [ -f "/snapshot.car"  ]
then
    echo "snapshot.car exists"
    ./venus daemon --network=${nettype} --auth-url=http://127.0.0.1:8989 --import-snapshot /snapshot.car
else
    echo "snapshot.car not found"
    ./venus daemon --network=${nettype} --auth-url=http://127.0.0.1:8989 
