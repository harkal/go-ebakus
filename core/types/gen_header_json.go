// Code generated by github.com/fjl/gencodec. DO NOT EDIT.

package types

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/ebakus/go-ebakus/common"
	"github.com/ebakus/go-ebakus/common/hexutil"
)

var _ = (*headerMarshaling)(nil)

// MarshalJSON marshals as JSON.
func (h Header) MarshalJSON() ([]byte, error) {
	type Header struct {
		ParentHash   common.Hash    `json:"parentHash"       gencodec:"required"`
		Signature    []byte         `json:"signature"        gencodec:"required"`
		Root         common.Hash    `json:"stateRoot"        gencodec:"required"`
		TxHash       common.Hash    `json:"transactionsRoot" gencodec:"required"`
		ReceiptHash  common.Hash    `json:"receiptsRoot"     gencodec:"required"`
		Bloom        Bloom          `json:"logsBloom"        gencodec:"required"`
		Number       *hexutil.Big   `json:"number"           gencodec:"required"`
		GasLimit     hexutil.Uint64 `json:"gasLimit"         gencodec:"required"`
		GasUsed      hexutil.Uint64 `json:"gasUsed"          gencodec:"required"`
		Time         hexutil.Uint64 `json:"timestamp"        gencodec:"required"`
		DelegateDiff DelegateDiff   `json:"delegateDiff"     gencodec:"required" rlp:"tail"`
		Hash         common.Hash    `json:"hash"`
	}
	var enc Header
	enc.ParentHash = h.ParentHash
	enc.Signature = h.Signature
	enc.Root = h.Root
	enc.TxHash = h.TxHash
	enc.ReceiptHash = h.ReceiptHash
	enc.Bloom = h.Bloom
	enc.Number = (*hexutil.Big)(h.Number)
	enc.GasLimit = hexutil.Uint64(h.GasLimit)
	enc.GasUsed = hexutil.Uint64(h.GasUsed)
	enc.Time = hexutil.Uint64(h.Time)
	enc.DelegateDiff = h.DelegateDiff
	enc.Hash = h.Hash()
	return json.Marshal(&enc)
}

// UnmarshalJSON unmarshals from JSON.
func (h *Header) UnmarshalJSON(input []byte) error {
	type Header struct {
		ParentHash   *common.Hash    `json:"parentHash"       gencodec:"required"`
		Signature    []byte          `json:"signature"        gencodec:"required"`
		Root         *common.Hash    `json:"stateRoot"        gencodec:"required"`
		TxHash       *common.Hash    `json:"transactionsRoot" gencodec:"required"`
		ReceiptHash  *common.Hash    `json:"receiptsRoot"     gencodec:"required"`
		Bloom        *Bloom          `json:"logsBloom"        gencodec:"required"`
		Number       *hexutil.Big    `json:"number"           gencodec:"required"`
		GasLimit     *hexutil.Uint64 `json:"gasLimit"         gencodec:"required"`
		GasUsed      *hexutil.Uint64 `json:"gasUsed"          gencodec:"required"`
		Time         *hexutil.Uint64 `json:"timestamp"        gencodec:"required"`
		DelegateDiff *DelegateDiff   `json:"delegateDiff"     gencodec:"required" rlp:"tail"`
	}
	var dec Header
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.ParentHash == nil {
		return errors.New("missing required field 'parentHash' for Header")
	}
	h.ParentHash = *dec.ParentHash
	if dec.Signature == nil {
		return errors.New("missing required field 'signature' for Header")
	}
	h.Signature = dec.Signature
	if dec.Root == nil {
		return errors.New("missing required field 'stateRoot' for Header")
	}
	h.Root = *dec.Root
	if dec.TxHash == nil {
		return errors.New("missing required field 'transactionsRoot' for Header")
	}
	h.TxHash = *dec.TxHash
	if dec.ReceiptHash == nil {
		return errors.New("missing required field 'receiptsRoot' for Header")
	}
	h.ReceiptHash = *dec.ReceiptHash
	if dec.Bloom == nil {
		return errors.New("missing required field 'logsBloom' for Header")
	}
	h.Bloom = *dec.Bloom
	if dec.Number == nil {
		return errors.New("missing required field 'number' for Header")
	}
	h.Number = (*big.Int)(dec.Number)
	if dec.GasLimit == nil {
		return errors.New("missing required field 'gasLimit' for Header")
	}
	h.GasLimit = uint64(*dec.GasLimit)
	if dec.GasUsed == nil {
		return errors.New("missing required field 'gasUsed' for Header")
	}
	h.GasUsed = uint64(*dec.GasUsed)
	if dec.Time == nil {
		return errors.New("missing required field 'timestamp' for Header")
	}
	h.Time = uint64(*dec.Time)
	if dec.DelegateDiff == nil {
		return errors.New("missing required field 'delegateDiff' for Header")
	}
	h.DelegateDiff = *dec.DelegateDiff
	return nil
}
