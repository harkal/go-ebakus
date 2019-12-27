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

package vm

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math/big"
	"strings"
	"unsafe"

	"github.com/ebakus/ebakusdb"
	"github.com/ebakus/go-ebakus/accounts/abi"
	"github.com/ebakus/go-ebakus/common"
	"github.com/ebakus/go-ebakus/common/math"
	"github.com/ebakus/go-ebakus/core/ebkdb"
	"github.com/ebakus/go-ebakus/core/types"
	"github.com/ebakus/go-ebakus/crypto"
	"github.com/ebakus/go-ebakus/crypto/blake2b"
	"github.com/ebakus/go-ebakus/crypto/bn256"
	"github.com/ebakus/go-ebakus/log"
	"github.com/ebakus/go-ebakus/params"
	"golang.org/x/crypto/ripemd160"
)

// PrecompiledContract is the basic interface for native Go contracts. The implementation
// requires a deterministic gas count based on the input size of the Run method of the
// contract.
type PrecompiledContract interface {
	RequiredGas(input []byte) uint64                                // RequiredPrice calculates the contract gas use
	Run(evm *EVM, contract *Contract, input []byte) ([]byte, error) // Run runs the precompiled contract
}

// PrecompiledContractsEbakus contains the default set of pre-compiled Ebakus
// contracts used in the Ebakus blockchain.
var PrecompiledContractsEbakus = map[common.Address]PrecompiledContract{
	common.BytesToAddress([]byte{1}): &ecrecover{},
	common.BytesToAddress([]byte{2}): &sha256hash{},
	common.BytesToAddress([]byte{3}): &ripemd160hash{},
	common.BytesToAddress([]byte{4}): &dataCopy{},
	common.BytesToAddress([]byte{5}): &bigModExp{},
	common.BytesToAddress([]byte{6}): &bn256AddIstanbul{},
	common.BytesToAddress([]byte{7}): &bn256ScalarMulIstanbul{},
	common.BytesToAddress([]byte{8}): &bn256PairingIstanbul{},
	common.BytesToAddress([]byte{9}): &blake2F{},
	types.PrecompliledSystemContract: &systemContract{},
	types.PrecompliledDBContract:     &dbContract{},
}

// RunPrecompiledContract runs and evaluates the output of a precompiled contract.
func RunPrecompiledContract(evm *EVM, p PrecompiledContract, input []byte, contract *Contract) (ret []byte, err error) {
	db := evm.EbakusState
	preUsedMemory := db.GetUsedMemory()

	minimumGas := p.RequiredGas(input)
	if contract.Gas < minimumGas {
		return nil, ErrOutOfGas
	}
	ret, err = p.Run(evm, contract, input)

	postUsedMemory := db.GetUsedMemory()
	usedMemoryGas := minimumGas
	usedMemory := int64(postUsedMemory - preUsedMemory)

	if usedMemory < 0 {
		usedMemory = 0
	}

	usedMemoryGas += (uint64(usedMemory) * params.EbakusDBMemoryUsageGas)

	if !contract.UseGas(usedMemoryGas) {
		return nil, ErrOutOfGas
	}

	return
}

const (
	SystemContractStakeCmd     = "stake"
	SystemContractGetStakedCmd = "getStaked"
	SystemContractUnstakeCmd   = "unstake"
	SystemContractClaimCmd     = "claim"

	SystemContractVoteCmd        = "vote"
	SystemContractUnvoteCmd      = "unvote"
	SystemContractElectEnableCmd = "electEnable"

	SystemContractStoreAbiCmd = "storeAbiForAddress"
	SystemContractGetAbiCmd   = "getAbiForAddress"

	DBContractCreateTableCmd = "createTable"
	DBContractInsertObjCmd   = "insertObj"
	DBContractDeleteObjCmd   = "deleteObj"
	DBContractGetCmd         = "get"
	DBContractSelectCmd      = "select"
	DBContractNextCmd        = "next"
)

const (
	maxClaimableEntries  = 5
	unstakeVestingPeriod = 60 * 60 * 24 * 3 // (3 days) Number of seconds taken for tokens to become claimable
)

var (
	valueDecimalPoints = int64(4)
	precisionFactor    = new(big.Int).Exp(big.NewInt(10), big.NewInt(18-valueDecimalPoints), nil)
)

var (
	errSystemContractError    = errors.New("system contract error")
	errSystemContractAbiError = errors.New("system contract ABI error")

	errStakeMalformed        = errors.New("staking transaction malformed")
	errStakeNotEnoughBalance = errors.New("not enough balance for staking")

	errUnstakeMalformed             = errors.New("unstaking transaction malformed")
	errUnstakeTooManyClaimable      = errors.New("unstaking failure because of too many claimable entries")
	errUnstakeNotEnoughStakedAmount = errors.New("not enough staked tokens for amount requested to unstake")

	errVoteMalformed           = errors.New("voting transaction malformed")
	errVoteAddressIsNotWitness = errors.New("a voted address is not valid")
	errElectEnableMalformed    = errors.New("elect enable transaction malformed")
	errContractAbiMalformed    = errors.New("contract abi transaction malformed")
	errContractAbiNotFound     = errors.New("contract abi not found")
	errContractAbiExists       = errors.New("contract abi exists")

	errDBContractError      = errors.New("db contract error")
	errNoEntryFound         = errors.New("no entry found in db")
	errEmptyTableNameError  = errors.New("table name is empty or invalid")
	errTableAbiMalformed    = errors.New("abi is empty or invalid")
	errCreateTableMalformed = errors.New("create table transaction malformed")
	errCreateTableExists    = errors.New("create table failed as table exists")
	errInsertObjMalformed   = errors.New("insert object transaction malformed")
	errDeleteObjMalformed   = errors.New("delete object transaction malformed")
	errSelectMalformed      = errors.New("db select transaction malformed")
	errIteratorMalformed    = errors.New("next iterator transaction malformed")
)

const (
	// The account wants to be witness and will be considered for block producer
	// by stake delegation
	ElectEnabledFlag uint64 = 1
)

type systemContract struct{}

func (c *systemContract) RequiredGas(input []byte) uint64 {
	if len(input) == 0 {
		return params.SystemContractBaseGas
	}

	evmABI, err := abi.JSON(strings.NewReader(SystemContractABI))
	if err != nil {
		return params.SystemContractBaseGas
	}

	cmdData, inputData := input[:4], input[4:]
	method, err := evmABI.MethodById(cmdData)
	if err != nil {
		return params.SystemContractBaseGas
	}

	cmd := method.Name

	switch cmd {
	case SystemContractStakeCmd:
		return params.SystemContractStakeGas
	case SystemContractGetStakedCmd:
		return params.SystemContractGetStakedGas
	case SystemContractUnstakeCmd:
		return params.SystemContractUnstakeGas
	case SystemContractClaimCmd:
		return params.SystemContractClaimGas
	case SystemContractVoteCmd:
		var addresses []common.Address
		if err = evmABI.UnpackWithArguments(&addresses, cmd, inputData, abi.InputsArgumentsType); err != nil {
			return params.SystemContractBaseGas
		}
		return params.SystemContractVoteGas * uint64(len(addresses))
	case SystemContractUnvoteCmd:
		return params.SystemContractUnvoteGas
	case SystemContractElectEnableCmd:
		return params.SystemContractElectEnableGas
	case SystemContractStoreAbiCmd:
		return params.SystemContractStoreAbiGas
	case SystemContractGetAbiCmd:
		return params.SystemContractGetAbiGas
	default:
		return params.SystemContractBaseGas
	}
}

type Witness struct {
	Id    common.Address
	Stake uint64
	Flags uint64
}

type WitnessArray []Witness

func (s WitnessArray) Diff(b WitnessArray) types.DelegateDiff {
	r := make(types.DelegateDiff, 0)

	for i, d := range b {
		found := false
		for j, from := range s {
			if from.Id == d.Id {
				if i != j {
					r = append(r, types.DelegateItem{Pos: byte(i), DelegateAddress: common.Address{}, DelegateNumber: byte(j)})
				}
				found = true
				break
			}
		}
		if !found {
			r = append(r, types.DelegateItem{Pos: byte(i), DelegateAddress: d.Id, DelegateNumber: 0})
		}
	}

	return r
}

