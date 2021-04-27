package types

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestActor_Empty(t *testing.T) {
	nCid, err := cid.Decode("bafy2bzaceakzxxsce5w6vnv77kkcon4lcotvbcym5dfz2jwxxsy5wva3u2kzc")
	require.NoError(t, err)
	type fields struct {
		Code    cid.Cid
		Head    cid.Cid
		Nonce   uint64
		Balance abi.TokenAmount
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			"empty",
			fields{
				Code:    cid.Undef,
				Head:    cid.Undef,
				Nonce:   0,
				Balance: abi.TokenAmount{},
			},
			true,
		},
		{
			"not empry",
			fields{
				Code:    nCid,
				Head:    nCid,
				Nonce:   0,
				Balance: abi.TokenAmount{},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Actor{
				Code:    tt.fields.Code,
				Head:    tt.fields.Head,
				Nonce:   tt.fields.Nonce,
				Balance: tt.fields.Balance,
			}
			if got := a.Empty(); got != tt.want {
				t.Errorf("Empty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestActor_IncrementSeqNum(t *testing.T) {
	actor := Actor{
		Code:    cid.Undef,
		Head:    cid.Undef,
		Nonce:   0,
		Balance: abi.TokenAmount{},
	}
	for i := 0; i < 10; i++ {
		actor.IncrementSeqNum()
	}
	require.Equal(t, int(actor.Nonce), 10)
}
