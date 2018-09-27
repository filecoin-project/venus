#!/bin/bash
set -ex

# create partition and mount instance storage
STORAGE_DISK=$$(find /dev/disk/by-id/ -name "nvme-Amazon_EC2_NVMe_Instance_Storage*")
STORAGE_PART="$${STORAGE_DISK}-part1"
STORAGE_MOUNT="/mnt/storage"
mkdir -p "$$STORAGE_MOUNT"
parted "$$STORAGE_DISK" mklabel msdos
parted "$$STORAGE_DISK" mkpart primary ext4 0% 100%
sync
while [ ! -b "$$STORAGE_PART" ]
do
  echo "Waiting for partition $$STORAGE_PART to be created" && sleep 3
done
mkfs.ext4 -F "$$STORAGE_PART"
echo "$$STORAGE_PART   $$STORAGE_MOUNT        ext4   defaults,discard        0 0" >> /etc/fstab
mount "$$STORAGE_MOUNT"