var WitnessesTable = ebkdb.GetDBTableName(types.PrecompliledSystemContract, "Witnesses")

type ClaimableId [common.AddressLength + unsafe.Sizeof(uint64(0))]byte // address + timestamp

type Claimable struct {
	Id        ClaimableId
	Amount    uint64
	Timestamp uint64
}

var ClaimableTable = ebkdb.GetDBTableName(types.PrecompliledSystemContract, "Claimable")

func GetClaimableId(from common.Address, timestamp uint64) ClaimableId {
	timestampBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBytes, timestamp)

	var id ClaimableId
	b := bytes.Join([][]byte{from.Bytes(), timestampBytes}, []byte(""))
	copy(id[:], b)
	return id
}

// DelegationId represents the 40 byte of two 20 bytes addresses combined.
type DelegationId [common.AddressLength * 2]byte

type Delegation struct {
	Id DelegationId // <from><witness>
}

var DelegationTable = ebkdb.GetDBTableName(types.PrecompliledSystemContract, "Delegations")

// AddressesToDelegationId returns bytes of both from address and target address.
func AddressesToDelegationId(from common.Address, witness common.Address) DelegationId {
	var id DelegationId

	copy(id[:], from[:])
	copy(id[common.AddressLength:], witness[:])

	return id
}

// Content gets from and witness addresses.
func (id DelegationId) Content() (from common.Address, witness common.Address) {
	from = common.BytesToAddress(id[:common.AddressLength])
	witness = common.BytesToAddress(id[common.AddressLength:])
	return
}

type ContractAbiId []byte

type ContractAbi struct {
	Id  ContractAbiId
	Abi string
}

// GetContractAbiId returns bytes of both from address type and name.
func GetContractAbiId(address common.Address, abiType string, name string) ContractAbiId {
	var id ContractAbiId

	if abiType == "" {
		abiType = "abi"
	}

	b := bytes.Join([][]byte{address.Bytes(), []byte(abiType), []byte(name)}, []byte(""))
	id = b[:]

	return id
}

var ContractAbiTable = ebkdb.GetDBTableName(types.PrecompliledSystemContract, "ContractAbi")

func SystemContractSetupDB(db *ebakusdb.Snapshot, address common.Address) error {

	if db.HasTable(WitnessesTable) {
		panic("Witnesses table existed in genesis")
	}

	if db.HasTable(types.StakedTable) {
		panic("Staked table existed in genesis")
	}

	if db.HasTable(ClaimableTable) {
		panic("Claimable table existed in genesis")
	}

	if db.HasTable(DelegationTable) {
		panic("Delegation table existed in genesis")
	}

	if db.HasTable(ContractAbiTable) {
		panic("ContractAbi table existed in genesis")
	}

	db.CreateTable(WitnessesTable, &Witness{})
	db.CreateIndex(ebakusdb.IndexField{
		Table: WitnessesTable,
		Field: "Stake",
	})

	if err := db.InsertObj(WitnessesTable, &Witness{Id: address, Stake: 0, Flags: ElectEnabledFlag}); err != nil {
		return err
	}

	db.CreateTable(types.StakedTable, &types.Staked{})
	db.CreateTable(ClaimableTable, &Claimable{})
	db.CreateTable(DelegationTable, &Delegation{})

	db.CreateTable(ContractAbiTable, &ContractAbi{})

	// it's not trully needed to store the ABIs, though we do this just for occuping the address of the system contracts
	if _, err := storeAbiAtAddress(db, types.PrecompliledSystemContract, SystemContractABI); err != nil {
		return err
	}

	if _, err := storeAbiAtAddress(db, types.PrecompliledDBContract, DBABI); err != nil {
		return err
	}

	return nil
}

func DelegateVotingGetDelegates(snap *ebakusdb.Snapshot, maxWitnesses uint64) WitnessArray {
	res := make(WitnessArray, 0)

	orderClause, err := snap.OrderParser([]byte("Stake DESC"))
	if err != nil {
		log.Error("DelegateVotingGetDelegates load witnesses", "err", err)
		return res
	}

	iter, err := snap.Select(WitnessesTable, nil, orderClause)
	if err != nil {
		log.Error("DelegateVotingGetDelegates load witnesses", "err", err)
		return res
	}

	var w Witness
	i := uint64(0)
	for iter.Next(&w) && i < maxWitnesses {
		if (w.Flags & ElectEnabledFlag) == 0 {
			continue
		}
		res = append(res, w)
		i++
	}

	return res
}

func vote(db *ebakusdb.Snapshot, from common.Address, addresses []common.Address, amount uint64) error {
	for _, address := range addresses {
		var witness Witness

		where := []byte("Id LIKE ")
		whereClause, err := db.WhereParser(append(where, address.Bytes()...))
		if err != nil {
			return errSystemContractError
		}

		iter, err := db.Select(WitnessesTable, whereClause)
		if err != nil {
			return errSystemContractError
		}

		if iter.Next(&witness) == false {
			return errVoteAddressIsNotWitness
		}

		witness.Stake = witness.Stake + amount

		if err := db.InsertObj(WitnessesTable, &witness); err != nil {
			return errSystemContractError
		}

		delegation := Delegation{
			Id: AddressesToDelegationId(from, address),
		}

		if err := db.InsertObj(DelegationTable, &delegation); err != nil {
			return errSystemContractError
		}
	}

	return nil
}

func unvote(db *ebakusdb.Snapshot, from common.Address, amount uint64) ([]common.Address, error) {

	where := []byte("Id LIKE ")
	whereClause, err := db.WhereParser(append(where, from.Bytes()...))
	if err != nil {
		return nil, errSystemContractError
	}

	iter, err := db.Select(DelegationTable, whereClause)
	if err != nil {
		return nil, errSystemContractError
	}

	var delegation Delegation
	delegationsAddresses := make([]common.Address, 0)
	delegationsToBeDeleted := make([]Delegation, 0)

	for iter.Next(&delegation) {
		_, witnessAddress := delegation.Id.Content()

		delegationsToBeDeleted = append(delegationsToBeDeleted, delegation)
		delegationsAddresses = append(delegationsAddresses, witnessAddress)

		var witness Witness

		where := []byte("Id LIKE ")
		whereClause, err := db.WhereParser(append(where, witnessAddress.Bytes()...))
		if err != nil {
			return nil, errSystemContractError
		}

		iter, err := db.Select(WitnessesTable, whereClause)
		if err != nil {
			return nil, errSystemContractError
		}

		if iter.Next(&witness) == false {
			return nil, errSystemContractError
		}

		if witness.Stake < amount {
			return nil, errSystemContractError
		}

		witness.Stake = witness.Stake - amount

		if witness.Stake < 0 {
			return nil, errSystemContractError
		}

		if err := db.InsertObj(WitnessesTable, &witness); err != nil {
			return nil, errSystemContractError
		}
	}

	for _, delegation := range delegationsToBeDeleted {
		if err := db.DeleteObj(DelegationTable, delegation.Id); err != nil {
			return nil, errSystemContractError
		}
	}

	return delegationsAddresses, nil
}

