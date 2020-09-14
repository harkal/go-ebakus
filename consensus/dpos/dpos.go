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
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"math/bits"
	"sync"
	"time"

	"github.com/ebakus/ebakusdb"

	"github.com/ebakus/go-ebakus/accounts"
	"github.com/ebakus/go-ebakus/common"
	"github.com/ebakus/go-ebakus/core"
	"github.com/ebakus/go-ebakus/core/rawdb"
	"github.com/ebakus/go-ebakus/core/state"
	"github.com/ebakus/go-ebakus/core/types"
	"github.com/ebakus/go-ebakus/core/vm"
	"github.com/ebakus/go-ebakus/crypto"
	"github.com/ebakus/go-ebakus/metrics"

	"github.com/ebakus/go-ebakus/log"
	"github.com/ebakus/go-ebakus/rlp"

	"github.com/ebakus/go-ebakus/consensus"
	"github.com/ebakus/go-ebakus/rpc"

	"github.com/ebakus/go-ebakus/ethdb"
	"github.com/ebakus/go-ebakus/params"
	lru "github.com/hashicorp/golang-lru"
	"golang.org/x/crypto/sha3"
)

var (
	checkpointInterval  = uint64(60 * 10)
	blockPeriod         = uint64(1)   // Default block issuance period of 5 sec
	initialDistribution = uint64(1e9) // EBK
	yearlyInflation     = float64(0.01)

	signatureCacheSize = 4096 // Number of recent block signatures to keep in memory
)

var (
	// errUnknownBlock is returned when the list of signers is requested for a block
	// that is not part of the local blockchain.
	errUnknownBlock = errors.New("unknown block")

	// errInvalidCheckpointBeneficiary is returned if a checkpoint/epoch transition
	// block has a beneficiary set to non-zeroes.
	errInvalidCheckpointBeneficiary = errors.New("beneficiary in checkpoint block non-zero")

	// ErrInvalidTimestamp is returned if the timestamp of a block is lower than
	// the previous block's timestamp + the minimum block period.
	ErrInvalidTimestamp = errors.New("invalid timestamp")

	// errUnauthorized is returned if a header is signed by a non-authorized entity.
	errUnauthorized = errors.New("unauthorized")

	// errInvalidVotingChain is returned when out-of-range or non-contiguous headers are provided.
	errInvalidHeaderChain = errors.New("invalid header chain")

	errInvalidStateHeaderAlignment = errors.New("invalid state header alignment")

	// errMissingSignature is returned if a block's does not contain a 65 byte secp256k1 signature.
	errMissingSignature = errors.New("65 byte signature missing")

	ErrInvalidDelegateUpdateBlock = errors.New("Delegates updated at wrong block")

	// ErrProductionAborted is returned when the producer is instructed to prepaturely abort
	ErrProductionAborted = errors.New("Production aborted")

	ErrWaitForTransactions = errors.New("Sealing paused, waiting for transactions")
)

var blockProduceTimer = metrics.GetOrRegisterTimer("worker/blocks/produce", nil)

// SignerFn is a signer callback function to request a hash to be signed by a
// backing account.
type SignerFn func(accounts.Account, string, []byte) ([]byte, error)

// DPOS is the delegate proof-of-stake consensus engine
type DPOS struct {
	config     *params.DPOSConfig
	db         ethdb.Database
	ebakusDb   *ebakusdb.DB
	blockchain *core.BlockChain
	genesis    *core.Genesis

	signatures *lru.ARCCache // Signatures of recent blocks to speed up address recover

	signer common.Address // Ebakus address of the signing key
	signFn SignerFn       // Signer function to authorize hashes with
	lock   sync.RWMutex
}

// ecrecover extracts the Ebakus account address from a signed header.
func ecrecover(header *types.Header, sigcache *lru.ARCCache) (common.Address, error) {
	// If the signature's already cached, return that
	hash := header.Hash()
	if address, known := sigcache.Get(hash); known {
		return address.(common.Address), nil
	}

	signature := header.Signature
	if len(signature) < 65 {
		return common.Address{}, errMissingSignature
	}

	// Recover the public key and the Ebakus address
	pubkey, err := crypto.Ecrecover(sigHash(header).Bytes(), signature)
	if err != nil {
		return common.Address{}, err
	}
	var signer common.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])

	sigcache.Add(hash, signer)
	return signer, nil
}

