#!/bin/bash

set -ex

while fuser /var/lib/dpkg/lock >/dev/null 2>&1 ; do
  sleep 0.5
done

passwd -d root

DEBIAN_FRONTEND=noninteractive apt-get purge -qy docker docker-engine docker.io || echo "Purged old versions of docker"
DEBIAN_FRONTEND=noninteractive apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y apt-transport-https ca-certificates curl software-properties-common awscli golang-go jq

# add Docker gpg key and repo
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
DEBIAN_FRONTEND=noninteractive \
               add-apt-repository \
               "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
                $$(lsb_release -cs) stable"

DEBIAN_FRONTEND=noninteractive apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y docker-ce

LOGSTASH_DOCKER_IMAGE=${logstash_docker_uri}:${logstash_docker_tag}
eval $$(aws --region us-east-1 ecr --no-include-email get-login)
docker pull $${LOGSTASH_DOCKER_IMAGE}

# start logstash
docker run \
       --log-driver json-file --log-opt max-size=10m \
       -d -p 5044:5044 --name logstash -e ES_HOST=${es_host} $${LOGSTASH_DOCKER_IMAGE}
