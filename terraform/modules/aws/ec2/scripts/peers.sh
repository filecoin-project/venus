#!/bin/bash

for i in {0..4}
do
  printf "\nfilecoin_$$i: "
  sudo docker exec "filecoin-$$i" go-filecoin --repodir=/var/local/filecoin id --format=\<addrs\> | uniq | grep 172\.19 | \
    sed "s|172.19.0.\+/tcp/9000/|${instance_name}.kittyhawk.wtf/tcp/900$${i}/|g"
done
