#!/bin/bash

set -ex

${docker_install}
LOGSTASH_DOCKER_IMAGE=${logstash_docker_uri}:${logstash_docker_tag}
eval $$(aws --region us-east-1 ecr --no-include-email get-login)
docker pull $${LOGSTASH_DOCKER_IMAGE}

# start logstash
docker run \
       --log-driver json-file --log-opt max-size=10m \
       -d -p 5044:5044 --name logstash -e ES_HOST=${es_host} $${LOGSTASH_DOCKER_IMAGE}