// New creates a Delegated Proof of Stake consensus engine
func New(config *params.DPOSConfig, db ethdb.Database, ebakusDb *ebakusdb.DB, genesis *core.Genesis) *DPOS {
	conf := *config

	if conf.Period == 0 {
		conf.Period = blockPeriod
	}

	if conf.InitialDistribution == 0 {
		conf.InitialDistribution = initialDistribution
	}

	if conf.YearlyInflation == 0 {
		conf.YearlyInflation = yearlyInflation
	}

	signatures, _ := lru.NewARC(signatureCacheSize)

	return &DPOS{
		config:     &conf,
		db:         db,
		ebakusDb:   ebakusDb,
		blockchain: nil,
		genesis:    genesis,

		signatures: signatures,
	}
}

func (d *DPOS) SetBlockchain(bc *core.BlockChain) {
	d.blockchain = bc
}

// Author implements consensus.Engine, returning the Ebakus address recovered
// from the signature in the header
func (d *DPOS) Author(header *types.Header) (common.Address, error) {
	return ecrecover(header, d.signatures)
}

// VerifyHeader checks whether a header conforms to the consensus rules of a
// given engine. Verifying the seal may be done optionally here, or explicitly
// via the VerifySeal method.
func (d *DPOS) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {
	return d.verifyHeader(chain, header, nil)
}

func (d *DPOS) getParent(chain consensus.ChainReader, header *types.Header, parents []*types.Header) *types.Header {
	if len(parents) > 0 {
		return parents[len(parents)-1]
	}
	return chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
}

