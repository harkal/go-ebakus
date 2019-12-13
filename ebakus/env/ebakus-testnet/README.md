# Ebakus

Dockerized Ebakus testnets

- - -

## Table of Contents

- [Ebakus](#ebakus)
  - [Table of Contents](#table-of-contents)
  - [Requirements](#requirements)
- [Environment variables](#environment-variables)
  - [Network up](#network-up)
  - [Network topology and configuration](#network-topology-and-configuration)
  - [Working with nodes from docker host](#working-with-nodes-from-docker-host)
    - [JSON-RPC access with `ebakus`](#json-rpc-access-with-ebakus)
    - [IPC access with `ebakus`](#ipc-access-with-ebakus)
  - [Working with a node from its container](#working-with-a-node-from-its-container)
    - [Example: Checking a node's `admin.peers`](#example-checking-a-nodes-adminpeers)
  - [Clean up](#clean-up)


> Credits to: https://github.com/the-chaingang/ethereal

## Requirements

+ [Docker](https://www.docker.com/get-docker)
+ [Docker compose](https://docs.docker.com/compose/)

# Environment variables

```commandline
export EBAKUS_CONTAINER_NAME=ebakus/node
export EBAKUS_CONTAINER_TAG=latest
```

## Network up

At first we have to add the keystore files for our producer accounts under `ebakus/env/ebakus-testnet/shared_data/keystore`.

To bring up your ebakus network, you first have to build the image:

```commandline
cd $GOPATH/src
docker build -t ${EBAKUS_CONTAINER_NAME} -f github.com/ebakus/node/ebakus/env/ebakus-testnet/Dockerfile .
```

and then start it by running:

```commandline
cd $GOPATH/src/github.com/ebakus/node/ebakus/env/ebakus-testnet
docker-compose up
```

## Network topology and configuration

Our default network consists of two mining nodes connected to each other.

The network is specified in [docker-compose.yaml](./docker-compose.yaml). If you would like to deploy a network with a different topology, this is the place to start.


## Working with nodes from docker host

### JSON-RPC access with `ebakus`

By default, we expose the [JSON-RPC interface](https://github.com/ethereum/wiki/wiki/JSON-RPC). To attach to this from your host machine, run

```commandline
ebakus attach http://localhost:${NODE_RPC_PORT}
```

Here, `$NODE_RPC_PORT` refers to the host port specified for your desired node when you brought up
your network. For our [sample network](./docker-compose.yaml), you can use `NODE_RPC_PORT=8545` for
`node-1` or `NODE_RPC_PORT=8645` for `node-2`.


### IPC access with `ebakus`

You can also use the IPC API to access administrative functions on the node. To do this from your
host machine, you will have to access the local mount point for your `ebakus` network's `shared`
volume. You can see the path to this mount point using:

```commandline
docker volume inspect ebakus-testnet_shared -f "{{.Mountpoint}}"
```

Note that your `$USER` probably does not have permissions to access this directory and so any
commands run against this directory will have to be run with superuser privileges.

To attach to the IPC interface for your desired node, simply run

```commandline
sudo ebakus attach ipc:$(docker volume inspect ebakus-testnet_shared -f "{{.Mountpoint}}")/$NODE_NAME/ebakus.ipc
```

Here, the `$NODE_NAME` specifies which node you are interacting with. For our
[sample network](./docker-compose.yaml), if you want to connect to `node-1`, you would set
`NODE_NAME=node-1`.

Note that if you built `ebakus` from source, it probably won't be available under your superuser
`PATH` and you may therefore, in the above command, have to specify the path to your `ebakus` binary instead.


## Working with a node from its container

For administrative functions especially, you are better off working with a node from its container.
You can do so by running a `bash` process on that container:

```commandline
docker exec -it $NODE_CONTAINER bash
```

Here, `$NODE_CONTAINER` specifies the name of the container running the desired node. For our
[sample network](./docker-compose.yaml), you can specify `NODE_CONTAINER=ebakus-testnet_node-1_1` to work with `node-1`.

From this shell, run

```commandline
/opt/ebakus/ebakus attach ipc:/opt/shared/$NODE_NAME/ebakus.ipc
```

for access to the IPC console. Note that you will have to use the appropriate `$NODE_NAME` and also
that you can access the `ebakus` console for any other node from this container (as a matter of convenience).


### Example: Checking a node's `admin.peers`

In the [sample network](./docker-compose.yaml), attach to the `node-1` container using:

```commandline
docker exec -it ebakus-testnet_node-1_1 bash
```

Attach a `ebakus` console to the node using:

```commandline
/opt/ebakus/ebakus attach ipc:/opt/shared/$NODE_NAME/ebakus.ipc
```

In the ebakus console, run:

```commandline
admin.peers
```

## Clean up

In order to cleanup the containers and the volume to start up fresh run:

```commandline
docker ps -a | grep ebakus-testnet | awk '{print $1}' | xargs docker rm
docker volume rm ebakus-testnet_shared
```