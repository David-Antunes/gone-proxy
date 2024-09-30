# GONE-PROXY

GONE-PROXY functionality is to intercept all traffic inside of the overlay network created in docker and redirect it to unix sockets.

This application should only be executed inside of a docker network namespace since it expects certain interfaces to exist.

## Setup

First, you have to compile the application in root.

```bash
sudo go build
```

Next, you will have to create a docker overlay network.

```bash
docker network create --driver=overlay --attachable --subnet=10.1.0.0/24 net
```

To enter the namespace of the network created you have to execute the following:

```bash
NETWORK_ID=$(docker network list -f "name=net" --format "{{.ID}}")

NETWORK_LEN=${#NETWORK_ID}
ns=$((NETWORK_LEN - 2))
namespace=${NETWORK_ID:0:$ns}

sudo nsenter --net=/var/run/docker/netns/1-$namespace ./gone-proxy
```

where the NETWORK_ID is the id of the network and namespace is the network id without the last two characters.

This setup does not include the rtt program that allows gone-proxy to calculate the time between sending a packet to a given container and vice versa.

To add that feature, you have to launch the rtt container after you created the network, since gone-proxy expects the first network interface to be that specific container.

## Docker

To run this application inside of a container, you will have to build the image and run the following docker commmand:

```bash
docker run --rm --privileged -it --network none --name proxy-$(hostname) -v /var/run/docker:/var/run/docker -v /tmp:/tmp -e NAMESPACE="/var/run/docker/netns/1-$namespace" proxy
```
