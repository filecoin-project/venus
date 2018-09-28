#!/bin/bash

for i in {0..9}
do
  printf "\nfilecoin_$$i: "
  sudo docker exec "filecoin_$$i" go-filecoin --repodir=/var/local/filecoin id --format=\<addrs\> | uniq | grep 172\.17 | \
    sed "s|172.17.0.\+/tcp/9000/|${instance_name}.kittyhawk.wtf/tcp/900$${i}/|g"
done
