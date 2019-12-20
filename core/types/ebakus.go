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

package types

import (
	"encoding/binary"

	"github.com/ebakus/go-ebakus/common"
	"github.com/ebakus/go-ebakus/core/ebkdb"
	"github.com/ebakus/ebakusdb"
)

var PrecompliledSystemContract = common.BytesToAddress([]byte{1, 1})
var PrecompliledDBContract = common.BytesToAddress([]byte{1, 2})

// EspilonStake for calculating virtual difficulty
const EspilonStake = 1e-10

const (
	SystemStakeDBKey = "ebk:global:systemStake"
)

type Staked struct {
	Id     common.Address // Owner account
	Amount uint64
}

var StakedTable = ebkdb.GetDBTableName(PrecompliledSystemContract, "Staked")

func VirtualCapacity(from common.Address, ebakusState *ebakusdb.Snapshot) float64 {
	accountStaked := uint64(0)
	var staked Staked

	where := []byte("Id LIKE ")
	if whereClause, err := ebakusState.WhereParser(append(where, from.Bytes()...)); err == nil {
		if iter, err := ebakusState.Select(StakedTable, whereClause); err == nil {
			if iter.Next(&staked) {
				accountStaked = staked.Amount
			}
		}
	}

	systemStaked := uint64(0)
	if systemStakedBytes, found := ebakusState.Get([]byte(SystemStakeDBKey)); found {
		systemStaked = binary.BigEndian.Uint64(*systemStakedBytes)
	}

	return (EspilonStake + float64(accountStaked)) / (EspilonStake + float64(systemStaked))
}
