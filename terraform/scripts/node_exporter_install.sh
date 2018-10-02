#!/bin/bash

set -ex

docker run -d \
       --net="host" \
       --pid="host" \
       quay.io/prometheus/node-exporter
