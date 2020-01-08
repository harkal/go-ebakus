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
	"container/heap"
	"encoding/binary"
	"errors"
	"io"
	"math/big"
	"sync/atomic"
	"time"

	"ekyu.moe/cryptonight"
	"github.com/ebakus/ebakusdb"
	"github.com/ebakus/go-ebakus/common"
	"github.com/ebakus/go-ebakus/common/hexutil"
	"github.com/ebakus/go-ebakus/crypto"
	"github.com/ebakus/go-ebakus/metrics"
	"github.com/ebakus/go-ebakus/rlp"
)

//go:generate gencodec -type txdata -field-override txdataMarshaling -out gen_tx_json.go

var (
	transactionCalculateWorkNonceTimer           = metrics.GetOrRegisterTimer("tx/calculate/workNonce", nil)
	transactionVirtualDifficultyTimer            = metrics.GetOrRegisterTimer("tx/calculate/virtualDifficulty", nil)
	transactionsByVirtualDifficultyAndNonceTimer = metrics.GetOrRegisterTimer("txpool/virtualDifficultyAndNonce/sort", nil)
)

var (
	CryptoNightVariant = 2 // 0: original, 1: variant 1, 2: variant 2

	// two256 is a big integer representing 2^256
	two256      = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))
	two256Float = new(big.Float).SetInt(two256)
)

// MinimumDifficulty for transaction PoW
const MinimumTargetDifficulty = 1000000

var (
	ErrInvalidSig = errors.New("invalid transaction v, r, s values")
)

type Transaction struct {
	data txdata
	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
	pow  atomic.Value
}

type txdata struct {
	AccountNonce uint64          `json:"nonce"    gencodec:"required"`
	WorkNonce    uint64          `json:"workNonce" gencodec:"required"`
	GasLimit     uint64          `json:"gas"      gencodec:"required"`
	Recipient    *common.Address `json:"to"       rlp:"nil"` // nil means contract creation
	Amount       *big.Int        `json:"value"    gencodec:"required"`
	Payload      []byte          `json:"input"    gencodec:"required"`

	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

type txdataMarshaling struct {
	AccountNonce hexutil.Uint64
	WorkNonce    hexutil.Uint64
	GasLimit     hexutil.Uint64
	Amount       *hexutil.Big
	Payload      hexutil.Bytes
	V            *hexutil.Big
	R            *hexutil.Big
	S            *hexutil.Big
}

func NewTransaction(workNonce uint64, nonce uint64, to common.Address, amount *big.Int, gasLimit uint64, data []byte) *Transaction {
	return newTransaction(workNonce, nonce, &to, amount, gasLimit, data)
}

func NewContractCreation(workNonce uint64, nonce uint64, amount *big.Int, gasLimit uint64, data []byte) *Transaction {
	return newTransaction(workNonce, nonce, nil, amount, gasLimit, data)
}

func newTransaction(workNonce uint64, nonce uint64, to *common.Address, amount *big.Int, gasLimit uint64, data []byte) *Transaction {
	if len(data) > 0 {
		data = common.CopyBytes(data)
	}
	d := txdata{
		AccountNonce: nonce,
		Recipient:    to,
		Payload:      data,
		Amount:       new(big.Int),
		GasLimit:     gasLimit,
		WorkNonce:    workNonce,
		V:            new(big.Int),
		R:            new(big.Int),
		S:            new(big.Int),
	}
	if amount != nil {
		d.Amount.Set(amount)
	}

	tx := &Transaction{data: d}

	return tx
}

// ChainId returns which chain id this transaction was signed for (if at all)
func (tx *Transaction) ChainId() *big.Int {
	return deriveChainId(tx.data.V)
}

// Protected returns whether the transaction is protected from replay protection.
func (tx *Transaction) Protected() bool {
	return isProtectedV(tx.data.V)
}

func isProtectedV(V *big.Int) bool {
	if V.BitLen() <= 8 {
		v := V.Uint64()
		return v != 27 && v != 28
	}
	// anything not 27 or 28 is considered protected
	return true
}

// EncodeRLP implements rlp.Encoder
func (tx *Transaction) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &tx.data)
}

// DecodeRLP implements rlp.Decoder
func (tx *Transaction) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err == nil {
		tx.size.Store(common.StorageSize(rlp.ListSize(size)))
	}

	return err
}

