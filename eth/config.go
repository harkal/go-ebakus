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

package eth

import (
	"math/big"
	"os"
	"os/user"
	"time"

	"github.com/ebakus/go-ebakus/common"
	"github.com/ebakus/go-ebakus/core"
	"github.com/ebakus/go-ebakus/core/types"
	"github.com/ebakus/go-ebakus/eth/downloader"
	"github.com/ebakus/go-ebakus/eth/gasprice"
	"github.com/ebakus/go-ebakus/miner"
	"github.com/ebakus/go-ebakus/params"
)

// DefaultConfig contains default settings for use on the Ebakus main net.
var DefaultConfig = Config{
	SyncMode:                   downloader.FullSync,
	DPOS:                       *params.MainnetDPOSConfig,
	NetworkId:                  params.MainnetChainConfig.ChainID.Uint64(),
	LightPeers:                 100,
	UltraLightFraction:         75,
	DatabaseCache:              768,
	TrieCleanCache:             256,
	TrieDirtyCache:             256,
	TrieTimeout:                60 * time.Minute,
	EbakusdbMaxActiveIterators: 1000,
	Miner: miner.Config{
		GasFloor: 80000000,
		GasCeil:  160000000,
		GasPrice: types.MinimumTargetDifficulty,
		Recommit: 3 * time.Second,
	},
	TxPool: core.DefaultTxPoolConfig,
	GPO: gasprice.Config{
		Blocks:     20,
		Percentile: 60,
	},
}

func init() {
	home := os.Getenv("HOME")
	if home == "" {
		if user, err := user.Current(); err == nil {
			home = user.HomeDir
		}
	}
}

//go:generate gencodec -type Config -formats toml -out gen_config.go

type Config struct {
	// The genesis block, which is inserted if the database is empty.
	// If nil, the Ebakus main net block is used.
	Genesis *core.Genesis `toml:",omitempty"`

	// Protocol options
	NetworkId uint64 // Network ID to use for selecting peers to connect to
	SyncMode  downloader.SyncMode

	NoPruning  bool // Whether to disable pruning and flush everything to disk
	NoPrefetch bool // Whether to disable prefetching and only load state on demand

	// Whitelist of required block number -> hash values to accept
	Whitelist map[uint64]common.Hash `toml:"-"`

	// Light client options
	LightServ    int `toml:",omitempty"` // Maximum percentage of time allowed for serving LES requests
	LightIngress int `toml:",omitempty"` // Incoming bandwidth limit for light servers
	LightEgress  int `toml:",omitempty"` // Outgoing bandwidth limit for light servers
	LightPeers   int `toml:",omitempty"` // Maximum number of LES client peers

	// Ultra Light client options
	UltraLightServers      []string `toml:",omitempty"` // List of trusted ultra light servers
	UltraLightFraction     int      `toml:",omitempty"` // Percentage of trusted servers to accept an announcement
	UltraLightOnlyAnnounce bool     `toml:",omitempty"` // Whether to only announce headers, or also serve them

	// Database options
	SkipBcVersionCheck bool `toml:"-"`
	DatabaseHandles    int  `toml:"-"`
	DatabaseCache      int
	DatabaseFreezer    string

	TrieCleanCache int
	TrieDirtyCache int
	TrieTimeout    time.Duration

	EbakusdbMaxActiveIterators uint64 // Maximum number of ebakusDb iterators to retain in memory for RPC APIs

	// Mining options
	Miner miner.Config

	// DPOS options
	DPOS params.DPOSConfig

	// Transaction pool options
	TxPool core.TxPoolConfig

	// Gas Price Oracle options
	GPO gasprice.Config

	// Enables tracking of SHA3 preimages in the VM
	EnablePreimageRecording bool

	// Miscellaneous options
	DocRoot string `toml:"-"`

	// Type of the EWASM interpreter ("" for default)
	EWASMInterpreter string

	// Type of the EVM interpreter ("" for default)
	EVMInterpreter string

	// RPCGasCap is the global gas cap for eth-call variants.
	RPCGasCap *big.Int `toml:",omitempty"`

	// Checkpoint is a hardcoded checkpoint which can be nil.
	Checkpoint *params.TrustedCheckpoint `toml:",omitempty"`

	// CheckpointOracle is the configuration for checkpoint oracle.
	CheckpointOracle *params.CheckpointOracleConfig `toml:",omitempty"`

	// Istanbul block override (TODO: remove after the fork)
	OverrideIstanbul *big.Int
}
