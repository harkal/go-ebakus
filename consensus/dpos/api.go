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

package dpos

import (
	"context"
	"fmt"

	"github.com/ebakus/go-ebakus/common"
	"github.com/ebakus/go-ebakus/consensus"
	"github.com/ebakus/go-ebakus/core/rawdb"
	"github.com/ebakus/go-ebakus/core/types"
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
	var header *types.Header
	if number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number))
	}

	if header == nil {
		return nil, consensus.ErrFutureBlock
	}

	ebakusSnapshotID := rawdb.ReadSnapshot(api.dpos.db, header.Hash(), header.Number.Uint64())
	ebakusState := api.dpos.ebakusDb.Snapshot(*ebakusSnapshotID)
	defer ebakusState.Release()

	delegates := GetDelegates(header, ebakusState, api.dpos.config.DelegateCount, api.dpos.config.BonusDelegateCount, api.dpos.config.TurnBlockCount)

	return api.rpcOutputWitnesses(&delegates), nil
}

// GetDelegate get delegate.
func (api *API) GetDelegate(ctx context.Context, address common.Address, number rpc.BlockNumber) (map[string]interface{}, error) {
	var header *types.Header
	if number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number))
	}

	if header == nil {
		return nil, consensus.ErrFutureBlock
	}

	ebakusSnapshotID := rawdb.ReadSnapshot(api.dpos.db, header.Hash(), header.Number.Uint64())
	ebakusState := api.dpos.ebakusDb.Snapshot(*ebakusSnapshotID)
	defer ebakusState.Release()

	var witness vm.Witness

	where := []byte("Id LIKE ")
	whereClause, err := ebakusState.WhereParser(append(where, address.Bytes()...))
	if err != nil {
		return nil, fmt.Errorf("Ebakusdb query error")
	}

	iter, err := ebakusState.Select(vm.WitnessesTable, whereClause)
	if err != nil {
		return nil, fmt.Errorf("Ebakusdb query error")
	}

	if iter.Next(&witness) == false {
		return nil, fmt.Errorf("Address is not a delegate")
	}

	out := map[string]interface{}{
		"address": witness.Id,
		"stake":   witness.Stake,
		"elected": (witness.Flags & vm.ElectEnabledFlag) == 1,
	}

	return out, nil
}

func (api *API) GetBlockDensity(ctx context.Context, number rpc.BlockNumber, lookbackTime uint64) (map[string]interface{}, error) {
	return api.dpos.getBlockDensity(api.chain, number, lookbackTime)
}