func (d *DPOS) verifyHeader(chain consensus.ChainReader, header *types.Header, parents []*types.Header) error {
	if header.Number == nil {
		return errUnknownBlock
	}

	blockNum := header.Number.Uint64()

	if header.Time > uint64(time.Now().Unix()) {
		return consensus.ErrFutureBlock
	}

	if blockNum == 0 {
		return nil
	}

	// Ensure that the block's timestamp isn't too close to it's parent
	parent := d.getParent(chain, header, parents)
	if parent == nil || parent.Number.Uint64() != blockNum-1 || parent.Hash() != header.ParentHash {
		return consensus.ErrUnknownAncestor
	}
	if parent.Time+d.config.Period > header.Time {
		return ErrInvalidTimestamp
	}

	return nil
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (d *DPOS) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		for i, header := range headers {
			err := d.verifyHeader(chain, header, headers[:i])

			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()

	return abort, results
}

// VerifyBlock verifies that the given block conform to the consensus
// rules of a given engine.
func (d *DPOS) VerifyBlock(chain consensus.ChainReader, block *types.Block) error {

	return d.verifySeal(chain, block.Header(), nil)
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (d *DPOS) VerifySeal(chain consensus.ChainReader, header *types.Header) error {
	return d.verifySeal(chain, header, nil)
}

func (d *DPOS) verifySeal(chain consensus.ChainReader, header *types.Header, parents []*types.Header) error {
	blockNumber := header.Number.Uint64()
	if blockNumber == 0 {
		return nil
	}

	slot := float64(header.Time) / float64(d.config.Period)

	parentBlockNumber := blockNumber - 1
	ebakusState, err := chain.EbakusStateAt(header.ParentHash, parentBlockNumber)
	if err != nil {
		return fmt.Errorf("Verify seal failed to get ebakus state: %s", err)
	}

	parentHeader := d.blockchain.GetHeaderByHash(header.ParentHash)

	signer := d.getSignerAtSlot(chain, parentHeader, ebakusState, slot)
	ebakusState.Release()

	blockSigner, err := ecrecover(header, d.signatures)
	if err != nil {
		return err
	}

	if blockSigner != signer {
		return errUnauthorized
	}

	return nil
}

// Close terminates any background threads maintained by the consensus engine (we don't have any).
func (d *DPOS) Close() error {
	var err error
	return err
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (d *DPOS) Prepare(chain consensus.ChainReader, stop <-chan struct{}) (*types.Block, *types.Header, error) {
	d.lock.RLock()
	signer := d.signer
	d.lock.RUnlock()

	for {
		head := chain.CurrentBlock()
		headSlot := float64(head.Time()) / float64(d.config.Period)

		now := unixNow()
		slot := float64(now) / float64(d.config.Period)

		headHash := head.Hash()
		headBlockNumber := head.NumberU64()
		ebakusState, err := chain.EbakusStateAt(headHash, headBlockNumber)
		if err != nil {
			return nil, nil, fmt.Errorf("Prepare new block failed to get ebakus state at block number %d: %s", headBlockNumber, err)
		}

		inTurnSigner := d.getSignerAtSlot(chain, head.Header(), ebakusState, slot)
		ebakusState.Release()

		log.Trace("Check turn", "slot", slot, "signer", signer, "turn for", inTurnSigner)

		if slot > headSlot && signer == inTurnSigner {
			// We are the chosen one. Break.
			num := head.Number()

			header := &types.Header{
				ParentHash: headHash,
				Number:     num.Add(num, common.Big1),
				GasLimit:   0,
				GasUsed:    0,
				Time:       uint64(slot * float64(d.config.Period)),
			}

			// Sealing the genesis block is not supported
			blockNumber := header.Number.Uint64()
			if blockNumber == 0 {
				return nil, nil, errUnknownBlock
			}

			log.Trace("Will seal block", "header", header, "slot", slot)

			return head, header, nil
		}

		nextSlotTime := time.Unix(int64((slot+1)*float64(d.config.Period)), 0)

		timeToNextSlot := nextSlotTime.Sub(time.Now())

		log.Trace("Sleeping", "time", timeToNextSlot)

		select {
		case <-stop:
			log.Info("Woke to abort")
			return nil, nil, ErrProductionAborted
		case <-time.After(timeToNextSlot):
		}
	}
}

// Finalize runs any post-transaction state modifications (e.g. block rewards)
// and assembles the final block.
// Note: The block header and state database might be updated to reflect any
// consensus rules that happen at finalization (e.g. block rewards).
func (d *DPOS) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDB, ebakusState *ebakusdb.Snapshot, coinbase common.Address, txs []*types.Transaction) {
	// Accumulate any block and uncle rewards and commit the final state root
	d.AccumulateRewards(chain.Config().DPOS, state, header, coinbase)
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
}

// FinalizeAndAssemble implements consensus.Engine, accumulating the block and
// setting the final state and assembling the block.
func (d *DPOS) FinalizeAndAssemble(chain consensus.ChainReader, header *types.Header, state *state.StateDB, ebakusState *ebakusdb.Snapshot, coinbase common.Address, txs []*types.Transaction,
	receipts []*types.Receipt) (*types.Block, error) {

	// For internal storage chains, refuse to seal empty blocks (no reward but would spin sealing)
	if len(txs) == 0 {
		now := unixNow()
		slot := float64(now) / float64(d.config.Period)
		nextSlotTime := time.Unix(int64((slot+1)*float64(d.config.Period)), 0)

		timeToNextSlot := nextSlotTime.Sub(time.Now())

		select {
		case <-time.After(timeToNextSlot):
		}

		return nil, ErrWaitForTransactions
	}

	// Accumulate any block and uncle rewards and commit the final state root
	d.AccumulateRewards(chain.Config().DPOS, state, header, coinbase)
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))

	// Calculate delegate changes
	oldBlockNumber := header.Number.Uint64() - 1
	oldEbakusSnapshotId := rawdb.ReadSnapshot(d.db, header.ParentHash, oldBlockNumber)
	if oldEbakusSnapshotId == nil {
		return nil, errUnknownBlock
	}
	oldEbakusState := d.ebakusDb.Snapshot(*oldEbakusSnapshotId)
	defer oldEbakusState.Release()

	delegateCount := d.config.DelegateCount
	bonusDelegateCount := d.config.BonusDelegateCount
	turnBlockCount := d.config.TurnBlockCount
	oldDelegates := GetDelegates(d.blockchain.GetHeaderByHash(header.ParentHash), oldEbakusState, delegateCount, bonusDelegateCount, turnBlockCount)
	newDelegates := GetDelegates(header, ebakusState, delegateCount, bonusDelegateCount, turnBlockCount)
	delegateDiff := oldDelegates.Diff(newDelegates)

	log.Trace("Delegates", "diff", delegateDiff)

	block := types.NewBlock(header, txs, receipts, &delegateDiff)

	return block, nil
}