// rlpWithoutNonce returns the RLP encoded transaction contents, except the nonce.
func (tx *Transaction) rlpForPoW() []byte {
	res, _ := rlp.EncodeToBytes([]interface{}{
		tx.data.AccountNonce,
		tx.data.GasLimit,
		tx.data.Recipient,
		tx.data.Amount,
		tx.data.Payload,
	})
	return res
}

// MarshalJSON encodes the web3 RPC transaction format.
func (tx *Transaction) MarshalJSON() ([]byte, error) {
	hash := tx.Hash()
	data := tx.data
	data.Hash = &hash
	return data.MarshalJSON()
}

// UnmarshalJSON decodes the web3 RPC transaction format.
func (tx *Transaction) UnmarshalJSON(input []byte) error {
	var dec txdata
	if err := dec.UnmarshalJSON(input); err != nil {
		return err
	}

	withSignature := dec.V.Sign() != 0 || dec.R.Sign() != 0 || dec.S.Sign() != 0
	if withSignature {
		var V byte
		if isProtectedV(dec.V) {
			chainID := deriveChainId(dec.V).Uint64()
			V = byte(dec.V.Uint64() - 35 - 2*chainID)
		} else {
			V = byte(dec.V.Uint64() - 27)
		}
		if !crypto.ValidateSignatureValues(V, dec.R, dec.S, false) {
			return ErrInvalidSig
		}
	}

	*tx = Transaction{data: dec}
	return nil
}

func (tx *Transaction) Data() []byte      { return common.CopyBytes(tx.data.Payload) }
func (tx *Transaction) Gas() uint64       { return tx.data.GasLimit }
func (tx *Transaction) WorkNonce() uint64 { return tx.data.WorkNonce }
func (tx *Transaction) Value() *big.Int   { return new(big.Int).Set(tx.data.Amount) }
func (tx *Transaction) Nonce() uint64     { return tx.data.AccountNonce }
func (tx *Transaction) CheckNonce() bool  { return true }

// To returns the recipient address of the transaction.
// It returns nil if the transaction is a contract creation.
func (tx *Transaction) To() *common.Address {
	if tx.data.Recipient == nil {
		return nil
	}
	to := *tx.data.Recipient
	return &to
}

// Hash hashes the RLP encoding of tx.
// It uniquely identifies the transaction.
func (tx *Transaction) Hash() common.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := rlpHash(tx)
	tx.hash.Store(v)
	return v
}

// Size returns the true RLP encoded storage size of the transaction, either by
// encoding and returning it, or returning a previsouly cached value.
func (tx *Transaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, &tx.data)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// GasPrice is for compatibility will be removed TODO
func (tx *Transaction) GasPrice() float64 {
	return tx.CalculateDifficulty() / float64(tx.data.GasLimit)
}

// CalculateDifficulty returns Proof of Work of the transaction either by calculating
// and returning it, or returning a previously cached value.
func (tx *Transaction) CalculateDifficulty() float64 {
	if pow := tx.pow.Load(); pow != nil {
		return pow.(float64)
	}

	buf := make([]byte, 64)
	// h := getCryptoNightBigEndian(tx.rlpForPoW())
	h := crypto.Keccak256(tx.rlpForPoW())
	copy(buf[:32], h[:])
	binary.BigEndian.PutUint64(buf[56:], tx.WorkNonce())

	// hash := new(big.Float).SetInt(new(big.Int).SetBytes(getCryptoNightBigEndian(buf)))
	hashB := crypto.Keccak256(buf)
	hash := new(big.Float).SetInt(new(big.Int).SetBytes(hashB[:]))
	diff := new(big.Float).Quo(two256Float, hash)

	v, _ := diff.Float64()

	tx.pow.Store(v)
	return v
}

// CalculateWorkNonce does the needed PoW for this transaction.
func (tx *Transaction) CalculateWorkNonce(targetDifficulty float64) {
	defer transactionCalculateWorkNonceTimer.UpdateSince(time.Now())

	if targetDifficulty < 1.0 {
		return
	}

	td := new(big.Float).SetFloat64(targetDifficulty)
	targetFloat := new(big.Float).Quo(two256Float, td)
	targetInt, _ := targetFloat.Int(nil)

	buf := make([]byte, 64)
	// h := getCryptoNightBigEndian(tx.rlpForPoW())
	h := crypto.Keccak256(tx.rlpForPoW())
	copy(buf[:32], h[:])

	nonce := uint64(0)
	smallestHash := new(big.Int).Set(two256)
	for {
		binary.BigEndian.PutUint64(buf[56:], nonce)
		// hash := getCryptoNightBigEndian(buf)
		hash := crypto.Keccak256(buf)
		t := new(big.Int).SetBytes(hash[:])

		if t.Cmp(smallestHash) == -1 {
			tx.data.WorkNonce, smallestHash = nonce, t
			if smallestHash.Cmp(targetInt) == -1 {
				return
			}
		}
		nonce++
	}
}

