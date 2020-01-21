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
	"enode://c65356c032b3acbcd156949d872f5e8e3abd1b70f2d44038cf0fa0738e7bb6552f25ff941d34de706f64434a234fcfc147e90a4d666d1ffce7d357a740d2a019@35.159.1.5:30402",
	"enode://100e4e51a268c51ae826a55f6e93bdc6ce630d23bb7ce898475f3d0944fe382aab2d0da0be0f5a8331bb7aed6d6cc8e015f9a7207e8eb6e1aa293d4dbb7121bc@35.159.1.5:30403",
	"enode://bd2b32e400437046d3b9f4914c2053c998396f95c45bd904a2682613a12eb4fc23cc11c086b148161262a5a49d019bdc942b6e1fd32c9c70ba9e360ef6ad211d@35.159.1.5:30404",
	"enode://3d9e784fd3370efc1af680b0bc9af2e88faad1976afb45acda541a3b46eca6e816759436240fe0c51ac0b90dc771dbfa5d8ccb855afab6ac6b3da2b5bf5fbd7a@3.126.72.30:30401",
	"enode://b26f25852de68259704f089403ee51acb3918f2177bb3e18f34b3f083b8138d470da22003366fea636d8908472f1c4d3621fad4fe8851cd5157b1734838f0bb4@3.126.72.30:30402",
	"enode://a5a2e457e07bd197d3de1ed55487fbe58a58e338b8d8254fe50b9bbddee548b30682d0accf2205eebe24430f9b7f06f0d2b321fcf83b10fa78b9bb45fb0b34f3@3.126.72.30:30403",
}

// TestnetBootnodes are the enode URLs of the P2P bootstrap nodes running on the
// test network.
var TestnetBootnodes = []string{
	"enode://bb49e583ad80b09cb6fbb294be31317b254df64457267a6de4ec7288c4cfaec89cd9e33077f761847af67905cecb96deb1ab019c3e85effc286045197345b424@172.105.65.124:30103",
	"enode://1e60196407dd57d8de4b8eda7d358ecf7a13dd9c92756a3c3f66ed9917d4f424eb2c174acd1ee6e993eef89d8e4f4d7c8a84f8a4cc5008578970b151f9140460@172.105.65.124:30203",
	"enode://794df227b077e3c5fc80694a3927b7b5fb5fde8115b14bb16d339ac836372aaa427f91bd5930f9365a8ab44369b195ccbeb833f92d78836d4ace3bd69de59de2@172.105.65.124:30303",
}

// DiscoveryV5Bootnodes are the enode URLs of the P2P bootstrap nodes for the
// experimental RLPx v5 topic-discovery network.
var DiscoveryV5Bootnodes = []string{}
