#!/bin/bash

set -ex

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
  echo "Waiting for partition $$STORAGE_PART to be created" && sleep 1
done
mkfs.ext4 -F "$$STORAGE_PART"
echo "$$STORAGE_PART   $$STORAGE_MOUNT        ext4   defaults,discard        0 0" >> /etc/fstab
mount "$$STORAGE_MOUNT"
mkdir -p "$$PROMETHEUS_STORAGE"
chown -R nobody "$$PROMETHEUS_STORAGE"

${docker_install}
${cadvisor_install}
${node_exporter_install}

# login to ECR
eval $$(aws --region us-east-1 ecr --no-include-email get-login)

# start prometheus
docker network create metrics
docker run -d --restart always \
       --name prometheus --hostname prometheus --network metrics \
       -p 9091:9090 \
       -v /mnt/storage/prometheus:/prometheus \
       657871693752.dkr.ecr.us-east-1.amazonaws.com/prometheus

# start alert manager
docker run -d \
       -p 9093:9093 \
       --name alertmanager --hostname=alertmanager --network metrics \
       -e SLACK_API_URL=${alerts_slack_api_url} \
       657871693752.dkr.ecr.us-east-1.amazonaws.com/alertmanager

# start nginx container
