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

package miner

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ebakus/ebakusdb"
	"github.com/ebakus/go-ebakus/common"
	"github.com/ebakus/go-ebakus/consensus"
	"github.com/ebakus/go-ebakus/consensus/dpos"
	"github.com/ebakus/go-ebakus/core"
	"github.com/ebakus/go-ebakus/core/state"
	"github.com/ebakus/go-ebakus/core/types"
	"github.com/ebakus/go-ebakus/event"
	"github.com/ebakus/go-ebakus/log"
	"github.com/ebakus/go-ebakus/metrics"
	"github.com/ebakus/go-ebakus/params"
)

var blockProduceTimer = metrics.GetOrRegisterTimer("worker/blocks/produce", nil)

// environment is the worker's current environment and holds all of the current state information.
type environment struct {
	signer types.Signer

	state       *state.StateDB // apply state changes here
	ebakusState *ebakusdb.Snapshot
	tcount      int           // tx count in cycle
	gasPool     *core.GasPool // available gas used to pack transactions

	Block *types.Block // the new block

	header   *types.Header
	txs      []*types.Transaction
	receipts []*types.Receipt

	createdAt time.Time
}

// worker is the main object which takes care of submitting new work to consensus engine
// and gathering the sealing result.
type worker struct {
	config      *Config
	chainConfig *params.ChainConfig
	engine      consensus.Engine
	eth         Backend
	chain       *core.BlockChain
	ebakusDb    *ebakusdb.DB

	// Subscriptions
	mux *event.TypeMux

	// Channels
	stopCh chan struct{}

	currentMu sync.Mutex
	current   *environment // An environment for current running cycle.

	mu       sync.RWMutex // The lock used to protect the coinbase and extra fields
	coinbase common.Address

	// atomic status counters
	running int32 // The indicator whether the consensus engine is running or not.

	// wait group is used for graceful shutdowns
	wg sync.WaitGroup

	// External functions
	isLocalBlock func(block *types.Block) bool // Function used to determine whether the specified block is mined by local miner.
}

func newWorker(config *Config, chainConfig *params.ChainConfig, engine consensus.Engine, eth Backend, mux *event.TypeMux, isLocalBlock func(*types.Block) bool) *worker {
	worker := &worker{
		config:       config,
		chainConfig:  chainConfig,
		engine:       engine,
		eth:          eth,
		mux:          mux,
		stopCh:       make(chan struct{}),
		chain:        eth.BlockChain(),
		ebakusDb:     eth.EbakusDb(),
		isLocalBlock: isLocalBlock,
	}

	return worker
}

// setEtherbase sets the etherbase used to initialize the block coinbase field.
func (w *worker) setEtherbase(addr common.Address) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.coinbase = addr
}

// pending returns the pending state and corresponding block.
func (w *worker) pending() (*types.Block, *state.StateDB) {
	w.currentMu.Lock()
	defer w.currentMu.Unlock()

	if !w.isRunning() && w.current != nil {
		return types.NewBlock(
			w.current.header,
			w.current.txs,
			w.current.receipts,
			nil,
		), w.current.state.Copy()
	}
	return w.current.Block, w.current.state.Copy()
}

func (w *worker) pendingSnapshot() (*types.Block, *ebakusdb.Snapshot) {
	w.currentMu.Lock()
	defer w.currentMu.Unlock()

	snapshot := w.current.ebakusState.Snapshot()

	if !w.isRunning() && w.current != nil {
		return types.NewBlock(
			w.current.header,
			w.current.txs,
			w.current.receipts,
			nil,
		), snapshot
	}

	return nil, snapshot
}

// pendingBlock returns pending block.
func (w *worker) pendingBlock() *types.Block {
	w.currentMu.Lock()
	defer w.currentMu.Unlock()

	if !w.isRunning() && w.current != nil {
		return types.NewBlock(
			w.current.header,
			w.current.txs,
			w.current.receipts,
			nil,
		)
	}
	return w.current.Block
}

// start sets the running status as 1 and triggers new work submitting.
func (w *worker) start() {
	log.Trace("Worker start")

	atomic.StoreInt32(&w.running, 1)

	go w.blockProducer()
}

// stop sets the running status as 0.
func (w *worker) stop() {
	log.Trace("Worker stop")
	if w.isRunning() {
		// Make it stop
		atomic.StoreInt32(&w.running, 0)

		select {
		case w.stopCh <- struct{}{}:
			break
		default:
			break
		}
	}

	w.wg.Wait()
	log.Trace("Worker stopped")
}

// isRunning returns an indicator whether worker is running or not.
func (w *worker) isRunning() bool {
	return atomic.LoadInt32(&w.running) == 1
}