const SystemContractABI = `[
{
  "type": "function",
  "name": "stake",
  "inputs": [
    {
      "name": "amount",
      "type": "uint64"
    }
  ],
  "outputs": [],
  "stateMutability": "nonpayable"
},{
  "type": "function",
  "name": "getStaked",
  "inputs": [],
  "outputs": [
    {
      "name": "staked",
      "type": "uint64"
    }
  ],
  "constant": true,
  "payable": false,
  "stateMutability": "view"
},{
  "type": "function",
  "name": "unstake",
  "inputs": [
    {
      "name": "amount",
      "type": "uint64"
    }
  ],
  "outputs": [],
  "stateMutability": "nonpayable"
},{
  "type": "function",
  "name": "claim",
  "inputs": [],
  "outputs": [],
  "stateMutability": "nonpayable"
},{
  "type": "function",
  "name": "vote",
  "inputs": [
    {
      "name": "addresses",
      "type": "address[]"
    }
  ],
  "outputs": [],
  "stateMutability": "nonpayable"
},{
  "type": "function",
  "name": "unvote",
  "inputs": [],
  "outputs": [],
  "stateMutability": "nonpayable"
},{
  "type": "function",
  "name": "electEnable",
  "inputs": [
    {
      "name": "enable",
      "type": "bool"
    }
  ],
  "outputs": [],
  "stateMutability": "nonpayable"
},{
  "type": "function",
  "name": "storeAbiForAddress",
  "inputs": [
    {
      "name": "address",
      "type": "address"
    },
    {
      "name": "abi",
      "type": "string"
    }
  ],
  "outputs": [],
  "stateMutability": "nonpayable"
},{
  "type": "function",
  "name": "getAbiForAddress",
  "inputs": [
    {
      "name": "address",
      "type": "address"
    }
  ],
  "outputs": [
    {
      "type": "string"
    }
  ],
  "constant": true,
  "payable": false,
  "stateMutability": "view"
}]`

const SystemContractTablesABI = `[
{
  "type": "table",
  "name": "Witnesses",
  "inputs": [
    {
      "name": "Id",
      "type": "address"
    },
    {
      "name": "Stake",
      "type": "uint64"
    },
    {
      "name": "Flags",
      "type": "uint64"
    }
  ]
},{
  "type": "table",
  "name": "Claimable",
  "inputs": [
    {
      "name": "Id",
      "type": "bytes28"
    },
    {
      "name": "Amount",
      "type": "uint64"
    },
    {
      "name": "Timestamp",
      "type": "uint64"
    }
  ]
},{
  "type": "table",
  "name": "Delegations",
  "inputs": [
    {
      "name": "Id",
      "type": "bytes40"
    }
  ]
},{
  "type": "table",
  "name": "ContractAbi",
  "inputs": [
    {
      "name": "Id",
      "type": "bytes"
    },
    {
      "name": "Abi",
      "type": "string"
    }
  ]
}]`

func (c *systemContract) stake(evm *EVM, from common.Address, amount uint64) ([]byte, error) {
	if amount <= 0 {
		log.Trace("Can't stake negative or zero amounts")
		return nil, errSystemContractError
	}

	db := evm.EbakusState

	hasEnoughBalance := false
	amountToBeTransfered := amount

	// Check if user has claimable entries that can be used for staking
	whereClaimable := []byte("Id LIKE ")
	whereClauseClaimable, err := db.WhereParser(append(whereClaimable, from.Bytes()...))
	if err != nil {
		return nil, errSystemContractError
	}

	orderClauseClaimable, err := db.OrderParser([]byte("Id DESC"))
	if err != nil {
		return nil, errSystemContractError
	}

	iterClaimable, err := db.Select(ClaimableTable, whereClauseClaimable, orderClauseClaimable)
	if err != nil {
		return nil, errSystemContractError
	}

	claimableAmount := uint64(0)
	claimablesToBeDeleted := make([]Claimable, 0)
	var claimable Claimable

	for iterClaimable.Next(&claimable) {
		if amount-claimableAmount >= claimable.Amount {
			claimableAmount = claimableAmount + claimable.Amount

			claimablesToBeDeleted = append(claimablesToBeDeleted, claimable)
		} else {
			claimable.Amount = claimable.Amount - (amount - claimableAmount)
			claimableAmount = claimableAmount + (amount - claimableAmount)

			if err := db.InsertObj(ClaimableTable, &claimable); err != nil {
				log.Trace("Claimable entry failed to be updated with new claimable amount", "err", err)
				return nil, errSystemContractError
			}
		}

		if claimableAmount == amount {
			hasEnoughBalance = true
			break
		}
	}

	amountToBeTransfered = amount - claimableAmount

	// Check account has balance (amount <= balance)
	balanceWei := evm.StateDB.GetBalance(from)
	balance := new(big.Int).Div(balanceWei, precisionFactor).Uint64()

	hasEnoughBalance = amountToBeTransfered <= balance

	if !hasEnoughBalance {
		log.Trace("Account doesn't have sufficient balance")
		return nil, errStakeNotEnoughBalance
	} else {
		for _, claimableEntry := range claimablesToBeDeleted {
			if err := db.DeleteObj(ClaimableTable, claimableEntry.Id); err != nil {
				log.Trace("Claimable tokens failed to delete (staked)", "err", err)
				return nil, errSystemContractError
			}
		}
	}

	//  Update whole system staked amount
	systemStaked := amount

	if systemStakedBytesOut, found := db.Get([]byte(types.SystemStakeDBKey)); found {
		systemStaked += binary.BigEndian.Uint64(*systemStakedBytesOut)
	}

	systemStakedBytesIn := make([]byte, 8)
	binary.BigEndian.PutUint64(systemStakedBytesIn[:], systemStaked)
	db.Insert([]byte(types.SystemStakeDBKey), systemStakedBytesIn)

	var staked types.Staked

	// Handle staked amount
	where := []byte("Id LIKE ")
	whereClause, err := db.WhereParser(append(where, from.Bytes()...))
	if err != nil {
		return nil, errSystemContractError
	}

	iter, err := db.Select(types.StakedTable, whereClause)
	if err != nil {
		return nil, errSystemContractError
	}

	if iter.Next(&staked) == true {
		delegatedAddresses, err := unvote(db, from, staked.Amount)
		if err != nil {
			return nil, errSystemContractError
		}

		staked.Amount = staked.Amount + amount

		if err := vote(db, from, delegatedAddresses, staked.Amount); err != nil {
			return nil, errSystemContractError
		}
	} else {
		delegatedAddresses, err := unvote(db, from, uint64(0))
		if err != nil {
			return nil, errSystemContractError
		}

		staked = types.Staked{
			Id:     from,
			Amount: amount,
		}

		if err := vote(db, from, delegatedAddresses, staked.Amount); err != nil {
			return nil, errSystemContractError
		}
	}

	if err := db.InsertObj(types.StakedTable, &staked); err != nil {
		return nil, errSystemContractError
	}

	amountToBeTransferedWei := new(big.Int).Mul(new(big.Int).SetUint64(amountToBeTransfered), precisionFactor)
	// Fail if we're trying to transfer more than the available balance
	if !evm.CanTransfer(evm.StateDB, from, amountToBeTransferedWei) {
		log.Trace("Failed to stake amount because of insufficient balance", "err", err)
		return nil, ErrInsufficientBalance
	}
	evm.Transfer(evm.StateDB, from, types.PrecompliledSystemContract, amountToBeTransferedWei)

	return nil, nil
}

func (c *systemContract) getStaked(evm *EVM, from common.Address) ([]byte, error) {
	db := evm.EbakusState

	var staked types.Staked

	where := []byte("Id LIKE ")
	whereClause, err := db.WhereParser(append(where, from.Bytes()...))
	if err != nil {
		return nil, errSystemContractError
	}

	iter, err := db.Select(types.StakedTable, whereClause)
	if err != nil {
		return nil, errSystemContractError
	}

	stakedAmount := uint64(0)

	if iter.Next(&staked) == true {
		stakedAmount = staked.Amount
	}

	stakedAmountBytes := make([]byte, 32)
	binary.BigEndian.PutUint64(stakedAmountBytes[24:], stakedAmount)
	return stakedAmountBytes, nil
}