// Authorize injects a private key into the consensus engine to mint new blocks
// with.
func (d *DPOS) Authorize(signer common.Address, signFn SignerFn) {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.signer = signer
	d.signFn = signFn
}

// Seal generates a new block for the given input block with the local miner's
// seal place on top.
func (d *DPOS) Seal(chain consensus.ChainReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	header := block.Header()

	// Sealing the genesis block is not supported
	blockNumber := header.Number.Uint64()
	if blockNumber == 0 {
		return errUnknownBlock
	}

	// Don't hold the signer fields for the entire sealing procedure
	d.lock.RLock()
	signer, signFn := d.signer, d.signFn
	d.lock.RUnlock()

	// Ensure the timestamp has the correct delay
	parent := chain.GetHeader(header.ParentHash, blockNumber-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}

	// Sign
	sighash, err := signFn(accounts.Account{Address: signer}, accounts.MimetypeDpos, RLP(header))
	if err != nil {
		return err
	}

	header.Signature = sighash

	results <- block.WithSeal(header)

	return nil
}

// SealHash returns the hash of a block prior to it being sealed.
func (d *DPOS) SealHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()

	rlp.Encode(hasher, []interface{}{
		header.ParentHash,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
	})
	hasher.Sum(hash[:0])
	return hash
}

// AccumulateRewards credits the coinbase of the given block with the reward
func (d *DPOS) AccumulateRewards(config *params.DPOSConfig, state *state.StateDB, header *types.Header, coinbase common.Address) {
	reward := big.NewInt(3171 * 1e14)

	state.AddBalance(coinbase, reward)
}

// CalcDifficulty is essentialy dummy in ebakus
func (d *DPOS) CalcDifficulty(chain consensus.ChainReader, time uint64, parent *types.Header) *big.Int {
	return big.NewInt(1)
}

// APIs implements consensus.Engine, returning the user facing RPC API to allow
// controlling the delegate voting etc.
func (d *DPOS) APIs(chain consensus.ChainReader) []rpc.API {
	return []rpc.API{{
		Namespace: "dpos",
		Version:   "1.0",
		Service:   &API{chain: chain, dpos: d},
	}}
}

func unixNow() uint64 {
	return uint64(time.Now().Unix())
}

func (d *DPOS) getSignerAtSlot(chain consensus.ChainReader, header *types.Header, state *ebakusdb.Snapshot, slot float64) common.Address {
	delegates := GetDelegates(header, state, d.config.DelegateCount, d.config.BonusDelegateCount, d.config.TurnBlockCount)

	if d.config.TurnBlockCount == 0 {
		log.Warn("DPOS.TurnBlockCount is zero. This means that mining won't match a signer.")
	}

	if d.config.DelegateCount == 0 || d.config.TurnBlockCount == 0 {
		return common.Address{}
	}

	slot = slot / float64(d.config.TurnBlockCount)
	s := int(slot) % int(d.config.DelegateCount)

	if s < len(delegates) {
		return delegates[s].Id
	}

	return common.Address{}
}