// close terminates all background threads maintained by the worker.
// Note the worker does not support being closed multiple times.
func (w *worker) close() {
	close(w.stopCh)
}

func (w *worker) blockProducer() {
	w.wg.Add(1)

	for {
		if !w.isRunning() {
			log.Info("Block producer terminating (no longer running)")
			break
		}

		w.commitNewWork()

		log.Trace("Block producer committed work", "running", w.isRunning())
	}

	w.wg.Done()

	log.Info("Block producer terminating")
}

func (w *worker) processWork(env *environment, block *types.Block) {
	// Update the block hash in all logs since it is now available and not when the
	// receipt/log of individual transactions were created.
	for i, receipt := range env.receipts {
		// add block location fields
		receipt.BlockHash = block.Hash()
		receipt.BlockNumber = block.Number()
		receipt.TransactionIndex = uint(i)

		for _, l := range receipt.Logs {
			l.BlockHash = block.Hash()
		}
	}
	for _, log := range env.state.Logs() {
		log.BlockHash = block.Hash()
	}

	stat, err := w.chain.WriteBlockWithState(block, env.receipts, env.state, env.ebakusState)
	if err != nil {
		log.Error("Failed writing block to chain", "err", err)
		return
	}

	env.ebakusState.Release()

	log.Info("Successfully sealed new block", "number", block.Number(), "hash", block.Hash())

	// Broadcast the block and announce chain insertion event
	w.mux.Post(core.NewMinedBlockEvent{Block: block})
	var (
		events []interface{}
		logs   = env.state.Logs()
	)
	switch stat {
	case core.CanonStatTy:
		events = append(events, core.ChainEvent{Block: block, Hash: block.Hash(), Logs: logs})
		events = append(events, core.ChainHeadEvent{Block: block})
	case core.SideStatTy:
		events = append(events, core.ChainSideEvent{Block: block})
	}
	w.chain.PostChainEvents(events, logs)

	log.Trace("Process work. Done")
}

// makeCurrent creates a new environment for the current cycle.
func (w *worker) makeCurrent(parent *types.Block, header *types.Header) error {
	state, err := w.chain.StateAt(parent.Root())
	if err != nil {
		return err
	}

	ebakusState, err := w.chain.EbakusStateAt(parent.Hash(), parent.NumberU64()) // gets released in processWork()
	if err != nil {
		return fmt.Errorf("Worker makeCurrent() failed to get ebakus state at block number %d: %s", parent.NumberU64(), err)
	}

	env := &environment{
		signer:      types.NewEIP155Signer(w.chainConfig.ChainID),
		state:       state,
		ebakusState: ebakusState,
		header:      header,
		createdAt:   time.Now(),
	}

	// Keep track of transactions which return errors so they can be removed
	env.tcount = 0
	w.current = env
	return nil
}