func (c *systemContract) unstake(evm *EVM, from common.Address, amount uint64) ([]byte, error) {
	db := evm.EbakusState

	timestamp := evm.Time.Uint64() + unstakeVestingPeriod
	newClaimableEntryId := GetClaimableId(from, timestamp)

	// get all claimable tokens
	where := []byte("Id LIKE ")
	whereClause, err := db.WhereParser(append(where, from.Bytes()...))
	if err != nil {
		return nil, errSystemContractError
	}

	iter, err := db.Select(ClaimableTable, whereClause)
	if err != nil {
		return nil, errSystemContractError
	}

	// Disallow more than maxClaimableEntries, to protect system abuse
	var claimable Claimable
	countClaimableEntries := 0
	for iter.Next(&claimable) {
		countClaimableEntries++

		if newClaimableEntryId == claimable.Id {
			log.Trace("Unstake request failed because of existing entry for same block, please try again.")
			return nil, errSystemContractError
		}

		if countClaimableEntries >= maxClaimableEntries {
			log.Trace("Unstake failed as maxClaimableEntries reached", "maxClaimableEntries", maxClaimableEntries)
			return nil, errUnstakeTooManyClaimable
		}
	}

	var staked types.Staked
	iter, err = db.Select(types.StakedTable, whereClause)
	if err != nil {
		return nil, errSystemContractError
	}

	if iter.Next(&staked) == false {
		return nil, errSystemContractError
	}

	oldStake := staked.Amount
	newStake := uint64(0)

	if amount > staked.Amount {
		return nil, errUnstakeNotEnoughStakedAmount

	} else if amount == staked.Amount {
		if err := db.DeleteObj(types.StakedTable, staked.Id); err != nil {
			return nil, errSystemContractError
		}
	} else {
		staked.Amount = staked.Amount - amount
		newStake = staked.Amount

		if err := db.InsertObj(types.StakedTable, &staked); err != nil {
			return nil, errSystemContractError
		}
	}

	newClaimableEntry := Claimable{
		Id:        newClaimableEntryId,
		Amount:    amount,
		Timestamp: timestamp,
	}

	if err := db.InsertObj(ClaimableTable, &newClaimableEntry); err != nil {
		return nil, errSystemContractError
	}

	delegatedAddresses, err := unvote(db, from, oldStake)
	if err != nil {
		return nil, errSystemContractError
	}

	if err := vote(db, from, delegatedAddresses, newStake); err != nil {
		return nil, errSystemContractError
	}

	//  Update whole system staked amount
	systemStakedBytesOut, found := db.Get([]byte(types.SystemStakeDBKey))
	if !found {
		return nil, errSystemContractError
	}

	systemStakedOut := binary.BigEndian.Uint64(*systemStakedBytesOut)
	if systemStakedOut < amount {
		return nil, errSystemContractError
	}

	systemStaked := systemStakedOut - amount
	systemStakedBytesIn := make([]byte, 8)
	binary.BigEndian.PutUint64(systemStakedBytesIn[:], systemStaked)
	db.Insert([]byte(types.SystemStakeDBKey), systemStakedBytesIn)

	return nil, nil
}

func (c *systemContract) claim(evm *EVM, from common.Address) ([]byte, error) {
	db := evm.EbakusState

	// check if user has claimable tokens
	where := []byte("Id LIKE ")
	whereClause, err := db.WhereParser(append(where, from.Bytes()...))
	if err != nil {
		return nil, errSystemContractError
	}

	iter, err := db.Select(ClaimableTable, whereClause)
	if err != nil {
		return nil, errSystemContractError
	}

	claimableAmount := uint64(0)
	var claimable Claimable
	claimablesToBeDeleted := make([]Claimable, 0)

	for iter.Next(&claimable) {
		if claimable.Timestamp <= evm.Time.Uint64() {
			claimableAmount = claimableAmount + claimable.Amount

			claimablesToBeDeleted = append(claimablesToBeDeleted, claimable)
		}
	}

	for _, claimableEntry := range claimablesToBeDeleted {
		if err := db.DeleteObj(ClaimableTable, claimableEntry.Id); err != nil {
			return nil, errSystemContractError
		}
	}

	if claimableAmount <= 0 {
		log.Trace("No amount to be claimed")
		return nil, nil
	}

	claimableAmountWei := new(big.Int).Mul(new(big.Int).SetUint64(claimableAmount), precisionFactor)
	// Fail if we're trying to transfer more than the available balance
	if !evm.CanTransfer(evm.StateDB, types.PrecompliledSystemContract, claimableAmountWei) {
		log.Trace("Failed to claim amount because of insufficient balance", "err", err)
		return nil, ErrInsufficientBalance
	}
	evm.Transfer(evm.StateDB, types.PrecompliledSystemContract, from, claimableAmountWei)

	return nil, nil
}

func (c *systemContract) vote(evm *EVM, from common.Address, addresses []common.Address) ([]byte, error) {
	db := evm.EbakusState

	var staked types.Staked

	where := []byte("Id LIKE ")
	whereClause, err := db.WhereParser(append(where, from.Bytes()...))
	if err != nil {
		return nil, errSystemContractError
	}

	iter, err := db.Select(types.StakedTable, whereClause)
	if err != nil {
		return nil, errSystemContractError
	}

	if iter.Next(&staked) == false {
		return nil, errSystemContractError
	}

	if _, err := unvote(db, from, staked.Amount); err != nil {
		return nil, errSystemContractError
	}

	if err := vote(db, from, addresses, staked.Amount); err != nil {
		return nil, errSystemContractError
	}

	return nil, nil
}

func (c *systemContract) unvote(evm *EVM, from common.Address) ([]byte, error) {
	db := evm.EbakusState

	var staked types.Staked

	where := []byte("Id LIKE ")
	whereClause, err := db.WhereParser(append(where, from.Bytes()...))
	if err != nil {
		return nil, errSystemContractError
	}

	iter, err := db.Select(types.StakedTable, whereClause)
	if err != nil {
		return nil, errSystemContractError
	}

	if iter.Next(&staked) == false {
		return nil, errSystemContractError
	}

	if _, err := unvote(db, from, staked.Amount); err != nil {
		return nil, errSystemContractError
	}

	return nil, nil
}

func (c *systemContract) electEnable(evm *EVM, from common.Address, enable bool) ([]byte, error) {
	db := evm.EbakusState

	var witness Witness

	where := []byte("Id LIKE ")
	whereClause, err := db.WhereParser(append(where, from.Bytes()...))
	if err != nil {
		return nil, errSystemContractError
	}

	iter, err := db.Select(WitnessesTable, whereClause)
	if err != nil {
		return nil, errSystemContractError
	}

	if iter.Next(&witness) == false {
		witness = Witness{
			Id:    from,
			Stake: 0,
			Flags: 0,
		}
	}

	if enable {
		witness.Flags |= ElectEnabledFlag
	} else {
		witness.Flags &= ^ElectEnabledFlag
	}

	if err := db.InsertObj(WitnessesTable, &witness); err != nil {
		return nil, errSystemContractError
	}

	return nil, nil
}

func (c *systemContract) storeAbiAtAddress(evm *EVM, contractAddress common.Address, abi string) ([]byte, error) {
	return storeAbiAtAddress(evm.EbakusState, contractAddress, abi)
}

func storeAbiAtAddress(db *ebakusdb.Snapshot, contractAddress common.Address, abi string) ([]byte, error) {
	id := GetContractAbiId(contractAddress, "abi", "")

	where := []byte("Id LIKE ")
	whereClause, err := db.WhereParser(append(where, id[:]...))
	if err != nil {
		return nil, errSystemContractError
	}

	iter, err := db.Select(ContractAbiTable, whereClause)
	if err != nil {
		return nil, errSystemContractError
	}

	var contractAbi ContractAbi

	if iter.Next(&contractAbi) == true {
		return nil, errContractAbiExists
	}

	contractAbi = ContractAbi{
		Id:  id,
		Abi: abi,
	}

	if err := db.InsertObj(ContractAbiTable, &contractAbi); err != nil {
		return nil, errSystemContractError
	}

	return nil, nil
}

func (c *systemContract) getAbiAtAddress(evm *EVM, contractAddress common.Address) (string, error) {
	return GetAbiAtAddress(evm.EbakusState, contractAddress)
}

func GetAbiAtAddress(db *ebakusdb.Snapshot, contractAddress common.Address) (string, error) {

	if contractAddress == types.PrecompliledSystemContract {
		return SystemContractABI, nil
	} else if contractAddress == types.PrecompliledDBContract {
		return DBABI, nil
	}

	idPrefix := GetContractAbiId(contractAddress, "abi", "")

	where := []byte("Id LIKE ")
	whereClause, err := db.WhereParser(append(where, idPrefix[:]...))
	if err != nil {
		return "", errSystemContractError
	}

	iter, err := db.Select(ContractAbiTable, whereClause)
	if err != nil {
		return "", errSystemContractError
	}

	var contractAbi ContractAbi
	if iter.Next(&contractAbi) == false {
		return "", errContractAbiNotFound
	}

	contractAbiID := common.BytesToAddress(contractAbi.Id[:common.AddressLength])
	if contractAbiID != contractAddress {
		return "", errSystemContractError
	}

	return contractAbi.Abi, nil
}

