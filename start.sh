#!/bin/bash

if [ -z "$NETWORK" ];
  then
    echo "NETWORK env variable not set"
    exit 1
fi

NETWORK_LEN=${#NETWORK}
NS=$((NETWORK_LEN - 2))
NAMESPACE_PATH="/var/run/docker/netns/1-${NETWORK:0:$NS}"

ulimit -m unlimited
nsenter --net=$NAMESPACE_PATH "./gone-proxy"