func getCryptoNightBigEndian(buf []byte) []byte {
	// IMPORTANT: cryptonight hash is little endian
	hash := cryptonight.Sum(buf, CryptoNightVariant)

	// swap byte order, since SetBytes accepts big instead of little endian
	swappedHash := make([]byte, 32)
	for i := 0; i < 16; i++ {
		swappedHash[i], swappedHash[31-i] = hash[31-i], hash[i]
	}

	return swappedHash
}

func firstBitSet256(hash []byte) int {
	d := new(big.Int).SetBytes(hash)
	return 256 - d.BitLen() // max bits - actual bits required to represent the hash
}

// AsMessage returns the transaction as a core.Message.
//
// AsMessage requires a signer to derive the sender.
//
// XXX Rename message to something less arbitrary?
func (tx *Transaction) AsMessage(s Signer) (Message, error) {
	msg := Message{
		nonce:      tx.data.AccountNonce,
		gasLimit:   tx.data.GasLimit,
		gasPrice:   big.NewInt(0),
		workNonce:  tx.data.WorkNonce,
		to:         tx.data.Recipient,
		amount:     tx.data.Amount,
		data:       tx.data.Payload,
		checkNonce: true,
	}

	var err error
	msg.from, err = Sender(s, tx)
	return msg, err
}

// WithSignature returns a new transaction with the given signature.
// This signature needs to be in the [R || S || V] format where V is 0 or 1.
func (tx *Transaction) WithSignature(signer Signer, sig []byte) (*Transaction, error) {
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return nil, err
	}
	cpy := &Transaction{data: tx.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v
	return cpy, nil
}

func (tx *Transaction) VirtualDifficulty(from common.Address, ebakusState *ebakusdb.Snapshot) *big.Float {
	defer transactionVirtualDifficultyTimer.UpdateSince(time.Now())
	cv := VirtualCapacity(from, ebakusState)
	txd := tx.CalculateDifficulty()
	return new(big.Float).SetFloat64(cv * txd / float64(tx.Gas()))
}

// Cost returns gas * price.
func (tx *Transaction) Cost() *big.Int {
	gasPrice := big.NewInt(int64(tx.GasPrice()))
	gasLimit := new(big.Int).SetUint64(tx.data.GasLimit)
	return new(big.Int).Mul(gasPrice, gasLimit)
}

// RawSignatureValues returns the V, R, S signature values of the transaction.
// The return values should not be modified by the caller.
func (tx *Transaction) RawSignatureValues() (v, r, s *big.Int) {
	return tx.data.V, tx.data.R, tx.data.S
}

// Transactions is a Transaction slice type for basic sorting.
type Transactions []*Transaction

// Len returns the length of s.
func (s Transactions) Len() int { return len(s) }

// Swap swaps the i'th and the j'th element in s.
func (s Transactions) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (s Transactions) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}

// TxDifference returns a new set which is the difference between a and b.
func TxDifference(a, b Transactions) Transactions {
	keep := make(Transactions, 0, len(a))

	remove := make(map[common.Hash]struct{})
	for _, tx := range b {
		remove[tx.Hash()] = struct{}{}
	}

	for _, tx := range a {
		if _, ok := remove[tx.Hash()]; !ok {
			keep = append(keep, tx)
		}
	}

	return keep
}

// TxByNonce implements the sort interface to allow sorting a list of transactions
// by their nonces. This is usually only useful for sorting transactions from a
// single account, otherwise a nonce comparison doesn't make much sense.
type TxByNonce Transactions

func (s TxByNonce) Len() int           { return len(s) }
func (s TxByNonce) Less(i, j int) bool { return s[i].data.AccountNonce < s[j].data.AccountNonce }
func (s TxByNonce) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// TxByPrice implements both the sort and the heap interface, making it useful
// for all at once sorting as well as individually adding and removing elements.
type TxByPrice struct {
	tx          *Transaction
	from        common.Address
	ebakusState *ebakusdb.Snapshot
}

