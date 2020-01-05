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
	"enode://a551e071bbd958e355598f8005e7268085348ab1a427552d09ebb407b59f36b70f10ec93118a42e6d77a11475a10cb1807bde5276d409dc25d96a9211ae8a1f2@3.125.186.52:30103",
	"enode://6be27a098b0ac5703e172d7effe30d23fcd598a398c2e3c3ec3a7374fe5a3e26b10f9f6aa371e4c18112ad19251293e6a7aeb8545b160287f9133f3d7adfd738@3.125.186.52:30203",
	"enode://8b023729c0590dedd881e9eeacc101f44cfa1ea3aa652f27993517892dbe0831c527f82f6cd7c6631a35089580c3ac615752950dffee2fba9c95c917ea21b0cd@3.124.37.129:30303",
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
