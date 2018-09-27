#!/bin/bash

set -ex

NGINX_PROMETHEUS_CONFIG=$$(cat <<-'END'
server {
  listen 9090;
  server_name prometheus.kittyhawk.wtf;
  location / {
    auth_basic "Restricted Content";
    auth_basic_user_file /etc/nginx/prometheus.httpasswd;
    proxy_pass http://prometheus:9090;
    proxy_set_header X-Forwarded-For $$remote_addr;
  }
}

server {
  listen 9093;
  server_name alertmanager.kittyhawk.wtf;
  location / {
    auth_basic "Restricted Content";
    auth_basic_user_file /etc/nginx/alertmanager.httpasswd;
    proxy_pass http://alertmanager:9093;
    proxy_set_header X-Forwarded-For $$remote_addr;
  }
}
END
)
echo "$$NGINX_PROMETHEUS_CONFIG" > /home/ubuntu/prometheus.conf

PROMETHEUS_HTTPASSWD=$$(cat <<-'END'
${prometheus_httpasswd}
END
)
echo "$$PROMETHEUS_HTTPASSWD" > /home/ubuntu/prometheus.httpasswd

ALERTMANAGER_HTTPASSWD=$$(cat <<-'END'
${alertmanager_httpasswd}
END
)
echo "$$ALERTMANAGER_HTTPASSWD" > /home/ubuntu/alertmanager.httpasswd


#mount instance storage
STORAGE_DISK=$$(find /dev/disk/by-id/ -name "nvme-Amazon_EC2_NVMe_Instance_Storage*")
STORAGE_PART="$${STORAGE_DISK}-part1"
STORAGE_MOUNT="/mnt/storage"
PROMETHEUS_STORAGE="$$STORAGE_MOUNT/prometheus"
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
# expose prometheus:9090 & alertmanager:80
docker run -d \
       --name nginx \
       -v /home/ubuntu/prometheus.conf:/etc/nginx/conf.d/prometheus.conf:ro \
       -v /home/ubuntu/prometheus.httpasswd:/etc/nginx/prometheus.httpasswd:ro \
       -v /home/ubuntu/alertmanager.httpasswd:/etc/nginx/alertmanager.httpasswd:ro \
       --network=metrics \
       -p 9090:9090 -p 80:9093 \
       nginx
