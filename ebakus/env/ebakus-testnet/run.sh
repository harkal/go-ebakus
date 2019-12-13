#!/bin/bash

FULLNODE_TARGET="fullnode"
MININGNODE_TARGET="miningnode"
ATTACHNODE_TARGET="attachnode"

BIN_PATH=/opt/ebakus
SHARED_DIR=/opt/shared

EBAKUS=$BIN_PATH/ebakus

echo "Running Ebakus with target: $TARGET"

if [ $TARGET == $FULLNODE_TARGET ] || [ $TARGET == $MININGNODE_TARGET ] ; then
	if [ -z $NODE_NAME ] ; then
		echo "ERROR: Container run with empty NODE_NAME"
		exit 1
	fi

	if [ ! -z $(ls -1 $SHARED_DIR | grep $NODE_NAME) ] ; then
		echo "WARNING: Node with name $NODE_NAME already exists"
	else
		mkdir -p $SHARED_DIR/$NODE_NAME

		if [ $TARGET == $MININGNODE_TARGET ] ; then
			mkdir -p $SHARED_DIR/$NODE_NAME/keystore
			cp -R $SHARED_DIR/shared_data/keystore/. $SHARED_DIR/$NODE_NAME/keystore/
		fi
	fi

	NETWORK_ARGS="--testnet"
	if [[ ! -z $GENESIS ]] ; then
		$EBAKUS --datadir $SHARED_DIR/$NODE_NAME init $SHARED_DIR/shared_data/genesis.json

		NETWORK_ARGS=""
	fi

	MININGNODE_ARGS=""
	if [ $TARGET == $MININGNODE_TARGET ] ; then
		ADDRESS=${ETHERBASE:-'0x92f00e7fee2211d230afbe4099185efa3a516cbf'}
		echo "Setting ebakus up to mine into account: $ADDRESS"

		MININGNODE_ARGS="--mine --etherbase $ADDRESS --unlock $ADDRESS --allow-insecure-unlock --password $SHARED_DIR/shared_data/pass.txt"
	fi

	$EBAKUS --datadir $SHARED_DIR/$NODE_NAME \
		--verbosity 3 \
		--nodiscover \
		--rpcaddr "0.0.0.0" \
		--rpcapi "eth,net,web3,debug,dpos" \
		--wsaddr "0.0.0.0" --wsorigins="*" \
		--wsapi "eth,net,web3,debug,dpos" \
		$NETWORK_ARGS $MININGNODE_ARGS $@

elif [ $TARGET == $ATTACHNODE_TARGET ] ; then
	echo "Getting ready to POLL..."

	# Wait for node to become available
	if [ -z $POLL_INTERVAL ] ; then
		POLL_INTERVAL=1
	fi
	if [ -z $RETRIES ] ; then
		RETRIES=20
	fi
	CURRENT=0
	while true ; do
		if [ $CURRENT -eq $RETRIES ] ; then
			exit 1
		fi

		sleep $POLL_INTERVAL

		if [ ! -z $(ls -1 $SHARED_DIR/$NODE_NAME | grep ebakus.ipc) ] ; then
			break
		fi

		CURRENT=$[$CURRENT+1]
	done

	echo "Acquired node ipc file"

	$EBAKUS --datadir $SHARED_DIR/$NODE_NAME $@ attach

else
	echo "ERROR: Invalid TARGET=$TARGET"
	exit 1
fi