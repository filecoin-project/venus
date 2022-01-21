package types

import (
	"encoding/binary"
	"io"
	"io/ioutil"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/crypto"
	fcbor "github.com/fxamacker/cbor/v2"
	"github.com/minio/blake2b-simd"
	"golang.org/x/xerrors"
)

type MsgType string

const (
	MTUnknown = MsgType("unknown")

	// Signing message CID. MsgMeta.Extra contains raw cbor message bytes
	MTChainMsg = MsgType("message")

	// Signing a blockheader. signing raw cbor block bytes (MsgMeta.Extra is empty)
	MTBlock = MsgType("block")

	// Signing a deal proposal. signing raw cbor proposal bytes (MsgMeta.Extra is empty)
	MTDealProposal = MsgType("dealproposal")
	// extra is nil, 'toSign' is cbor raw bytes of 'DrawRandomParams'
	//  following types follow above rule
	MTDrawRandomParam = MsgType("drawrandomparam")
	MTSignedVoucher   = MsgType("signedvoucher")
	MTStorageAsk      = MsgType("storageask")
	MTAskResponse     = MsgType("askresponse")
	MTNetWorkResponse = MsgType("networkresposne")

	// reference : storagemarket/impl/remotecli.go:330
	// sign storagemarket.ClientDeal.ProposalCid,
	// MsgMeta.Extra is nil, 'toSign' is market.ClientDealProposal
	// storagemarket.ClientDeal.ProposalCid equals cborutil.AsIpld(market.ClientDealProposal).Cid()
	MTClientDeal = MsgType("clientdeal")

	MTProviderDealState = MsgType("providerdealstate")

	MTVerifyAddress = MsgType("verifyaddress")
)

type MsgMeta struct {
	Type MsgType

	// Additional data related to what is signed. Should be verifiable with the
	// signed bytes (e.g. CID(Extra).Bytes() == toSign)
	Extra []byte
}

type DrawRandomParams struct {
	Rbase   []byte
	Pers    crypto.DomainSeparationTag
	Round   abi.ChainEpoch
	Entropy []byte
}

// return store.DrawRandomness(dr.Rbase, dr.Pers, dr.Round, dr.Entropy)
func (dr *DrawRandomParams) SignBytes() ([]byte, error) {
	h := blake2b.New256()
	if err := binary.Write(h, binary.BigEndian, int64(dr.Pers)); err != nil {
		return nil, xerrors.Errorf("deriving randomness: %w", err)
	}
	VRFDigest := blake2b.Sum256(dr.Rbase)
	_, err := h.Write(VRFDigest[:])
	if err != nil {
		return nil, xerrors.Errorf("hashing VRFDigest: %w", err)
	}
	if err := binary.Write(h, binary.BigEndian, dr.Round); err != nil {
		return nil, xerrors.Errorf("deriving randomness: %w", err)
	}
	_, err = h.Write(dr.Entropy)
	if err != nil {
		return nil, xerrors.Errorf("hashing entropy: %w", err)
	}

	return h.Sum(nil), nil
}

func (dr *DrawRandomParams) MarshalCBOR(w io.Writer) error {
	data, err := fcbor.Marshal(dr)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func (dr *DrawRandomParams) UnmarshalCBOR(r io.Reader) error {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return fcbor.Unmarshal(data, dr)
}

var _ = cbor.Unmarshaler((*DrawRandomParams)(nil))
var _ = cbor.Marshaler((*DrawRandomParams)(nil))