func (c *systemContract) Run(evm *EVM, contract *Contract, input []byte) ([]byte, error) {
	from := contract.Caller()

	if len(input) == 0 {
		return nil, errSystemContractError
	}

	evmABI, err := abi.JSON(strings.NewReader(SystemContractABI))
	if err != nil {
		return nil, errSystemContractAbiError
	}

	cmdData, inputData := input[:4], input[4:]
	method, err := evmABI.MethodById(cmdData)
	if err != nil {
		return nil, errSystemContractAbiError
	}

	cmd := method.Name

	switch cmd {
	case SystemContractStakeCmd:
		var amount uint64
		err = evmABI.UnpackWithArguments(&amount, cmd, inputData, abi.InputsArgumentsType)
		if err != nil {
			log.Trace("SystemContractABI failed to unpack input", "cmd", cmd, "err", err)
			return nil, errStakeMalformed
		}

		_, err := c.claim(evm, from)
		if err != nil {
			return nil, err
		}

		return c.stake(evm, from, amount)
	case SystemContractGetStakedCmd:
		return c.getStaked(evm, from)
	case SystemContractUnstakeCmd:
		var amount uint64
		err = evmABI.UnpackWithArguments(&amount, cmd, inputData, abi.InputsArgumentsType)
		if err != nil {
			log.Trace("SystemContractABI failed to unpack input", "cmd", cmd, "err", err)
			return nil, errUnstakeMalformed
		}

		return c.unstake(evm, from, amount)
	case SystemContractClaimCmd:
		return c.claim(evm, from)
	case SystemContractVoteCmd:
		var addresses []common.Address
		err = evmABI.UnpackWithArguments(&addresses, cmd, inputData, abi.InputsArgumentsType)
		if err != nil {
			log.Trace("SystemContractABI failed to unpack input", "cmd", cmd, "err", err)
			return nil, errVoteMalformed
		}

		return c.vote(evm, from, addresses)
	case SystemContractUnvoteCmd:
		return c.unvote(evm, from)
	case SystemContractElectEnableCmd:
		var enable bool
		err = evmABI.UnpackWithArguments(&enable, cmd, inputData, abi.InputsArgumentsType)
		if err != nil {
			return nil, errElectEnableMalformed
		}

		return c.electEnable(evm, from, enable)
	case SystemContractStoreAbiCmd:
		type contractAbiInput struct {
			Address common.Address
			Abi     string
		}

		var input contractAbiInput
		err = evmABI.UnpackWithArguments(&input, cmd, inputData, abi.InputsArgumentsType)
		if err != nil {
			log.Trace("SystemContractABI failed to unpack input", "cmd", cmd, "err", err)
			return nil, errContractAbiMalformed
		}

		return c.storeAbiAtAddress(evm, input.Address, input.Abi)
	case SystemContractGetAbiCmd:
		var contractAddress common.Address
		err = evmABI.UnpackWithArguments(&contractAddress, cmd, inputData, abi.InputsArgumentsType)
		if err != nil {
			log.Trace("SystemContractABI failed to unpack input", "cmd", cmd, "err", err)
			return nil, errContractAbiMalformed
		}

		contractAbi, err := c.getAbiAtAddress(evm, contractAddress)
		if err != nil {
			return nil, errSystemContractError
		}

		res, err := evmABI.PackWithArguments(cmd, abi.OutputsArgumentsType, contractAbi)
		if err != nil {
			log.Trace("ContractAbi failed to pack response", "err", err)
			return nil, errSystemContractError
		}

		return res[4:], nil
	default:
		return nil, errSystemContractError
	}

	return nil, nil
}

// ECRECOVER implemented as a native contract.
type ecrecover struct{}

func (c *ecrecover) RequiredGas(input []byte) uint64 {
	return params.EcrecoverGas
}

func (c *ecrecover) Run(evm *EVM, contract *Contract, input []byte) ([]byte, error) {
	const ecRecoverInputLength = 128

	input = common.RightPadBytes(input, ecRecoverInputLength)
	// "input" is (hash, v, r, s), each 32 bytes
	// but for ecrecover we want (r, s, v)

	r := new(big.Int).SetBytes(input[64:96])
	s := new(big.Int).SetBytes(input[96:128])
	v := input[63] - 27

	// tighter sig s values input homestead only apply to tx sigs
	if !allZero(input[32:63]) || !crypto.ValidateSignatureValues(v, r, s, false) {
		return nil, nil
	}
	// We must make sure not to modify the 'input', so placing the 'v' along with
	// the signature needs to be done on a new allocation
	sig := make([]byte, 65)
	copy(sig, input[64:128])
	sig[64] = v
	// v needs to be at the end for libsecp256k1
	pubKey, err := crypto.Ecrecover(input[:32], sig)
	// make sure the public key is a valid one
	if err != nil {
		return nil, nil
	}

	// the first byte of pubkey is bitcoin heritage
	return common.LeftPadBytes(crypto.Keccak256(pubKey[1:])[12:], 32), nil
}

// SHA256 implemented as a native contract.
type sha256hash struct{}

// RequiredGas returns the gas required to execute the pre-compiled contract.
//
// This method does not require any overflow checking as the input size gas costs
// required for anything significant is so high it's impossible to pay for.
func (c *sha256hash) RequiredGas(input []byte) uint64 {
	return uint64(len(input)+31)/32*params.Sha256PerWordGas + params.Sha256BaseGas
}
func (c *sha256hash) Run(evm *EVM, contract *Contract, input []byte) ([]byte, error) {
	h := sha256.Sum256(input)
	return h[:], nil
}

// RIPEMD160 implemented as a native contract.
type ripemd160hash struct{}

// RequiredGas returns the gas required to execute the pre-compiled contract.
//
// This method does not require any overflow checking as the input size gas costs
// required for anything significant is so high it's impossible to pay for.
func (c *ripemd160hash) RequiredGas(input []byte) uint64 {
	return uint64(len(input)+31)/32*params.Ripemd160PerWordGas + params.Ripemd160BaseGas
}
func (c *ripemd160hash) Run(evm *EVM, contract *Contract, input []byte) ([]byte, error) {
	ripemd := ripemd160.New()
	ripemd.Write(input)
	return common.LeftPadBytes(ripemd.Sum(nil), 32), nil
}

// data copy implemented as a native contract.
type dataCopy struct{}

// RequiredGas returns the gas required to execute the pre-compiled contract.
//
// This method does not require any overflow checking as the input size gas costs
// required for anything significant is so high it's impossible to pay for.
func (c *dataCopy) RequiredGas(input []byte) uint64 {
	return uint64(len(input)+31)/32*params.IdentityPerWordGas + params.IdentityBaseGas
}
func (c *dataCopy) Run(evm *EVM, contract *Contract, in []byte) ([]byte, error) {
	return in, nil
}

// bigModExp implements a native big integer exponential modular operation.
type bigModExp struct{}

var (
	big1      = big.NewInt(1)
	big4      = big.NewInt(4)
	big8      = big.NewInt(8)
	big16     = big.NewInt(16)
	big32     = big.NewInt(32)
	big64     = big.NewInt(64)
	big96     = big.NewInt(96)
	big480    = big.NewInt(480)
	big1024   = big.NewInt(1024)
	big3072   = big.NewInt(3072)
	big199680 = big.NewInt(199680)
)

