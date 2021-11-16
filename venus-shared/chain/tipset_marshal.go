package chain

import (
	"encoding/json"
	"io"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"
)

type ExpTipSet struct {
	Cids   []cid.Cid
	Blocks []*BlockHeader
	Height abi.ChainEpoch
}

func (ts *TipSet) MarshalJSON() ([]byte, error) {
	// why didnt i just export the fields? Because the struct has methods with the
	// same names already
	return json.Marshal(ExpTipSet{
		Cids:   ts.cids,
		Blocks: ts.blocks,
		Height: ts.height,
	})
}

func (ts *TipSet) UnmarshalJSON(b []byte) error {
	var ets ExpTipSet
	if err := json.Unmarshal(b, &ets); err != nil {
		return err
	}

	ots, err := NewTipSet(ets.Blocks)
	if err != nil {
		return err
	}

	*ts = *ots

	return nil
}

func (ts *TipSet) MarshalCBOR(w io.Writer) error {
	if ts == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	return (&ExpTipSet{
		Cids:   ts.cids,
		Blocks: ts.blocks,
		Height: ts.height,
	}).MarshalCBOR(w)
}

func (ts *TipSet) UnmarshalCBOR(r io.Reader) error {
	var ets ExpTipSet
	if err := ets.UnmarshalCBOR(r); err != nil {
		return err
	}

	ots, err := NewTipSet(ets.Blocks)
	if err != nil {
		return err
	}

	*ts = *ots

	return nil
}
