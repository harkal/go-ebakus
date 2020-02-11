// Copyright 2019 The ebakus/go-ebakus Authors
// This file is part of the ebakus/go-ebakus library.
//
// The ebakus/go-ebakus library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The ebakus/go-ebakus library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the ebakus/go-ebakus library. If not, see <http://www.gnu.org/licenses/>.

package params

// Ebakus MainnetBootnodes are the enode URLs of the P2P bootstrap nodes running on
// the main Ebakus network.
var MainnetBootnodes = []string{
	"enode://48cd968e7b7fe800f2fc65e39e913bd3c2dbba61d27d7bd741168f05410905457affe439025250e313fe5a3ae4217bdc785adb481c45ad0c4cb934b89735b06a@35.159.1.5:30401",
	"enode://3d9e784fd3370efc1af680b0bc9af2e88faad1976afb45acda541a3b46eca6e816759436240fe0c51ac0b90dc771dbfa5d8ccb855afab6ac6b3da2b5bf5fbd7a@3.126.72.30:30401",
	"enode://bc31820f82bb339534eb12b3ffd60ff5d033dcbbc57e807dd503c42a9cc14b40124c135e575593c54af15972c449c5bce8ceb956b0f43abc6ad1bc986f741358@3.133.198.232:30401",
	"enode://102f4801f8fc25eb1a5a867dc9e8c1b972afbcf310c720186365b88bbf455c8d3b43cc2f9e64484fdafea5e1822bab9ee456ca7d99dbe29f7990b16220ad6f4a@3.12.91.81:30401",
	"enode://bacced7168d0f0f429810ac9c826bf91de50414f5413bc57dde4962326230a5b7a5a9d39ea8fde66f31955a0bb08ef6a90549ad0dace3c21d23701cc5cbd5f75@3.12.42.195:30401",
	"enode://9b241ccf4ff3b828e266a381dea77e08e97362c4ec7d2ee122a7b66ffd8f01923e508c9cc5a6d16224c922c841e659cbad6099cb15d0781c7409b49f164ed63c@3.126.58.129:30401",
}

// TestnetBootnodes are the enode URLs of the P2P bootstrap nodes running on the
// test network.
var TestnetBootnodes = []string{
	"enode://bb49e583ad80b09cb6fbb294be31317b254df64457267a6de4ec7288c4cfaec89cd9e33077f761847af67905cecb96deb1ab019c3e85effc286045197345b424@172.104.148.4:30103",
	"enode://1e60196407dd57d8de4b8eda7d358ecf7a13dd9c92756a3c3f66ed9917d4f424eb2c174acd1ee6e993eef89d8e4f4d7c8a84f8a4cc5008578970b151f9140460@172.104.148.4:30203",
	"enode://794df227b077e3c5fc80694a3927b7b5fb5fde8115b14bb16d339ac836372aaa427f91bd5930f9365a8ab44369b195ccbeb833f92d78836d4ace3bd69de59de2@172.104.148.4:30303",
}

// DiscoveryV5Bootnodes are the enode URLs of the P2P bootstrap nodes for the
// experimental RLPx v5 topic-discovery network.
var DiscoveryV5Bootnodes = []string{}
