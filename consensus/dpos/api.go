// Copyright 2017 The ebakus/node Authors
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

package dpos

import (
	"context"

	"github.com/ebakus/go-ebakus/consensus"
	"github.com/ebakus/go-ebakus/core/rawdb"
	"github.com/ebakus/go-ebakus/core/vm"
	"github.com/ebakus/go-ebakus/rpc"
)

// API is a user facing RPC API to allow controlling the voting
// mechanisms of the delegeted proof-of-stake scheme.
type API struct {
	chain consensus.ChainReader
	dpos  *DPOS
}

func (api *API) rpcOutputWitnesses(wits *vm.WitnessArray) []interface{} {
	dels := make([]interface{}, len(*wits))

	for i, wit := range *wits {
		d := map[string]interface{}{
			"address": wit.Id,
			"stake":   wit.Stake,
		}
		dels[i] = d
	}

	return dels
}

// GetDelegates retrieves the list of delegates at the specified block.
func (api *API) GetDelegates(ctx context.Context, number rpc.BlockNumber) ([]interface{}, error) {
	header := api.chain.GetHeaderByNumber(uint64(number))
	if header == nil {
		return nil, consensus.ErrFutureBlock
	}

	ebakusSnapshotID := rawdb.ReadSnapshot(api.dpos.db, header.Hash(), header.Number.Uint64())
	ebakusState := api.dpos.ebakusDb.Snapshot(*ebakusSnapshotID)
	defer ebakusState.Release()

	delegates := GetDelegates(header.Number.Uint64(), ebakusState, api.dpos.config.DelegateCount, api.dpos.config.BonusDelegateCount, api.dpos.config.TurnBlockCount, api.dpos.blockchain.GetHeaderByNumber)

	return api.rpcOutputWitnesses(&delegates), nil
}

func (api *API) GetBlockDensity(ctx context.Context, number rpc.BlockNumber, lookbackTime uint64) (map[string]interface{}, error) {
	return api.dpos.getBlockDensity(api.chain, number, lookbackTime)
}
