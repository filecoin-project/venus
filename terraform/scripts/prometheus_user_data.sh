#!/bin/bash

set -ex

${docker_install}
${node_exporter_install}
#mount instance storage
STORAGE_DISK=$$(find /dev/disk/by-id/ -name "nvme-Amazon_EC2_NVMe_Instance_Storage*")
STORAGE_PART="$${STORAGE_DISK}-part1"
STORAGE_MOUNT="/mnt/storage"
PROMETHEUS_STORAGE="$$STORAGE_MOUNT/prometheus"
mkdir -p "$$STORAGE_MOUNT"
parted "$$STORAGE_DISK" mklabel msdos
parted "$$STORAGE_DISK" mkpart primary ext4 0% 100%
while [ ! -b "$$STORAGE_PART" ]
do
  echo "Waiting for partition $$STORAGE_PART to be created" && sleep 0.5
done
mkfs.ext4 -F "$$STORAGE_PART"
echo "$$STORAGE_PART   $$STORAGE_MOUNT        ext4   defaults,discard        0 0" >> /etc/fstab
mount "$$STORAGE_MOUNT"
mkdir -p "$$PROMETHEUS_STORAGE"
chown -R nobody "$$PROMETHEUS_STORAGE"

# login to ECR
eval $(aws --region us-east-1 ecr --no-include-email get-login)

# start prometheus
docker run --name prometheus \
       -p 9090:9090 \
       -v /mnt/storage/prometheus:/prometheus \
       657871693752.dkr.ecr.us-east-1.amazonaws.com/prometheus

# start alert manager

