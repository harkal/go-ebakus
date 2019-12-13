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

// Package types contains data types related to Ebakus consensus.
package types

import (
	"fmt"
	"io"
	"math/big"
	"reflect"
	"sort"
	"sync/atomic"
	"time"

	"github.com/ebakus/go-ebakus/common"
	"github.com/ebakus/go-ebakus/common/hexutil"

	"github.com/ebakus/go-ebakus/rlp"
	"golang.org/x/crypto/sha3"
)

var (
	EmptyRootHash = DeriveSha(Transactions{})
)

type DelegateItem struct {
	Pos             byte
	DelegateAddress common.Address
	DelegateNumber  byte
}

// EncodeRLP is a specialized encoder for DelegateItem to encode only one of the
// two contained DelegateAddress and DelegateNumber.
func (d *DelegateItem) EncodeRLP(w io.Writer) error {
	if d.DelegateAddress == (common.Address{}) {
		return rlp.Encode(w, [2]byte{d.Pos, d.DelegateNumber})
	}

	data := [21]byte{d.Pos}
	copy(data[1:], d.DelegateAddress[:])
	return rlp.Encode(w, data)
}

// DecodeRLP is a specialized decoder for DelegateItem to decode the contents
// into either a block hash or a block number.
func (d *DelegateItem) DecodeRLP(s *rlp.Stream) error {
	var list = []byte{}

	err := s.Decode(&list)

	if err == nil {
		switch len(list) {
		case common.AddressLength + 1:
			d.DelegateAddress = common.BytesToAddress(list[1:])
		case 2:
			d.DelegateNumber = list[1]
		default:
			err = fmt.Errorf("invalid input size for DelegateItem")
		}

		d.Pos = list[0]
	}

	return err
}

// DelegateArray is used for keeping track of delegates in the headers
type DelegateArray []common.Address

type DelegateDiff []DelegateItem

func (a DelegateArray) Diff(b DelegateArray) DelegateDiff {
	r := make(DelegateDiff, 0)

	for i, d := range b {
		found := false
		for j, from := range a {
			if from == d {
				if i != j {
					r = append(r, DelegateItem{byte(i), common.Address{}, byte(j)})
				}
				found = true
				break
			}
		}
		if !found {
			r = append(r, DelegateItem{byte(i), d, 0})
		}
	}

	return r
}

//go:generate gencodec -type Header -field-override headerMarshaling -out gen_header_json.go