// RequiredGas returns the gas required to execute the pre-compiled contract.
func (c *bigModExp) RequiredGas(input []byte) uint64 {
	var (
		baseLen = new(big.Int).SetBytes(getData(input, 0, 32))
		expLen  = new(big.Int).SetBytes(getData(input, 32, 32))
		modLen  = new(big.Int).SetBytes(getData(input, 64, 32))
	)
	if len(input) > 96 {
		input = input[96:]
	} else {
		input = input[:0]
	}
	// Retrieve the head 32 bytes of exp for the adjusted exponent length
	var expHead *big.Int
	if big.NewInt(int64(len(input))).Cmp(baseLen) <= 0 {
		expHead = new(big.Int)
	} else {
		if expLen.Cmp(big32) > 0 {
			expHead = new(big.Int).SetBytes(getData(input, baseLen.Uint64(), 32))
		} else {
			expHead = new(big.Int).SetBytes(getData(input, baseLen.Uint64(), expLen.Uint64()))
		}
	}
	// Calculate the adjusted exponent length
	var msb int
	if bitlen := expHead.BitLen(); bitlen > 0 {
		msb = bitlen - 1
	}
	adjExpLen := new(big.Int)
	if expLen.Cmp(big32) > 0 {
		adjExpLen.Sub(expLen, big32)
		adjExpLen.Mul(big8, adjExpLen)
	}
	adjExpLen.Add(adjExpLen, big.NewInt(int64(msb)))

	// Calculate the gas cost of the operation
	gas := new(big.Int).Set(math.BigMax(modLen, baseLen))
	switch {
	case gas.Cmp(big64) <= 0:
		gas.Mul(gas, gas)
	case gas.Cmp(big1024) <= 0:
		gas = new(big.Int).Add(
			new(big.Int).Div(new(big.Int).Mul(gas, gas), big4),
			new(big.Int).Sub(new(big.Int).Mul(big96, gas), big3072),
		)
	default:
		gas = new(big.Int).Add(
			new(big.Int).Div(new(big.Int).Mul(gas, gas), big16),
			new(big.Int).Sub(new(big.Int).Mul(big480, gas), big199680),
		)
	}
	gas.Mul(gas, math.BigMax(adjExpLen, big1))
	gas.Div(gas, new(big.Int).SetUint64(params.ModExpQuadCoeffDiv))

	if gas.BitLen() > 64 {
		return math.MaxUint64
	}
	return gas.Uint64()
}

func (c *bigModExp) Run(evm *EVM, contract *Contract, input []byte) ([]byte, error) {
	var (
		baseLen = new(big.Int).SetBytes(getData(input, 0, 32)).Uint64()
		expLen  = new(big.Int).SetBytes(getData(input, 32, 32)).Uint64()
		modLen  = new(big.Int).SetBytes(getData(input, 64, 32)).Uint64()
	)
	if len(input) > 96 {
		input = input[96:]
	} else {
		input = input[:0]
	}
	// Handle a special case when both the base and mod length is zero
	if baseLen == 0 && modLen == 0 {
		return []byte{}, nil
	}
	// Retrieve the operands and execute the exponentiation
	var (
		base = new(big.Int).SetBytes(getData(input, 0, baseLen))
		exp  = new(big.Int).SetBytes(getData(input, baseLen, expLen))
		mod  = new(big.Int).SetBytes(getData(input, baseLen+expLen, modLen))
	)
	if mod.BitLen() == 0 {
		// Modulo 0 is undefined, return zero
		return common.LeftPadBytes([]byte{}, int(modLen)), nil
	}
	return common.LeftPadBytes(base.Exp(base, exp, mod).Bytes(), int(modLen)), nil
}

// newCurvePoint unmarshals a binary blob into a bn256 elliptic curve point,
// returning it, or an error if the point is invalid.
func newCurvePoint(blob []byte) (*bn256.G1, error) {
	p := new(bn256.G1)
	if _, err := p.Unmarshal(blob); err != nil {
		return nil, err
	}
	return p, nil
}

// newTwistPoint unmarshals a binary blob into a bn256 elliptic curve point,
// returning it, or an error if the point is invalid.
func newTwistPoint(blob []byte) (*bn256.G2, error) {
	p := new(bn256.G2)
	if _, err := p.Unmarshal(blob); err != nil {
		return nil, err
	}
	return p, nil
}

// runBn256Add implements the Bn256Add precompile, referenced by both
// Byzantium and Istanbul operations.
func runBn256Add(input []byte) ([]byte, error) {
	x, err := newCurvePoint(getData(input, 0, 64))
	if err != nil {
		return nil, err
	}
	y, err := newCurvePoint(getData(input, 64, 64))
	if err != nil {
		return nil, err
	}
	res := new(bn256.G1)
	res.Add(x, y)
	return res.Marshal(), nil
}

// bn256Add implements a native elliptic curve point addition conforming to
// Istanbul consensus rules.
type bn256AddIstanbul struct{}

// RequiredGas returns the gas required to execute the pre-compiled contract.
func (c *bn256AddIstanbul) RequiredGas(input []byte) uint64 {
	return params.Bn256AddGasIstanbul
}

func (c *bn256AddIstanbul) Run(evm *EVM, contract *Contract, input []byte) ([]byte, error) {
	return runBn256Add(input)
}

// runBn256ScalarMul implements the Bn256ScalarMul precompile, referenced by
// both Byzantium and Istanbul operations.
func runBn256ScalarMul(input []byte) ([]byte, error) {
	p, err := newCurvePoint(getData(input, 0, 64))
	if err != nil {
		return nil, err
	}
	res := new(bn256.G1)
	res.ScalarMult(p, new(big.Int).SetBytes(getData(input, 64, 32)))
	return res.Marshal(), nil
}

// bn256ScalarMulIstanbul implements a native elliptic curve scalar
// multiplication conforming to Istanbul consensus rules.
type bn256ScalarMulIstanbul struct{}

// RequiredGas returns the gas required to execute the pre-compiled contract.
func (c *bn256ScalarMulIstanbul) RequiredGas(input []byte) uint64 {
	return params.Bn256ScalarMulGasIstanbul
}

func (c *bn256ScalarMulIstanbul) Run(evm *EVM, contract *Contract, input []byte) ([]byte, error) {
	return runBn256ScalarMul(input)
}

var (
	// true32Byte is returned if the bn256 pairing check succeeds.
	true32Byte = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}

	// false32Byte is returned if the bn256 pairing check fails.
	false32Byte = make([]byte, 32)

	// errBadPairingInput is returned if the bn256 pairing input is invalid.
	errBadPairingInput = errors.New("bad elliptic curve pairing size")
)

// runBn256Pairing implements the Bn256Pairing precompile, referenced by both
// Byzantium and Istanbul operations.
func runBn256Pairing(input []byte) ([]byte, error) {
	// Handle some corner cases cheaply
	if len(input)%192 > 0 {
		return nil, errBadPairingInput
	}
	// Convert the input into a set of coordinates
	var (
		cs []*bn256.G1
		ts []*bn256.G2
	)
	for i := 0; i < len(input); i += 192 {
		c, err := newCurvePoint(input[i : i+64])
		if err != nil {
			return nil, err
		}
		t, err := newTwistPoint(input[i+64 : i+192])
		if err != nil {
			return nil, err
		}
		cs = append(cs, c)
		ts = append(ts, t)
	}
	// Execute the pairing checks and return the results
	if bn256.PairingCheck(cs, ts) {
		return true32Byte, nil
	}
	return false32Byte, nil
}

// bn256PairingIstanbul implements a pairing pre-compile for the bn256 curve
// conforming to Istanbul consensus rules.
type bn256PairingIstanbul struct{}

// RequiredGas returns the gas required to execute the pre-compiled contract.
func (c *bn256PairingIstanbul) RequiredGas(input []byte) uint64 {
	return params.Bn256PairingBaseGasIstanbul + uint64(len(input)/192)*params.Bn256PairingPerPointGasIstanbul
}

func (c *bn256PairingIstanbul) Run(evm *EVM, contract *Contract, input []byte) ([]byte, error) {
	return runBn256Pairing(input)
}

type blake2F struct{}

func (c *blake2F) RequiredGas(input []byte) uint64 {
	// If the input is malformed, we can't calculate the gas, return 0 and let the
	// actual call choke and fault.
	if len(input) != blake2FInputLength {
		return 0
	}
	return uint64(binary.BigEndian.Uint32(input[0:4]))
}