func (d *DPOS) getBlockDensity(chain consensus.ChainReader, number rpc.BlockNumber, lookbackTime uint64) (map[string]interface{}, error) {
	var prevBlock *types.Block
	totalMissedBlocks := 0
	latestBlockNumber := chain.CurrentBlock().NumberU64()

	if number == rpc.LatestBlockNumber {
		number = rpc.BlockNumber(latestBlockNumber)
	}

	if uint64(number) > latestBlockNumber {
		return nil, consensus.ErrFutureBlock
	}

	initialBlock := d.blockchain.GetBlockByNumber(uint64(number))
	blocksHashes := d.blockchain.GetBlockHashesFromHash(initialBlock.Hash(), lookbackTime)

	// create a map using `timestamp` as key for our algorithm lookup
	blocksMap := make(map[uint64]*types.Block, len(blocksHashes)+1)
	blocksMap[uint64(initialBlock.Time())] = initialBlock

	for _, blockHash := range blocksHashes {
		block := d.blockchain.GetBlockByHash(blockHash)
		blocksMap[uint64(block.Time())] = block
	}

	lookbackTimestamp := initialBlock.Time() - lookbackTime

	blocksNumber := uint64(len(blocksHashes) + 1)
	if lookbackTime > blocksNumber {
		lookbackTimestamp = initialBlock.Time() - blocksNumber
	}

	for timestamp := initialBlock.Time(); timestamp >= lookbackTimestamp; timestamp-- {
		block, blockFound := blocksMap[timestamp]
		if !blockFound {
			totalMissedBlocks++
			continue
		}

		if err := d.verifySeal(chain, block.Header(), nil); err != nil {
			totalMissedBlocks++
		}

		if prevBlock != nil && (prevBlock.NumberU64() != block.NumberU64()+1 || block.Hash() != prevBlock.ParentHash()) {
			totalMissedBlocks++
		}

		prevBlock = block
	}

	result := map[string]interface{}{
		"total_missed_blocks": totalMissedBlocks,
	}

	return result, nil
}

func uniformRandom(max uint64, hash common.Hash) uint64 {
	bitsRequired := bits.Len64(max - 1)

	startBit := 0
	var rand uint64
	for {
		rand = 0
		for i := 0; i < bitsRequired; i++ {
			b := hash[((startBit+i)/8)%common.HashLength]
			p := byte((startBit + i) % 8)
			rand += uint64((b & (1 << p) >> p)) << i
		}
		if rand < max {
			break
		}
		if startBit/8 >= common.HashLength {
			rand = rand % max
			break
		}
		startBit++
	}

	return rand
}

func GetDelegates(header *types.Header, snap *ebakusdb.Snapshot, maxWitnesses uint64, maxBonusWitnesses uint64, turnBlockCount uint64) vm.WitnessArray {
	if maxWitnesses == 0 {
		log.Warn("DPOS.getDelegates maxWitnesses is zero. This means that mining won't match a signer. Check if DPOS.DelegatesCount is set to zero")
	}

	maxWitnessesToLoad := maxWitnesses
	if maxBonusWitnesses > 0 {
		maxWitnessesToLoad += maxBonusWitnesses
	}

	delegates := vm.DelegateVotingGetDelegates(snap, maxWitnessesToLoad)

	// get bonus delegate
	if uint64(len(delegates)) > maxWitnesses {
		bonusCandidateDelegates := delegates[maxWitnesses-1:]
		delegates = delegates[:maxWitnesses-1] // excluding the last bonus position from maxWitnesses

		slot := (header.Time + 1) / turnBlockCount
		slotData := make([]byte, 8)
		binary.BigEndian.PutUint64(slotData, slot)
		rand := uniformRandom(uint64(len(bonusCandidateDelegates)), crypto.Keccak256Hash(slotData))
		delegates = append(delegates, bonusCandidateDelegates[rand])
	}

	return delegates
}

// sigHash returns the hash which is used as input for the delegate proof-of-stake
// signing. It is the hash of the entire header apart from the 65 byte signature
// contained at the end of the extra data.
func sigHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	encodeSigHeader(hasher, header)
	hasher.Sum(hash[:0])
	return hash
}

// RLP returns the rlp bytes which needs to be signed for the proof-of-authority
// sealing.
func RLP(header *types.Header) []byte {
	b := new(bytes.Buffer)
	encodeSigHeader(b, header)
	return b.Bytes()
}

func encodeSigHeader(w io.Writer, header *types.Header) {
	err := rlp.Encode(w, []interface{}{
		header.ParentHash,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.DelegateDiff,
	})
	if err != nil {
		panic("can't encode: " + err.Error())
	}
}
