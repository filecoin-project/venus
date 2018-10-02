#!/bin/bash
set -ex
STORAGE_MOUNT="/mnt/storage"

hdds=()
for drive in /dev/xvd[b-z]
do
  hdds=("$${hdds[@]}" "$$drive")
done

pvcreate "$${hdds[@]}"
vgcreate LVMFilecoinVolGroup "$${hdds[@]}"
lvcreate -l 100%FREE -n storage LVMFilecoinVolGroup
mkfs.ext4 /dev/LVMFilecoinVolGroup/storage
mkdir -p "$$STORAGE_MOUNT"
echo "/dev/LVMFilecoinVolGroup/storage   $$STORAGE_MOUNT        ext4   defaults,discard        0 0" >> /etc/fstab
mount "$$STORAGE_MOUNT"
