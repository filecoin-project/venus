package types

import (
	"bytes"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-state-types/abi"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func TestBlockMsgBasic(t *testing.T) {
	tf.UnitTest(t)
	msgLen := 16
	emptyCids := make([]cid.Cid, msgLen)

	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var src, dst BlockMsg

		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				require.Equal(t, src, dst)
				require.Nil(t, src.Header)
				require.Nil(t, src.BlsMessages)
				require.Nil(t, src.SecpkMessages)
			},

			ProvideOpts: []interface{}{
				testutil.WithSliceLen(msgLen),
			},

			Provided: func() {
				require.NotEqual(t, src, dst, "value provided")
				require.NotNil(t, src.Header)
				require.NotEqual(t, emptyCids, src.BlsMessages)
				require.NotEqual(t, emptyCids, src.SecpkMessages)
			},

			Marshaled: func(b []byte) {
				bmCid := src.Cid()
				require.Equal(t, bmCid, src.Header.Cid(), "Cid() result for BlockMsg")

				sumCid, err := abi.CidBuilder.Sum(b)
				require.NoError(t, err, "CidBuilder.Sum")

				require.NotEqual(t, bmCid, sumCid)

				serialized, err := src.Serialize()
				require.NoError(t, err, "Serialize")
				require.Equal(t, b, serialized)
			},

			Finished: func() {
				require.Equal(t, src, dst, "after unmarshaling")
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}

func TestDecodeBlockMsg(t *testing.T) {
	type args struct {
		b []byte
	}
	tests := []struct {
		name    string
		data    []byte
		want    *BlockMsg
		wantErr bool
	}{
		{"decode empty BlockMsg with extra data at the end", []byte{0x83, 0xf6, 0x80, 0x80, 0x20}, nil, true},
		{"decode valid empty BlockMsg", []byte{0x83, 0xf6, 0x80, 0x80}, new(BlockMsg), false},
		{"decode invalid cbor", []byte{0x83, 0xf6, 0x80}, nil, true},
	}
	for _, tt := range tests {
		data := tt.data
		want := tt.want
		wantErr := tt.wantErr
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeBlockMsg(data)
			if wantErr {
				require.Errorf(t, err, "DecodeBlockMsg(%x)", data)
				return
			}
			require.NoErrorf(t, err, "DecodeBlockMsg(%x)", data)
			require.Equalf(t, want, got, "DecodeBlockMsg(%x)", data)
			serialized, err := got.Serialize()
			require.NoErrorf(t, err, "DecodeBlockMsg(%x)", data)
			require.Equalf(t, serialized, data, "DecodeBlockMsg(%x)", data)
		})
	}
}