const (
	blake2FInputLength        = 213
	blake2FFinalBlockBytes    = byte(1)
	blake2FNonFinalBlockBytes = byte(0)
)

var (
	errBlake2FInvalidInputLength = errors.New("invalid input length")
	errBlake2FInvalidFinalFlag   = errors.New("invalid final flag")
)

func (c *blake2F) Run(evm *EVM, contract *Contract, input []byte) ([]byte, error) {
	// Make sure the input is valid (correct lenth and final flag)
	if len(input) != blake2FInputLength {
		return nil, errBlake2FInvalidInputLength
	}
	if input[212] != blake2FNonFinalBlockBytes && input[212] != blake2FFinalBlockBytes {
		return nil, errBlake2FInvalidFinalFlag
	}
	// Parse the input into the Blake2b call parameters
	var (
		rounds = binary.BigEndian.Uint32(input[0:4])
		final  = (input[212] == blake2FFinalBlockBytes)

		h [8]uint64
		m [16]uint64
		t [2]uint64
	)
	for i := 0; i < 8; i++ {
		offset := 4 + i*8
		h[i] = binary.LittleEndian.Uint64(input[offset : offset+8])
	}
	for i := 0; i < 16; i++ {
		offset := 68 + i*8
		m[i] = binary.LittleEndian.Uint64(input[offset : offset+8])
	}
	t[0] = binary.LittleEndian.Uint64(input[196:204])
	t[1] = binary.LittleEndian.Uint64(input[204:212])

	// Execute the compression function, extract and return the result
	blake2b.F(&h, m, t, final, rounds)

	output := make([]byte, 64)
	for i := 0; i < 8; i++ {
		offset := i * 8
		binary.LittleEndian.PutUint64(output[offset:offset+8], h[i])
	}
	return output, nil
}

const DBABI = `[
{
  "type": "function",
  "name": "createTable",
  "inputs": [
    {
      "name": "tableName",
      "type": "string"
    },
    {
      "name": "indexes",
      "type": "string"
    },
    {
      "name": "abi",
      "type": "string"
    }
  ],
  "outputs": [],
  "stateMutability": "nonpayable"
},{
  "type": "function",
  "name": "insertObj",
  "inputs": [
    {
      "name": "tableName",
      "type": "string"
    },
    {
      "name": "data",
      "type": "bytes"
    }
  ],
  "outputs": [
    {
      "type": "bool"
    }
  ],
  "stateMutability": "nonpayable"
},{
  "type": "function",
  "name": "deleteObj",
  "inputs": [
    {
      "name": "tableName",
      "type": "string"
    },
    {
      "name": "id",
      "type": "bytes"
    }
  ],
  "outputs": [
    {
      "type": "bool"
    }
  ],
  "stateMutability": "nonpayable"
},{
  "type": "function",
  "name": "get",
  "inputs": [
    {
      "name": "tableName",
      "type": "string"
    },
    {
      "name": "whereClause",
      "type": "string"
    },
    {
      "name": "orderClause",
      "type": "string"
    }
  ],
  "outputs": [
    {
      "type": "bytes"
    }
  ],
  "stateMutability": "nonpayable"
},{
  "type": "function",
  "name": "select",
  "inputs": [
    {
      "name": "tableName",
      "type": "string"
    },
    {
      "name": "whereClause",
      "type": "string"
    },
    {
      "name": "orderClause",
      "type": "string"
    }
  ],
  "outputs": [
    {
      "type": "bytes32"
    }
  ],
  "stateMutability": "nonpayable"
},{
  "type": "function",
  "name": "next",
  "inputs": [
    {
      "type": "bytes32"
    }
  ],
  "outputs": [
    {
      "type": "bytes"
    }
  ],
  "stateMutability": "nonpayable"
}]`

// dbContract exposes ebakusdb to solidity
type dbContract struct{}

// RequiredGas returns the gas required to execute the pre-compiled contract.
func (c *dbContract) RequiredGas(input []byte) uint64 {
	if len(input) == 0 {
		return params.DBContractBaseGas
	}

	evmABI, err := abi.JSON(strings.NewReader(DBABI))
	if err != nil {
		return params.DBContractBaseGas
	}

	cmdData, _ := input[:4], input[4:]
	method, err := evmABI.MethodById(cmdData)
	if err != nil {
		return params.DBContractBaseGas
	}

	cmd := method.Name

	switch cmd {
	case DBContractCreateTableCmd:
		return params.DBContractCreateTableGas
	case DBContractInsertObjCmd:
		return params.DBContractInsertObjGas
	case DBContractDeleteObjCmd:
		return params.DBContractDeleteObjGas
	case DBContractGetCmd:
		return params.DBContractGetGas
	case DBContractSelectCmd:
		return params.DBContractSelectGas
	case DBContractNextCmd:
		return params.DBContractNextGas
	default:
		return params.DBContractBaseGas
	}
}

type tableDef struct {
	TableName string
	Indexes   string
	Abi       string
}

type insertObjDef struct {
	TableName string
	Data      []byte
}
type deleteObjDef struct {
	TableName string
	Id        []byte
}

type selectDef struct {
	TableName   string
	WhereClause string
	OrderClause string
}

func GetAbiForTable(db *ebakusdb.Snapshot, contractAddress common.Address, name string) (*abi.ABI, error) {
	var abiString string

	if contractAddress == types.PrecompliledSystemContract {
		abiString = SystemContractTablesABI
	} else {
		id := GetContractAbiId(contractAddress, "table", name)

		where := []byte("Id LIKE ")
		whereClause, err := db.WhereParser(append(where, id...))
		if err != nil {
			return nil, errSystemContractError
		}

		iter, err := db.Select(ContractAbiTable, whereClause)
		if err != nil {
			return nil, errContractAbiNotFound
		}

		var contractAbi ContractAbi
		if iter.Next(&contractAbi) == false {
			return nil, errContractAbiNotFound
		}

		abiString = contractAbi.Abi
	}

	tableABI, err := abi.JSON(strings.NewReader(abiString))
	if err != nil {
		return nil, errDBContractError
	}

	return &tableABI, nil
}

func (c *dbContract) prependByteSize(data []byte) []byte {
	size := make([]byte, 32)
	binary.BigEndian.PutUint32(size[28:], uint32(len(data)))
	return append(size, data...)
}

func (c *dbContract) createTable(evm *EVM, contractAddress common.Address, table tableDef) ([]byte, error) {
	db := evm.EbakusState

	if table.TableName == "" {
		return nil, errEmptyTableNameError
	}
	dbTableName := ebkdb.GetDBTableName(contractAddress, table.TableName)

	if table.Abi == "" {
		return nil, errTableAbiMalformed
	}

	tableABI, err := abi.JSON(strings.NewReader(table.Abi))
	if err != nil {
		return nil, errTableAbiMalformed
	}

	obj, err := tableABI.GetTableInstance(table.TableName)
	if err != nil {
		return nil, err
	}

	id := GetContractAbiId(contractAddress, "table", table.TableName)

	where := []byte("Id = ")
	whereClause, err := db.WhereParser(append(where, id...))
	if err != nil {
		return nil, errDBContractError
	}

	iter, err := db.Select(ContractAbiTable, whereClause)
	if err != nil {
		return nil, errDBContractError
	}

	var contractAbi ContractAbi
	if iter.Next(&contractAbi) == true {
		return nil, errCreateTableExists
	}

	contractAbi = ContractAbi{
		Id:  id,
		Abi: table.Abi,
	}

	db.CreateTable(dbTableName, obj)

	if table.Indexes != "" {
		indexes := strings.Split(table.Indexes, ",")
		for _, index := range indexes {
			db.CreateIndex(ebakusdb.IndexField{
				Table: dbTableName,
				Field: index,
			})
		}
	}

	if err := db.InsertObj(ContractAbiTable, &contractAbi); err != nil {
		return nil, errDBContractError
	}

	return common.LeftPadBytes([]byte{1}, 32), nil
}