// commitNewWork generates several new sealing tasks based on the parent block.
// func (w *worker) commitNewWork(interrupt *int32, timestamp int64) {
func (w *worker) commitNewWork() {
	if !w.isRunning() {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.currentMu.Lock()
	defer w.currentMu.Unlock()

	parent, header, err := w.engine.Prepare(w.chain, w.stopCh)
	if err != nil {
		if err != dpos.ErrProductionAborted {
			log.Error("Failed to prepare header for mining", "err", err)
			time.Sleep(2 * time.Second)
		}
		return
	}

	// Are we still mining?
	if !w.isRunning() {
		return
	}

	header.GasLimit = core.CalcGasLimit(parent.Header(), w.config.GasFloor, w.config.GasCeil)

	// Could potentially happen if starting to mine in an odd state.
	err = w.makeCurrent(parent, header)
	if err != nil {
		log.Error("Failed to create mining context", "err", err)
		return
	}

	// Fill the block with all available pending transactions.
	pending, err := w.eth.TxPool().Pending()
	if err != nil {
		log.Error("Failed to fetch pending transactions", "err", err)
		return
	}

	env := w.current
	txs := types.NewTransactionsByVirtualDifficultyAndNonce(w.current.signer, pending, env.ebakusState)
	// tcount := w.current.tcount
	w.commitTransactions(txs, w.coinbase)

	// Create the new block to seal with the consensus engine
	if env.Block, err = w.engine.FinalizeAndAssemble(w.chain, header, env.state, env.ebakusState, w.coinbase, env.txs, env.receipts); err != nil {
		if err != dpos.ErrWaitForTransactions {
			log.Error("Failed to finalize block for sealing", "err", err)
		}
		return
	}
	// We only care about logging if we're actually mining.
	if w.isRunning() {
		log.Info("Commit new mining work", "number", env.Block.Number(), "txs", env.tcount, "hash", env.Block.Hash())
	}

	results := make(chan *types.Block, 1)
	if err := w.engine.Seal(w.chain, env.Block, results, nil); err != nil {
		log.Error("Block sealing failed", "err", err)
		return
	}

	select {
	case res := <-results:
		w.processWork(env, res)

		log.Info("Committed work", "number", env.Block.Number())
	}
}

func (w *worker) commitTransaction(tx *types.Transaction, coinbase common.Address) ([]*types.Log, error) {
	snap := w.current.state.Snapshot()
	ebakusSnapshot := w.current.ebakusState.Snapshot()
	defer ebakusSnapshot.Release()

	receipt, _, err := core.ApplyTransaction(w.chainConfig, w.chain, &coinbase, w.current.gasPool, w.current.state, ebakusSnapshot, w.current.header, tx, &w.current.header.GasUsed, *w.chain.GetVMConfig())
	if err != nil {
		w.current.state.RevertToSnapshot(snap)
		return nil, err
	}

	w.current.ebakusState.ResetTo(ebakusSnapshot)
	w.current.txs = append(w.current.txs, tx)
	w.current.receipts = append(w.current.receipts, receipt)

	return receipt.Logs, nil
}

func (w *worker) commitTransactions(txs *types.TransactionsByVirtualDifficultyAndNonce, coinbase common.Address) bool {
	// Short circuit if current is nil
	if w.current == nil {
		return true
	}

	if w.current.gasPool == nil {
		w.current.gasPool = new(core.GasPool).AddGas(w.current.header.GasLimit)
	}

	var coalescedLogs []*types.Log

	startTime := time.Now()

	for {
		if elapsed := time.Since(startTime); elapsed > time.Millisecond*500 {
			log.Trace("Not enough time for further transactions", elapsed)
			break
		}

		// If we don't have enough gas for any further transactions then we're done
		if w.current.gasPool.Gas() < params.TxGas {
			log.Trace("Not enough gas for further transactions", "have", w.current.gasPool, "want", params.TxGas)
			break
		}
		// Retrieve the next transaction and abort if all done
		tx := txs.Peek()
		if tx == nil {
			break
		}
		// Error may be ignored here. The error has already been checked
		// during transaction acceptance is the transaction pool.
		//
		// We use the eip155 signer regardless of the current hf.
		from, _ := types.Sender(w.current.signer, tx)
		// Check whether the tx is replay protected. If we're not in the EIP155 hf
		// phase, start ignoring the sender until we do.
		if tx.Protected() && !w.chainConfig.IsEIP155(w.current.header.Number) {
			log.Trace("Ignoring reply protected transaction", "hash", tx.Hash(), "eip155", w.chainConfig.EIP155Block)

			txs.Pop()
			continue
		}

		// Start executing the transaction
		w.current.state.Prepare(tx.Hash(), common.Hash{}, w.current.tcount)

		logs, err := w.commitTransaction(tx, coinbase)
		switch err {
		case core.ErrGasLimitReached:
			// Pop the current out-of-gas transaction without shifting in the next from the account
			log.Trace("Gas limit exceeded for current block", "sender", from)
			txs.Pop()

		case core.ErrNonceTooLow:
			// New head notification data race between the transaction pool and miner, shift
			log.Trace("Skipping transaction with low nonce", "sender", from, "nonce", tx.Nonce())
			txs.Shift()

		case core.ErrNonceTooHigh:
			// Reorg notification data race between the transaction pool and miner, skip account =
			log.Trace("Skipping account with hight nonce", "sender", from, "nonce", tx.Nonce())
			txs.Pop()

		case nil:
			// Everything ok, collect the logs and shift in the next transaction from the same account
			coalescedLogs = append(coalescedLogs, logs...)
			w.current.tcount++
			txs.Shift()

		default:
			// Strange error, discard the transaction and get the next in line (note, the
			// nonce-too-high clause will prevent us from executing in vain).
			log.Debug("Transaction failed, account skipped", "hash", tx.Hash(), "err", err)
			txs.Shift()
		}
	}

	if len(coalescedLogs) > 0 || w.current.tcount > 0 {
		// make a copy, the state caches the logs and these logs get "upgraded" from pending to mined
		// logs by filling in the block hash when the block was mined by the local miner. This can
		// cause a race condition if a log was "upgraded" before the PendingLogsEvent is processed.
		cpy := make([]*types.Log, len(coalescedLogs))
		for i, l := range coalescedLogs {
			cpy[i] = new(types.Log)
			*cpy[i] = *l
		}
		go w.mux.Post(core.PendingLogsEvent{Logs: cpy})
	}

	elapsed := time.Since(startTime)

	log.Trace("Commit transactions completed", "elapsed", common.PrettyDuration(elapsed))
	blockProduceTimer.Update(elapsed)

	return false
}
