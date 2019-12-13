// Copyright 2014 The ebakus/node Authors
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
	"encoding/json"
	"fmt"
	"math/big"
	"sort"

	"github.com/ebakus/node/common"
	"github.com/ebakus/node/consensus"
	"github.com/ebakus/node/core/types"
	"github.com/ebakus/node/ethdb"
	"github.com/ebakus/node/log"
	"github.com/ebakus/node/params"
)

const (
	// The account wants to be witness and will be considered for block producer
	// by stake delegation
	ElectEnabledFlag uint64 = 1
)

type Witness struct {
	Addr      common.Address
	Flags     uint64
	Stake     *big.Int
	VoteCount uint64
}

func newWitness(addr common.Address) *Witness {
	return &Witness{
		Addr:      addr,
		Flags:     0,
		Stake:     big.NewInt(0),
		VoteCount: 0,
	}
}

type WitnessArray []*Witness

func (s WitnessArray) Len() int { return len(s) }

func (s WitnessArray) Less(i, j int) bool {
	return s[i].Stake.Cmp(s[j].Stake) == -1
}

func (s WitnessArray) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (s WitnessArray) Diff(b WitnessArray) types.DelegateDiff {
	r := make(types.DelegateDiff, 0)

	for i, d := range b {
		found := false
		for j, from := range s {
			if from.Addr == d.Addr {
				if i != j {
					r = append(r, types.DelegateItem{Pos: byte(i), DelegateAddress: common.Address{}, DelegateNumber: byte(j)})
				}
				found = true
				break
			}
		}
		if !found {
			r = append(r, types.DelegateItem{Pos: byte(i), DelegateAddress: d.Addr, DelegateNumber: 0})
		}
	}

	return r
}

// getData returns a slice from the data based on the start and size and pads
// up to size with zero's. This function is overflow safe.
func getData(data []byte, start uint64, size uint64) []byte {
	length := uint64(len(data))
	if start > length {
		start = length
	}
	end := start + size
	if end > length {
		end = length
	}
	return common.RightPadBytes(data[start:end], int(size))
}

// State holds the information for the current delegate sealing
// procedure at a specific point in time
type State struct {
	config *params.DPOSConfig

	BlockNum  uint64
	Hash      common.Hash
	Witnesses map[common.Address]*Witness
}

func newState(config *params.DPOSConfig, blockNum uint64, hash common.Hash) *State {
	state := &State{
		config: config,

		BlockNum:  blockNum,
		Hash:      hash,
		Witnesses: make(map[common.Address]*Witness),
	}
	return state
}

func (s *State) copy() *State {
	c := &State{
		config: s.config,

		BlockNum:  s.BlockNum,
		Hash:      s.Hash,
		Witnesses: make(map[common.Address]*Witness),
	}

	for wit := range s.Witnesses {
		c.Witnesses[wit] = s.Witnesses[wit]
	}

	return c
}

func retrieve(config *params.DPOSConfig, db ethdb.Database, hash common.Hash) (*State, error) {
	blob, err := db.Get(append([]byte("dpos-"), hash[:]...))
	if err != nil {
		return nil, err
	}

	state := new(State)
	if err := json.Unmarshal(blob, state); err != nil {
		return nil, err
	}

	return state, nil
}

func (s *State) store(db ethdb.Database, hash common.Hash) error {
	blob, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return db.Put(append([]byte("dpos-"), hash[:]...), blob)
}

func (s *State) apply(chain consensus.ChainReader, header *types.Header) (*State, error) {
	if header == nil {
		return s, nil
	}

	if header.Number.Uint64() != s.BlockNum+1 {
		return nil, errInvalidStateHeaderAlignment
	}

	state := s.copy()

	number := header.Number.Uint64()

	block := chain.GetBlock(header.Hash(), number)
	if block == nil {
		log.Info("Failed GetBlock()")
		return nil, fmt.Errorf("block not found")
	}

	state.BlockNum++
	state.Hash = header.Hash()

	return state, nil
}

func (s *State) addWitness(addr common.Address, stake *big.Int) {
	w := &Witness{
		Addr:  addr,
		Stake: stake,
	}
	s.Witnesses[addr] = w
}

func (s *State) removeWitness(addr common.Address) {
	delete(s.Witnesses, addr)
}

func (s *State) getDelegates(maxDelegates int) WitnessArray {
	dels := make(WitnessArray, len(s.Witnesses))
	i := 0
	for _, w := range s.Witnesses {
		dels[i] = w
		i++
	}
	sort.Sort(dels)
	start := len(dels) - maxDelegates
	if start < 0 {
		start = 0
	}
	return dels[start:len(dels)]
}

func (s *State) logDelegates(maxDelegates int) {
	ds := s.getDelegates(maxDelegates)
	for i, d := range ds {
		log.Trace("Delegate", "i", i, "addr", d.Addr, "stake", d.Stake)
	}
}