func (c *dbContract) insertObj(evm *EVM, contractAddress common.Address, insertObj insertObjDef) ([]byte, error) {
	db := evm.EbakusState

	if insertObj.TableName == "" {
		return nil, errEmptyTableNameError
	}
	dbTableName := ebkdb.GetDBTableName(contractAddress, insertObj.TableName)

	tableABI, err := GetAbiForTable(db, contractAddress, insertObj.TableName)
	if err != nil {
		return nil, err
	}

	obj, err := tableABI.GetTableInstance(insertObj.TableName)
	if err != nil {
		return nil, err
	}

	if err = tableABI.Unpack(obj, insertObj.TableName, insertObj.Data); err != nil {
		return nil, err
	}

	if err := db.InsertObj(dbTableName, obj); err != nil {
		return common.LeftPadBytes([]byte{0}, 32), nil
	}

	return common.LeftPadBytes([]byte{1}, 32), nil
}

func (c *dbContract) deleteObj(evm *EVM, contractAddress common.Address, deleteObj deleteObjDef) ([]byte, error) {
	db := evm.EbakusState

	if deleteObj.TableName == "" {
		return nil, errEmptyTableNameError
	}
	dbTableName := ebkdb.GetDBTableName(contractAddress, deleteObj.TableName)

	tableABI, err := GetAbiForTable(db, contractAddress, deleteObj.TableName)
	if err != nil {
		return nil, err
	}

	obj, err := tableABI.GetTableInstance(deleteObj.TableName)
	if err != nil {
		return nil, err
	}

	id, err := tableABI.UnpackSingle(obj, deleteObj.TableName, "Id", deleteObj.Id)
	if err != nil {
		return nil, err
	}

	if err := db.DeleteObj(dbTableName, id); err != nil {
		return common.LeftPadBytes([]byte{0}, 32), nil
	}

	return common.LeftPadBytes([]byte{1}, 32), nil
}

func EbakusDBGet(db *ebakusdb.Snapshot, contractAddress common.Address, tableName string, whereClause string, orderClause string) (interface{}, error) {
	if tableName == "" {
		return nil, errEmptyTableNameError
	}

	dbTableName := ebkdb.GetDBTableName(contractAddress, tableName)

	tableABI, err := GetAbiForTable(db, contractAddress, tableName)
	if err != nil {
		return nil, err
	}

	obj, err := tableABI.GetTableInstance(tableName)
	if err != nil {
		return nil, err
	}

	whereQuery, err := db.WhereParser([]byte(whereClause))
	if err != nil {
		return nil, errDBContractError
	}

	orderQuery, err := db.OrderParser([]byte(orderClause))
	if err != nil {
		return nil, errDBContractError
	}

	iter, err := db.Select(dbTableName, whereQuery, orderQuery)
	if err != nil {
		return nil, errDBContractError
	}

	if iter.Next(obj) == false {
		return nil, errNoEntryFound
	}

	return obj, nil
}

func (c *dbContract) get(evm *EVM, contractAddress common.Address, selectObj selectDef) ([]byte, error) {
	db := evm.EbakusState

	obj, err := EbakusDBGet(db, contractAddress, selectObj.TableName, selectObj.WhereClause, selectObj.OrderClause)
	if err != nil {
		return nil, err
	}

	tableABI, err := GetAbiForTable(db, contractAddress, selectObj.TableName)
	if err != nil {
		return nil, err
	}

	data, err := tableABI.Pack(selectObj.TableName, obj)
	if err != nil {
		return nil, err
	}

	return c.prependByteSize(data), nil
}

func EbakusDBSelect(db *ebakusdb.Snapshot, contractAddress common.Address, tableName string, whereClause string, orderClause string) (*ebakusdb.ResultIterator, error) {
	if tableName == "" {
		return nil, errEmptyTableNameError
	}
	dbTableName := ebkdb.GetDBTableName(contractAddress, tableName)

	whereQuery, err := db.WhereParser([]byte(whereClause))
	if err != nil {
		return nil, errDBContractError
	}

	orderQuery, err := db.OrderParser([]byte(orderClause))
	if err != nil {
		return nil, errDBContractError
	}

	iter, err := db.Select(dbTableName, whereQuery, orderQuery)
	if err != nil {
		return nil, errDBContractError
	}

	return iter, err
}

func (c *dbContract) selectIter(evm *EVM, contractAddress common.Address, obj selectDef) ([]byte, error) {
	db := evm.EbakusState

	iter, err := EbakusDBSelect(db, contractAddress, obj.TableName, obj.WhereClause, obj.OrderClause)
	if err != nil {
		return nil, err
	}

	iterPointer := evm.addEbakusStateIterator(obj.TableName, iter)

	var b bytes.Buffer
	binary.Write(&b, binary.BigEndian, iterPointer)

	return common.RightPadBytes(b.Bytes(), 32), nil
}

func EbakusDBNext(db *ebakusdb.Snapshot, contractAddress common.Address, tableName string, iter *ebakusdb.ResultIterator) (interface{}, error) {
	tableABI, err := GetAbiForTable(db, contractAddress, tableName)
	if err != nil {
		return nil, err
	}

	obj, err := tableABI.GetTableInstance(tableName)
	if err != nil {
		return nil, err
	}

	if iter.Next(obj) == false {
		// don't return an error as the contract doesn't have to stop execution
		// developer will check that no object found
		return nil, nil
	}

	return obj, nil
}

func (c *dbContract) next(evm *EVM, contractAddress common.Address, input []byte) ([]byte, error) {
	db := evm.EbakusState

	tableIter := evm.getEbakusStateIterator(binary.BigEndian.Uint64(input))

	obj, err := EbakusDBNext(db, contractAddress, tableIter.TableName, tableIter.Iter)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return c.prependByteSize([]byte{}), nil
	}

	tableABI, err := GetAbiForTable(db, contractAddress, tableIter.TableName)
	if err != nil {
		return nil, err
	}

	data, err := tableABI.Pack(tableIter.TableName, obj)
	if err != nil {
		return nil, err
	}

	return c.prependByteSize(data), nil
}

func (c *dbContract) Run(evm *EVM, contract *Contract, input []byte) ([]byte, error) {
	from := contract.Caller()

	if len(input) == 0 {
		return nil, errDBContractError
	}

	evmABI, err := abi.JSON(strings.NewReader(DBABI))
	if err != nil {
		return nil, errDBContractError
	}

	cmdData, inputData := input[:4], input[4:]
	method, err := evmABI.MethodById(cmdData)
	if err != nil {
		return nil, errDBContractError
	}

	cmd := method.Name

	switch cmd {
	case DBContractCreateTableCmd:
		var tableObj tableDef
		err = evmABI.UnpackWithArguments(&tableObj, cmd, inputData, abi.InputsArgumentsType)
		if err != nil {
			return nil, errCreateTableMalformed
		}

		return c.createTable(evm, from, tableObj)
	case DBContractInsertObjCmd:
		var insertObj insertObjDef
		err = evmABI.UnpackWithArguments(&insertObj, cmd, inputData, abi.InputsArgumentsType)
		if err != nil {
			return nil, errInsertObjMalformed
		}

		return c.insertObj(evm, from, insertObj)
	case DBContractDeleteObjCmd:
		var deleteObj deleteObjDef
		err = evmABI.UnpackWithArguments(&deleteObj, cmd, inputData, abi.InputsArgumentsType)
		if err != nil {
			return nil, errDeleteObjMalformed
		}

		return c.deleteObj(evm, from, deleteObj)
	case DBContractGetCmd:
		var selectData selectDef
		err = evmABI.UnpackWithArguments(&selectData, cmd, inputData, abi.InputsArgumentsType)
		if err != nil {
			return nil, errSelectMalformed
		}

		return c.get(evm, from, selectData)
	case DBContractSelectCmd:
		var selectData selectDef
		err = evmABI.UnpackWithArguments(&selectData, cmd, inputData, abi.InputsArgumentsType)
		if err != nil {
			return nil, errSelectMalformed
		}

		return c.selectIter(evm, from, selectData)
	case DBContractNextCmd:
		var iterData [32]byte
		err = evmABI.UnpackWithArguments(&iterData, cmd, inputData, abi.InputsArgumentsType)
		if err != nil {
			return nil, errIteratorMalformed
		}

		return c.next(evm, from, iterData[:])
	}

	return nil, nil
}