// Header represents a block header in the Ebakus blockchain.
type Header struct {
	ParentHash   common.Hash  `json:"parentHash"       gencodec:"required"`
	Signature    []byte       `json:"signature"        gencodec:"required"`
	Root         common.Hash  `json:"stateRoot"        gencodec:"required"`
	TxHash       common.Hash  `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash  common.Hash  `json:"receiptsRoot"     gencodec:"required"`
	Bloom        Bloom        `json:"logsBloom"        gencodec:"required"`
	Number       *big.Int     `json:"number"           gencodec:"required"`
	GasLimit     uint64       `json:"gasLimit"         gencodec:"required"`
	GasUsed      uint64       `json:"gasUsed"          gencodec:"required"`
	Time         uint64       `json:"timestamp"        gencodec:"required"`
	DelegateDiff DelegateDiff `json:"delegateDiff"     gencodec:"required" rlp:"tail"`
}

// field type overrides for gencodec
type headerMarshaling struct {
	Number   *hexutil.Big
	GasLimit hexutil.Uint64
	GasUsed  hexutil.Uint64
	Time     hexutil.Uint64
	Hash     common.Hash `json:"hash"` // adds call to Hash() in MarshalJSON
}

// Hash returns the block hash of the header, which is simply the keccak256 hash of its
// RLP encoding.
func (h *Header) Hash() common.Hash {
	return rlpHash(h)
}

var headerSize = common.StorageSize(reflect.TypeOf(Header{}).Size())

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (h *Header) Size() common.StorageSize {
	return headerSize + common.StorageSize((h.Number.BitLen()/8)+int(reflect.TypeOf(h.Time).Size()))
}

// SanityCheck checks a few basic things -- these checks are way beyond what
// any 'sane' production values should hold, and can mainly be used to prevent
// that the unbounded fields are stuffed with junk data to add processing
// overhead
func (h *Header) SanityCheck() error {
	if h.Number != nil && !h.Number.IsUint64() {
		return fmt.Errorf("too large block number: bitlen %d", h.Number.BitLen())
	}
	return nil
}

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

// Body is a simple (mutable, non-safe) data container for storing and moving
// a block's data contents (transactions) together.
type Body struct {
	Transactions []*Transaction
}

// Block represents an entire block in the Ebakus blockchain.
type Block struct {
	header       *Header
	transactions Transactions

	// caches
	hash atomic.Value
	size atomic.Value

	// These fields are used by package eth to track
	// inter-peer block relay.
	ReceivedAt   time.Time
	ReceivedFrom interface{}
}

// "external" block encoding. used for eth protocol, etc.
type extblock struct {
	Header *Header
	Txs    []*Transaction
}

// NewBlock creates a new block. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of TxHash, UncleHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs and receipts.
func NewBlock(header *Header, txs []*Transaction, receipts []*Receipt, delegateDiff *DelegateDiff) *Block {
	b := &Block{header: CopyHeader(header)}

	if delegateDiff != nil {
		b.header.DelegateDiff = *delegateDiff
	}

	// TODO: panic if len(txs) != len(receipts)
	if len(txs) == 0 {
		b.header.TxHash = EmptyRootHash
	} else {
		b.header.TxHash = DeriveSha(Transactions(txs))
		b.transactions = make(Transactions, len(txs))
		copy(b.transactions, txs)
	}

	if len(receipts) == 0 {
		b.header.ReceiptHash = EmptyRootHash
	} else {
		b.header.ReceiptHash = DeriveSha(Receipts(receipts))
		b.header.Bloom = CreateBloom(receipts)
	}

	return b
}

// NewBlockWithHeader creates a block with the given header data. The
// header data is copied, changes to header and to the field values
// will not affect the block.
func NewBlockWithHeader(header *Header) *Block {
	return &Block{header: CopyHeader(header)}
}

// CopyHeader creates a deep copy of a block header to prevent side effects from
// modifying a header variable.
func CopyHeader(h *Header) *Header {
	cpy := *h
	if cpy.Number = new(big.Int); h.Number != nil {
		cpy.Number.Set(h.Number)
	}
	return &cpy
}

// DecodeRLP decodes the Ebakus
func (b *Block) DecodeRLP(s *rlp.Stream) error {
	var eb extblock
	_, size, _ := s.Kind()
	if err := s.Decode(&eb); err != nil {
		return err
	}
	b.header, b.transactions = eb.Header, eb.Txs
	b.size.Store(common.StorageSize(rlp.ListSize(size)))
	return nil
}

// EncodeRLP serializes b into the Ebakus RLP block format.
func (b *Block) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, extblock{
		Header: b.header,
		Txs:    b.transactions,
	})
}

// TODO: copies

func (b *Block) Transactions() Transactions { return b.transactions }

func (b *Block) Transaction(hash common.Hash) *Transaction {
	for _, transaction := range b.transactions {
		if transaction.Hash() == hash {
			return transaction
		}
	}
	return nil
}

func (b *Block) Number() *big.Int { return new(big.Int).Set(b.header.Number) }
func (b *Block) GasLimit() uint64 { return b.header.GasLimit }
func (b *Block) GasUsed() uint64  { return b.header.GasUsed }
func (b *Block) Time() uint64     { return b.header.Time }

func (b *Block) NumberU64() uint64          { return b.header.Number.Uint64() }
func (b *Block) Bloom() Bloom               { return b.header.Bloom }
func (b *Block) Root() common.Hash          { return b.header.Root }
func (b *Block) ParentHash() common.Hash    { return b.header.ParentHash }
func (b *Block) TxHash() common.Hash        { return b.header.TxHash }
func (b *Block) ReceiptHash() common.Hash   { return b.header.ReceiptHash }
func (b *Block) DelegateDiff() DelegateDiff { return b.header.DelegateDiff }

func (b *Block) Header() *Header { return CopyHeader(b.header) }

// Body returns the non-header content of the block.
func (b *Block) Body() *Body { return &Body{b.transactions} }

// Size returns the true RLP encoded storage size of the block, either by encoding
// and returning it, or returning a previsouly cached value.
func (b *Block) Size() common.StorageSize {
	if size := b.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, b)
	b.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// SanityCheck can be used to prevent that unbounded fields are
// stuffed with junk data to add processing overhead
func (b *Block) SanityCheck() error {
	return b.header.SanityCheck()
}

type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

// WithSeal returns a new block with the data from b but the header replaced with
// the sealed one.
func (b *Block) WithSeal(header *Header) *Block {
	cpy := *header

	return &Block{
		header:       &cpy,
		transactions: b.transactions,
	}
}

// WithBody returns a new block with the given transaction and uncle contents.
func (b *Block) WithBody(transactions []*Transaction) *Block {
	block := &Block{
		header:       CopyHeader(b.header),
		transactions: make([]*Transaction, len(transactions)),
	}
	copy(block.transactions, transactions)

	return block
}

// Hash returns the keccak256 hash of b's header.
// The hash is computed on the first call and cached thereafter.
func (b *Block) Hash() common.Hash {
	if hash := b.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := b.header.Hash()
	b.hash.Store(v)
	return v
}

func (b *Block) String() string {
	str := fmt.Sprintf(`Block(#%v): Size: %v {
%v
Transactions:
%v
}
`, b.Number(), b.Size(), b.header, b.transactions)
	return str
}

func (h *Header) String() string {
	return fmt.Sprintf(`Header(%x):
[
	ParentHash:	    %x
	Signature:		%x
	Root:		    %x
	TxSha		    %x
	ReceiptSha:	    %x
	Bloom:		    %x
	Number:		    %v
	GasLimit:	    %v
	GasUsed:	    %v
	Time:		    %v
]`, h.Hash(), h.ParentHash, h.Signature, h.Root, h.TxHash, h.ReceiptHash, h.Bloom, h.Number, h.GasLimit, h.GasUsed, h.Time)
}

type Blocks []*Block

type BlockBy func(b1, b2 *Block) bool

func (self BlockBy) Sort(blocks Blocks) {
	bs := blockSorter{
		blocks: blocks,
		by:     self,
	}
	sort.Sort(bs)
}

type blockSorter struct {
	blocks Blocks
	by     func(b1, b2 *Block) bool
}

func (self blockSorter) Len() int { return len(self.blocks) }
func (self blockSorter) Swap(i, j int) {
	self.blocks[i], self.blocks[j] = self.blocks[j], self.blocks[i]
}
func (self blockSorter) Less(i, j int) bool { return self.by(self.blocks[i], self.blocks[j]) }

func Number(b1, b2 *Block) bool { return b1.header.Number.Cmp(b2.header.Number) < 0 }
