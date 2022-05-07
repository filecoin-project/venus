#!/bin/sh

echo $@

if [ $nettype = "calibnet" ];then
    nettype="cali"
fi

echo $nettype
./venus daemon --network=${nettype} --auth-url=http://127.0.0.1:8989 --import-snapshot /snapshot.car
