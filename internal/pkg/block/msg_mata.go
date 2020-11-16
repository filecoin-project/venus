package block

import (
	"bytes"
	"fmt"
	"io"

	"github.com/filecoin-project/go-state-types/abi"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"
)

type MsgMeta struct {
	BlsMessages   cid.Cid
	SecpkMessages cid.Cid
}

func (mm *MsgMeta) Cid() cid.Cid {
	b, err := mm.ToStorageBlock()
	if err != nil {
		panic(err) // also maybe sketchy
	}
	return b.Cid()
}

func (mm *MsgMeta) ToStorageBlock() (blocks.Block, error) {
	var buf bytes.Buffer
	if err := mm.MarshalCBOR(&buf); err != nil {
		return nil, xerrors.Errorf("failed to marshal MsgMeta: %w", err)
	}

	c, err := abi.CidBuilder.Sum(buf.Bytes())
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(buf.Bytes(), c)
}

var lengthBufMsgMeta = []byte{130}

func (t *MsgMeta) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufMsgMeta); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.BlsMessages (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.BlsMessages); err != nil {
		return xerrors.Errorf("failed to write cid field t.BlsMessages: %w", err)
	}

	// t.SecpkMessages (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.SecpkMessages); err != nil {
		return xerrors.Errorf("failed to write cid field t.SecpkMessages: %w", err)
	}

	return nil
}

func (t *MsgMeta) UnmarshalCBOR(r io.Reader) error {
	*t = MsgMeta{}

	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)

	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 2 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.BlsMessages (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.BlsMessages: %w", err)
		}

		t.BlsMessages = c

	}
	// t.SecpkMessages (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.SecpkMessages: %w", err)
		}

		t.SecpkMessages = c

	}
	return nil
}
