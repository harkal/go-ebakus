// Copyright 2019 The ebakus/node Authors
// This file is part of the ebakus/node library.
//
// The ebakus/node library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The ebakus/node library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the ebakus/node library. If not, see <http://www.gnu.org/licenses/>.

var abi = [
  {
    type: 'function',
    name: 'stake',
    inputs: [
      {
        name: 'amount',
        type: 'uint64',
      },
    ],
    outputs: [],
    stateMutability: 'nonpayable',
  },
  {
    type: 'function',
    name: 'getStaked',
    inputs: [],
    outputs: [
      {
        type: 'uint64',
      },
    ],
    constant: true,
    payable: false,
    stateMutability: 'view',
  },
  {
    type: 'function',
    name: 'unstake',
    inputs: [
      {
        name: 'amount',
        type: 'uint64',
      },
    ],
    outputs: [],
    stateMutability: 'nonpayable',
  },
  {
    type: 'function',
    name: 'claim',
    inputs: [],
    outputs: [],
    stateMutability: 'nonpayable',
  },
  {
    type: 'function',
    name: 'vote',
    inputs: [
      {
        name: 'addresses',
        type: 'address[]',
      },
    ],
    outputs: [],
    stateMutability: 'nonpayable',
  },
  {
    type: 'function',
    name: 'unvote',
    inputs: [],
    outputs: [],
    stateMutability: 'nonpayable',
  },
  {
    type: 'function',
    name: 'electEnable',
    inputs: [
      {
        name: 'enable',
        type: 'bool',
      },
    ],
    outputs: [],
    stateMutability: 'nonpayable',
  },
  {
    type: 'function',
    name: 'storeAbiForAddress',
    inputs: [
      {
        name: 'address',
        type: 'address',
      },
      {
        name: 'abi',
        type: 'string',
      },
    ],
    outputs: [],
    stateMutability: 'nonpayable',
  },
  {
    type: 'function',
    name: 'getAbiForAddress',
    inputs: [
      {
        name: 'address',
        type: 'address',
      },
    ],
    outputs: [
      {
        type: 'string',
      },
    ],
    constant: true,
    payable: false,
    stateMutability: 'view',
  },
];

var systemContractAddress = '0x0000000000000000000000000000000000000101';
var systemContract = eth.contract(abi).at(systemContractAddress);

function commonInit() {
  // Connect node-1
  admin.addPeer(
    'enode://c39d8d56a48eddcadd07fbc3ff78f083a09c47b3a91c0513e1d88b508a26abfdd219432cf1a73d558266231469fed7a5197318c41be6bc26f8dddd2323db5e69@13.3.7.3:30403'
  );

  // Connect node-2
  admin.addPeer(
    'enode://3fd2322b06601601f90cc0f5d86c35150591bda11e3f67279143812380c3d0e7074dea825cc8821a3464b83d3b78e3f1c2360d78c56cdae419e1b5172c182f5c@13.3.7.4:30403'
  );

  // Connect node-3
  admin.addPeer(
    'enode://bd8e8be5316e6b718cc879a284d7b47bcebe8bbe7d5889e6efafbb7b378f5461cc33d556e5f307747f80ba4e86b7ce3bdc934e0f41ce03ca1713b737346d0917@13.3.7.5:30403'
  );
}

function initBootProducer() {
  systemContract.stake(5 * 10000);
  systemContract.vote([
    '0x4ed2aca4f9b3ee904ff44f03e91fde90829a8381',
    '0x92f00e7fee2211d230afbe4099185efa3a516cbf',
    '0xbc391b1dbc5fb11962b8fb419007b902e0aa6bf5',
  ]);
}

function initRestNodes() {
  var found = false;
  var delegates = eth.getDelegates(eth.blockNumber);
  for (var i = 0; i < delegates.length; i++) {
    if (delegates[i].address == eth.coinbase) {
      found = true;
      continue;
    }
  }
  console.log('TCL: initRestNodes -> found', found);
  if (!found) {
    systemContract.electEnable(true);

    admin.sleep(25);
    systemContract.stake(5 * 10000);
    systemContract.vote([
      '0x4ed2aca4f9b3ee904ff44f03e91fde90829a8381',
      '0x92f00e7fee2211d230afbe4099185efa3a516cbf',
      '0xbc391b1dbc5fb11962b8fb419007b902e0aa6bf5',
    ]);
  }
}

commonInit();

try {
  console.log('TCL: eth.blockNumber', eth.blockNumber, eth.coinbase);

  if (eth.coinbase == '0x4ed2aca4f9b3ee904ff44f03e91fde90829a8381') {
    console.log('Init node-1 starting!');
    eth.defaultAccount = eth.coinbase;

    admin.sleep(20);
    initBootProducer();
    console.log('Init node-1 done!');
  } else if (
    [
      '0x92f00e7fee2211d230afbe4099185efa3a516cbf',
      '0xbc391b1dbc5fb11962b8fb419007b902e0aa6bf5',
    ].indexOf(eth.coinbase) > -1
  ) {
    console.log('Init node starting!');
    eth.defaultAccount = eth.coinbase;

    admin.sleep(5);
    initRestNodes();
    console.log('Init node done!');
  }
} catch (e) {
  console.error('This is not a producer', e);
}