type TxsByPrice []*TxByPrice

func (s TxsByPrice) Len() int { return len(s) }
func (s TxsByPrice) Less(i, j int) bool {
	cur, next := s[i], s[j]
	curcv := cur.tx.VirtualDifficulty(cur.from, cur.ebakusState)
	nextcv := next.tx.VirtualDifficulty(next.from, next.ebakusState)
	return curcv.Cmp(nextcv) == 1
}

func (s TxsByPrice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (s *TxsByPrice) Push(x interface{}) {
	*s = append(*s, x.(*TxByPrice))
}

func (s *TxsByPrice) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

// TransactionsByVirtualDifficultyAndNonce represents a set of transactions that can return
// transactions in a profit-maximizing sorted order, while supporting removing
// entire batches of transactions for non-executable accounts.
type TransactionsByVirtualDifficultyAndNonce struct {
	txs    map[common.Address]Transactions // Per account nonce-sorted list of transactions
	heads  TxsByPrice                      // Next transaction for each unique account (price heap)
	signer Signer                          // Signer for the set of transactions
}

// NewTransactionsByVirtualDifficultyAndNonce creates a transaction set that can retrieve
// virtualDifficulty sorted transactions in a nonce-honouring way.
//
// Note, the input map is reowned so the caller should not interact any more with
// if after providing it to the constructor.
func NewTransactionsByVirtualDifficultyAndNonce(signer Signer, txs map[common.Address]Transactions, ebakusState *ebakusdb.Snapshot) *TransactionsByVirtualDifficultyAndNonce {
	defer transactionsByVirtualDifficultyAndNonceTimer.UpdateSince(time.Now())

	// Initialize a price based heap with the head transactions
	heads := make(TxsByPrice, 0, len(txs))
	for from, accTxs := range txs {
		heads = append(heads, &TxByPrice{
			tx:          accTxs[0],
			from:        from,
			ebakusState: ebakusState,
		})
		// Ensure the sender address is from the signer
		acc, _ := Sender(signer, accTxs[0])
		txs[acc] = accTxs[1:]
		if from != acc {
			delete(txs, from)
		}
	}
	heap.Init(&heads)

	// Assemble and return the transaction set
	return &TransactionsByVirtualDifficultyAndNonce{
		txs:    txs,
		heads:  heads,
		signer: signer,
	}
}

// Peek returns the next transaction by price.
func (t *TransactionsByVirtualDifficultyAndNonce) Peek() *Transaction {
	if len(t.heads) == 0 {
		return nil
	}
	return t.heads[0].tx
}

// Shift replaces the current best head with the next one from the same account.
func (t *TransactionsByVirtualDifficultyAndNonce) Shift() {
	acc, _ := Sender(t.signer, t.heads[0].tx)
	if txs, ok := t.txs[acc]; ok && len(txs) > 0 {
		t.heads[0].tx, t.txs[acc] = txs[0], txs[1:]
		heap.Fix(&t.heads, 0)
	} else {
		heap.Pop(&t.heads)
	}
}

// Pop removes the best transaction, *not* replacing it with the next one from
// the same account. This should be used when a transaction cannot be executed
// and hence all subsequent ones should be discarded from the same account.
func (t *TransactionsByVirtualDifficultyAndNonce) Pop() {
	heap.Pop(&t.heads)
}

// Message is a fully derived transaction and implements core.Message
//
// NOTE: In a future PR this will be removed.
type Message struct {
	to         *common.Address
	from       common.Address
	nonce      uint64
	workNonce  uint64
	amount     *big.Int
	gasLimit   uint64
	gasPrice   *big.Int
	data       []byte
	checkNonce bool
}

func NewMessage(from common.Address, to *common.Address, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, checkNonce bool) Message {
	return Message{
		from:       from,
		to:         to,
		nonce:      nonce,
		amount:     amount,
		gasLimit:   gasLimit,
		gasPrice:   gasPrice,
		data:       data,
		checkNonce: checkNonce,
	}
}

func (m Message) From() common.Address { return m.from }
func (m Message) To() *common.Address  { return m.to }
func (m Message) GasPrice() *big.Int   { return m.gasPrice }
func (m Message) Value() *big.Int      { return m.amount }
func (m Message) Gas() uint64          { return m.gasLimit }
func (m Message) Nonce() uint64        { return m.nonce }
func (m Message) Data() []byte         { return m.data }
func (m Message) CheckNonce() bool     { return m.checkNonce }
